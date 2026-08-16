# Linkomat — v1 Development Plan

This document captures the agreed design for v1 of Linkomat, derived from
`README.md` and a one-question-at-a-time design review. v1 is a
single-user-style MVP, but the architecture is user-scoped from day one.

## 0. Resume Instructions

For a fresh agent session continuing work on this project:

1. **Read in this order**: `README.md` → `AGENTS.md` (root) → this file
   (`doc/PLANNING.md`) → if you need component-level context, the
   relevant doc in `doc/architecture/`.
2. **Source of truth**: this plan overrides the README where they
   disagree. If you find a discrepancy during implementation, update
   this file and call it out in the commit message.
3. **Scope guardrails**: do not introduce features listed in §11
   (Deferred) without an explicit ask. Do not add automated tests or
   CI in v1 (§1).
4. **Working agreement with the user**: address them as
   “Mr. Anderson”. Do not summarize at the end of a task — just stop.
   Do not add code comments unless asked.
5. **Pinned decisions** (do not renegotiate without prompting):
   - No self-signup; users created via server CLI only.
   - Tokens-as-sessions model; multiple sessions per user coexist as
     multiple `tokens` rows. (See `architecture/api.md` — Auth.)
   - Refresh-token rotation on every `/auth/refresh`.
   - Server-authoritative `updated_at`; client tracks
     `local_dirty_at` + `base_updated_at`.
   - Last-save-wins by client intent, no merge UI.
   - sql.js + OPFS for web local DB (no IDB fallback in v1).
   - sql.js WASM is the **browser** build (`sql-wasm-browser.wasm`),
     NOT `sql-wasm.wasm` (the Node build). Self-hosted at
     `/sqlite/`. The sql.js loader calls `locateFile` with that
     exact name.
   - The web SW does **not** precache anything under `/sqlite/*`
     (`globPatterns` excludes `wasm`; `navigateFallbackDenylist`
     blocks `/^\/sqlite\//`). sql.js's `locateFile` URL carries a
     `?v=${SQLJS_ASSET_VERSION}` cache-buster so even a stale SW
     cannot match a precache entry against it.
   - Web CSP `script-src` must include `'wasm-unsafe-eval'`
     (required by `WebAssembly.instantiate` for sql.js).
   - No Background Sync API usage.
   - Light mode only.
   - Embedded web assets in the server binary.
   - Node.js ≥ 24 is required for the web build (Vite 8 / vite-plugin-pwa 1
     require it; `npm install` and `npm run build` both emit engine warnings on
     older Node). The `web/` folder ships a `.nvmrc` pinned to `24`. Use a
     system-wide Node ≥ 24 if available; otherwise `cd web && nvm use` before
     any direct `npm` call. The Makefile already sources `$(HOME)/.nvm/nvm.sh`
     and runs `nvm use` for `make web-build` / `make dev`, so those targets work
     regardless of the system Node version.
6. **Placeholders to confirm before module init**:
   - Go module path: `github.com/bylexus/linkomat/{api,cli,shared}`
     (confirmed in step 1; existing `go.mod` files set this).
   - Default server URL in CLI: `https://app.linkomat.example.com`
     — update to the operator’s domain before any release.
   - Default PWA theme/background colors: confirmed in step 1 —
     `theme_color=#2563eb`, `background_color=#ffffff` (light only).
7. **Build order**: follow §10 strictly. Each step should end with a
   manual smoke check; no automated tests exist yet.

**Architecture reference**: `doc/architecture/` documents the system
that has already been built — the global map (`overview.md`), the Go
server (`api.md`), the Vue 3 PWA (`web.md`), the CLI (`cli.md`,
placeholder), and build / run / test (`build.md`). For component-level
questions (request lifecycle, local DB schema, sync engine state
machine, env vars, Makefile targets) read those docs rather than the
old sections here — they are kept in sync with the code.

## 1. Scope

- Single deployable server + embedded web PWA + CLI client.
- Multiple provisioned users supported, but no public sign-up.
- No multi-device awareness, no merge UI, no admin web UI.
- No CI, no automated tests; manual testing only in v1.
- v2 candidates (deferred): offline CLI, rich text, dark mode, multi-device
  locking, bulk sync endpoint.

## 2. Components

See `architecture/overview.md` for the global map and per-component
docs (`api.md`, `web.md`, `cli.md`, `build.md`) for what each part
does and how they connect.


## 10. Build Checklist

Steps 1–9 done. **Next: step 10 — end-to-end manual pass on mobile
(PWA install). CLI lands in step 11.** Build/test commands, env
vars, and deploy notes live in `architecture/build.md` — read that
for the actual commands. The CLI is intentionally the last step so
the API surface stays stable while the web app — the primary v1
client — drives the design first.

