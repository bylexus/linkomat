# Web app (`web/`)

Vue 3 + TypeScript SPA built with Vite. Local-first, offline-capable
PWA. Syncs with the Go server via cookies; the local DB is the
source of truth for reads.

## Stack

- Vue 3, TypeScript, Vite, `vite-plugin-pwa` (workbox).
- `vue-router` (history mode).
- `vue-virtual-scroller` (fixed `item-size=96` rows on the home view).
- `sql.js` (pure WASM SQLite) for the local DB. The **browser** build
  (`sql-wasm-browser.wasm`) is copied to `web/public/sqlite/` by
  `web/scripts/copy-sqljs-wasm.mjs` and self-hosted from there.
- OPFS (Origin Private File System) for persistence. No IndexedDB
  fallback in v1.
- FTS3 (built into `sql.js@1.14.2`'s WASM) for search. FTS5 is not
  available without an EMSDK fork build — see
  `PLANNING.md` §7.7.

## Top-level layout

```
web/src/
├── main.ts                # boot: load auth state, initSync(), create app, mount
├── App.vue                # auth-guard shell + RouterView + status panel
├── router.ts              # 3 routes: /login, /, /entries/:id
├── env.d.ts, sql.js.d.ts  # local type shims
├── api/
│   ├── client.ts          # fetch wrapper + single in-flight 401→refresh
│   ├── auth.ts            # login / logout / probe
│   └── entries.ts         # listEntries / getEntry / putEntry / deleteEntry + DTOs
├── db/
│   ├── index.ts           # useDb(username) singleton; closeDb(); dev global
│   ├── schema.ts          # CURRENT_SCHEMA_VERSION, DDL, OPFS filename helpers
│   ├── opfs.ts            # OPFS read/write/rename/sidecar helpers
│   └── sqlite.ts          # LocalDb class — the only thing that touches sql.js
├── state/
│   ├── auth.ts            # useAuth(): AuthState, username, login/logout/probe
│   └── sync.ts            # useSyncState(): reactive SyncState (pending, syncing, lastError, …)
├── sync/
│   ├── engine.ts          # initSync / syncOnce / syncNow; push + full pull + delta pull
│   └── reachability.ts    # navigator.onLine + /healthz poll; useReachability()
├── utils/
│   ├── uuid.ts            # client-side UUIDv7 (no dep)
│   └── url.ts             # normalizeUrl on blur + render
└── views/
    ├── LoginView.vue      # username + password form
    ├── HomeView.vue       # search bar, virtualized list, FAB, sync chip row
    └── EditView.vue       # title / url / tags / note, save-on-blur, delete
```

## State model

- **Auth** (`state/auth.ts`) — `unknown` | `logged-in` | `logged-out`.
  On boot, `unknown` if `localStorage.linkomat.current_username` is
  set, else `logged-out`. `LoginView` calls `login()`; `App.vue`
  triggers a `probe()` on `unknown` to confirm. On a 401-from-refresh
  the API client calls the installed logout handler, which
  transitions to `logged-out` and closes the DB.
- **DB** (`db/index.ts`) — `useDb(username)` returns a singleton
  `LocalDb` plus a reactive `DbState` (`pending` / `opfs-unavailable` /
  `ready` / `error`). Recreates the singleton on username change.
  Exposes `window.__linkomatDb` in dev (`import.meta.env.DEV` or
  `VITE_EXPOSE_DEV`) for manual DevTools seeding — see
  `Makefile` → `build-server-expose-dev`.
- **Sync** (`state/sync.ts`) — single reactive `SyncState`: `online`,
  `syncing`, `pendingCount`, `lastError`, `lastSkipReason`,
  `lastSyncAt`, `persistentFailure`. Driven by the sync engine; the
  HomeView and EditView read it for the status chip + dots.

## Local DB

`db/sqlite.ts` is the only file that touches sql.js.

- `open()` — loads the WASM, reads `linkomat.<username>.db` from
  OPFS, runs `PRAGMA integrity_check`, and either keeps the loaded DB
  or sidecars the corrupt file (`corrupt-<username>-<ts>.db`) and
  creates a fresh schema. Sets `state` accordingly.
- `persist()` — writes the in-memory database back to OPFS. Called
  after every `upsertEntry`, `softDeleteEntry`, `setCursor`, and
  `clearDirty`. `upsertEntry` and `softDeleteEntry` also dispatch a
  `linkomat-db-change` window `CustomEvent` (constant
  `DB_CHANGE_EVENT`) so other views can refresh.
- `getCursor()` / `setCursor()` — read/write the single-row
  `sync_state` table.
- `listDirtyEntries()` / `countDirtyEntries()` — FIFO over
  `local_dirty_at` for the push queue.
- `applyServerEntries()` / `applyServerEntry()` — write server rows
  into the local DB; updates the FTS triggers transparently. Does
  not touch `local_dirty_at` (server is authoritative for `updated_at`).
- `clearDirty(id, serverUpdatedAt)` — drop the dirty mark, set
  `base_updated_at` to the server's `updated_at`.
- `searchEntries(query)` — FTS3 `MATCH`. `toFtsQuery` strips stray
  `"` and `'`, preserves uppercase FTS3 operators (`AND`/`OR`/`NOT`/
  `NEAR`), appends `*` to non-operator tokens for prefix matching.
  Empty query → `listEntries()`.

## Sync engine

`sync/engine.ts` is a module-level singleton. **It runs on the main
thread**, not a `Worker` (despite the historical name). See
`PLANNING.md` §7.6.

- `initSync()` is called from `main.ts` once. It starts
  `reachability`, watches `useAuth()`. On `logged-in`: attaches
  listeners and starts the 5-min interval; calls `syncOnce()`.
  On `logged-out`: detaches, stops the interval, resets sync state.
- `syncOnce()` — single in-flight job. Skips with a
  `lastSkipReason` if already running, no username, offline, or DB
  not ready. Otherwise:
  1. `pushDirtyEntries()` — FIFO over `listDirtyEntries()`. `PUT`
     for live rows, `DELETE` for soft-deleted. On `ok` (or 404 for
     delete) call `clearDirty` with the server's `updated_at` /
     `deleted_at`. Any other result throws and lands in backoff.
  2. **Full pull** if no cursor (`PULL_PAGE_SIZE = 200`,
     `PULL_MAX_PAGES = 200`). Otherwise **delta pull** with
     `?changed_since=<cursor>`. `applyServerEntries` upserts by id.
  3. `applyCursor(server_time)` — persist the new cursor.
  4. On success: reset `lastError`, `lastSyncAt = now()`,
     `resetBackoff()`.
  5. On throw: `setSyncState({ lastError })`, `scheduleBackoff()`.
- **Backoff** — single `setTimeout` per streak. `backoffAttempt`
  doubles the base (1s → 60s cap) with ±20% jitter. `firstFailureAt`
  is set on the first failure; when the next timer fires after
  >5 min of streak (`PERSISTENT_FAILURE_MS`), `persistentFailure`
  flips on and the UI shows "Retry now".
- **Triggers** — `online` event, `visibilitychange` to visible,
  5-min interval, `linkomat-db-change` (debounced 2s), manual
  `syncNow()`.

## API client

`api/client.ts` is a thin `fetch` wrapper.

- Always `credentials: 'same-origin'` — the browser sends the
  `linkomat_auth` / `linkomat_refresh` cookies.
- On 401, `tryRefresh()` does one in-flight
  `POST /auth/refresh` (no body, relies on cookie). On success the
  original request is retried once; on failure the installed
  `setLogoutHandler` runs and the 401 is returned.
- Returns an `ApiResult<T>`: `ok` / `http-error` / `network-error`.
  Callers inspect `kind` rather than throwing.

## PWA

`vite.config.ts` configures `vite-plugin-pwa` (workbox).

- `manifest.webmanifest` — `name=Linkomat`, `short_name=Link`,
  `theme_color=#2563eb`, `background_color=#ffffff` (light only).
  Icons: 192, 512, maskable.
- `globPatterns` excludes `wasm` and
  `navigateFallbackDenylist` excludes `/^\/sqlite\//`. The service
  worker **never** precaches or serves anything under `/sqlite/`.
- `runtimeCaching: []` — `/entries` and `/healthz` are *not*
  intercepted; all traffic is plain `fetch` and shows up in
  DevTools' Network tab.
- `locateFile` URL carries `?v=${SQLJS_ASSET_VERSION}` cache-buster
  so a stale SW cannot match the WASM.

## Where to look when…

- "How does an entry reach the server?" — `views/EditView.vue` →
  `LocalDb.upsertEntry` → `DB_CHANGE_EVENT` → `sync/engine.ts`
  `onDbChange` → `schedulePush` (2s debounce) → `syncOnce` →
  `pushDirtyEntries` → `apiPut` (`api/entries.ts`) → server.
- "Why is search broken on a token like `[rust]`?" — FTS3 sees the
  literal JSON syntax. The `entries_ai` / `entries_au` triggers
  replace `[`, `]`, `"`, `,` with spaces before indexing.
  `db/schema.ts`.
- "Why is `useDb()` returning `pending`?" — `LocalDb.open()` is
  async; `App.vue` shows a splash until it resolves. The auth guard
  also waits for the auth state to settle.
- "Where does the status dot in the row come from?" — HomeView reads
  `useSyncState()` and computes per-row status from the entry's
  `local_dirty_at` vs. the global `syncing` flag.