1. Repo scaffold: `go.work`, `Makefile`, module skeletons for `api`,
   `cli`, `shared`, Vite scaffold for `web`.
2. `shared/` DTOs: `Entry`, `AuthResponse`, `ErrorResponse`, constants
   for cookie names, schema version constant, `currentSchemaVersion`.
3. Server: meta DB, user CRUD CLI, password hashing, `/auth/login`,
   `/auth/refresh`, `/auth/logout`, rate limit.
4. Server: per-user DB cache, `GET/PUT/DELETE /entries`, soft-delete,
   `AutoMigrate` + `schema_version`.
5. Server: embedded web assets, `/healthz`, `/version`, security
   headers, request logging.
6. Web: PWA scaffold, sql.js wrapper, OPFS persistence, integrity
   check. **Done**
7. Web: home view + virtualized list + search. **Done**
8. Web: note editing view + save-on-blur + delete. **Done**
9. Web: sync engine (dirty queue, delta pull, full pull on first
   launch, retry/backoff, status UI). Split into 9.1 auth + login view
   (**done**), 9.2 sync engine + cursor + reachability (**done**),
   9.3 status UI + backoff + retry (**done**).
10. End-to-end manual pass on mobile (PWA install) — CLI still
    pending at this point.
11. CLI: `login`, `logout`, `add`, `ls`, `search`, `get`, `rm`,
    `edit` (with `fsnotify`).
12. Final end-to-end pass covering both PWA and CLI.

## 11. Deferred (v2+)

- Offline CLI.
- Bulk sync endpoint.
- Multi-device awareness and per-device token lock.
- Self-sign-up flow.
- Rich text editing / preview.
- Dark mode.
- GitHub Actions CI + automated tests.

## 12. Progress Log

Session-by-session state. Read this section first when resuming.
Step implementations live in the code; the docs in
`doc/architecture/` describe the current shape of the system. This
section records session intent + commit hash + landed files, and
keeps the items the next session needs cold.

### Session index

| #  | Step          | Commit                                  | Title                                                              |
|----|---------------|-----------------------------------------|--------------------------------------------------------------------|
| 1  | §10 step 1    | `1cab067`                               | repo scaffold (`go.work`, Makefile, Vite 8 + Vue 3 + PWA)         |
| 2  | §10 step 2    | `c51fa80`                               | `shared/` DTOs (Entry, AuthResponse, ErrorResponse, cookies)       |
| 3  | §10 step 3    | `2acc1c7`                               | server auth + meta DB + user CLI + Bruno collection                 |
| 4  | (small change)| `01d74d3`                               | drop server-side TLS termination; rely on TLS proxy                |
| 5  | §10 step 4    | `0c5bd1f`                               | per-user DB cache + entries endpoints                              |
| 6  | plan          | `23af127`                               | reorder build steps; defer CLI to step 11                          |
| 7  | §10 step 5    | `bda620f`                               | embedded web assets + /healthz + /version + security headers       |
| 8  | (cleanup)     | `c61c83b`                               | fold Cache-Control into apiHeaders middleware                      |
| 9  | §10 step 6    | `d67d6e2`                               | web PWA scaffold + sql.js + OPFS + integrity check                 |
| 10 | §10 step 7    | `6320abf`                               | web home view + virtualized list + FTS3 search + dev global        |
| 11 | §10 step 8    | `bb64b7a`                               | web EditView + save-on-blur + delete + URL link                   |
| 12 | §10 step 9.1  | `29059b8`                               | API client + auth flow + Login view + per-user OPFS                 |
| 13 | §10 step 9.2  | `b4faa16`                               | sync engine: cursor + dirty queue + reachability + push-on-save    |
| 14 | §10 step 9.3  | `29a032c`                               | sync status UI + backoff + retry button                            |

Key landed files by step (also reachable from `git show <commit>`):

- 9 — `web/src/db/{schema.ts,opfs.ts,sqlite.ts}`, `web/src/App.vue`,
  `web/scripts/copy-sqljs-wasm.mjs`, `web/public/sqlite/`
  (gitignored).
- 10 — `web/src/views/HomeView.vue`, `web/src/router.ts`,
  `web/src/db/index.ts` (FTS3 + `entries_fts` triggers in `schema.ts`).
- 11 — `web/src/views/EditView.vue`, `web/src/utils/{uuid.ts,url.ts}`.
- 12 — `web/src/api/{client.ts,auth.ts}`, `web/src/state/auth.ts`,
  `web/src/views/LoginView.vue`.
- 13 — `web/src/api/entries.ts`, `web/src/state/sync.ts`,
  `web/src/sync/{reachability.ts,engine.ts}`, `sync_state` table
  (schema v3). Server: `shared/entry.go`, `api/handlers_entries.go`.
- 14 — `web/src/views/HomeView.vue` (status chip + row dots),
  `web/src/views/EditView.vue` (per-entry dot + echo), backoff in
  `web/src/sync/engine.ts`.

### Key traps worth remembering

- **sql.js rejects `undefined` bind parameters** (only `null` maps to
  SQL NULL). A Go DTO with `json:"…,omitempty"` on a nil pointer omits
  the key, which arrives as `undefined` and throws
  `Wrong API use : tried to bind a value of an unknown type`. The
  `deleted_at` wire contract is fixed on both sides (server drops
  `omitempty`; client binds `entry.deleted_at ?? null`).
- **Do not expose a Vue state snapshot for debugging.** The dev global
  must publish the reactive `Ref` itself, not a one-shot value —
  otherwise console reads are frozen at login time and misleading.
- **`<sqlite>` rebuild needs `INSERT OR UPDATE` sql.js helper, not
  `INSERT OR REPLACE`.** `REPLACE` does not fire the `AFTER DELETE`
  trigger (recursive triggers off) and reallocates the rowid,
  orphaning `entries_fts` docids on every sync.
- **sql.js WASM file is `sql-wasm-browser.wasm`, not
  `sql-wasm.wasm`.** The Node build ships in the npm artefact and
  causes a 404 → SPA fallback → HTML parsed as WASM → magic byte
  mismatch failure that looks like a SW problem.
- **Backoff + 2 s push debounce + 5 min interval must share the same
  `syncInFlight` guard.** Backoff is a single `setTimeout` keyed by
  `backoffTimer`; the debounce skips scheduling while a cycle is
  in-flight; the interval re-arms only on a successful cycle that
  still has dirty rows. Adding a second retry timer breaks the loop
  suppression.

### Step 10 — Mobile PWA install (next)

**Scope (§10 step 10):** install the PWA on iOS Safari and Android
Chrome; verify the app shell, status chip, entry CRUD, and sync
round-trip from a mobile browser. CLI is still pending at step 11.
The server must be reachable over HTTPS — the cookies are `Secure`
and the PWA install flow on iOS rejects plain HTTP.

**Prerequisites (developer step):**

1. Run the server behind a TLS-terminating reverse proxy (Caddy is
   the simplest — one Caddyfile with `reverse_proxy localhost:8080`).
   Set `LINKOMAT_BEHIND_PROXY=1` so the server trusts
   `X-Forwarded-For` and the `Secure` cookie flag stays `true`.
2. Confirm `https://<host>/healthz` returns 200 from the mobile
   device's network (same Wi-Fi, VPN, or public DNS).
3. Build the production server: `make build-server` (NOT
   `expose-dev` — the dev global opens a manual SQL injection vector
   that should not ship). Restart the server.
4. Provision a test user: `printf "testpw\ntestpw\n" |
   ./bin/linkomat-server user create mobile`. Throwaway; rotate or
   delete after the test pass.

**iOS Safari flow:**

1. Open `https://<host>/` in Safari. Sign in.
2. Share → "Add to Home Screen". Confirm the manifest name "Link"
   appears (not "Linkomat" — `web/src/manifest.webmanifest`
   `short_name: Link`).
3. Launch from the home-screen icon. Confirm full-screen (no Safari
   chrome), theme color `#2563eb`, home view loads.
4. Status chip visible; `pending: N` behaves correctly offline
   (toggle Airplane mode on/off).
5. Create an entry while offline → `pending: 1` + `Offline` (gray),
   bring network back → within 10 s the chip flips to `Syncing…`
   and the count drops to 0.

**Android Chrome flow:**

1. Open `https://<host>/` in Chrome. Sign in.
2. Install banner (or kebab → "Install app"). Confirm install.
3. Launch from the app drawer. Same full-screen / theme checks.
4. Offline create → push round-trip (Chrome install is more
   permissive than iOS on backgrounding).

**Acceptance for step 10:**

- PWA installs on iOS 17+ Safari and Android Chrome 120+.
- App shell loads from cache (DevTools / Web Inspector → Network /
  Storage should show zero network requests on relaunch).
- Offline create / edit / delete works; sync resumes on reconnect.
- The dev global is **not** exposed in the production build.

**Known gaps to verify still hold:**

- The `/settings` route is still not implemented (HomeView's
  temporary logout button is fine for step 10).
- Direct browser navigation to `/entries/:id` still returns 401 on a
  hard reload (mux precedence). Mobile reloads are not part of the
  test pass; SPA pushState navigation is what users hit in practice.
  A proper fix would route on `Accept: text/html`.
- The `Secure` cookie flag is on by default —
  `LINKOMAT_COOKIE_SECURE=false` is only for local HTTP dev. Do not
  ship that.