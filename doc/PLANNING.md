# Linkomat — v1 Development Plan

This document captures the agreed design for v1 of Linkomat, derived from
`README.md` and a one-question-at-a-time design review. v1 is a
single-user-style MVP, but the architecture is user-scoped from day one.

## 0. Resume Instructions

For a fresh agent session continuing work on this project:

1. **Read in this order**: `README.md` → this file (`doc/PLANNING.md`).
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
   - Tokens-as-sessions model (§3.4); multiple sessions per user
     coexist as multiple `tokens` rows.
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

## 1. Scope

- Single deployable server + embedded web PWA + CLI client.
- Multiple provisioned users supported, but no public sign-up.
- No multi-device awareness, no merge UI, no admin web UI.
- No CI, no automated tests; manual testing only in v1.
- v2 candidates (deferred): offline CLI, rich text, dark mode, multi-device
  locking, bulk sync endpoint.

## 2. Components

| Component | Path        | Language | Purpose                                          |
|-----------|-------------|----------|--------------------------------------------------|
| Server    | `api/`      | Go       | Persistence, auth, API, embedded web assets      |
| CLI       | `cli/`      | Go       | Online-only client over the same API             |
| Web       | `web/`      | TS + Vite| Local-first, offline-first PWA                   |
| Shared    | `shared/`   | Go       | DTOs, constants, schema version constant         |
| Docs      | `doc/`      | Markdown | Planning, design notes                           |

A root `go.work` wires the three Go modules. A `Makefile` provides
`build-server`, `build-cli`, `web-build`, `dev`.

## 3. Data Model

### 3.1 Entry

| Field                   | Type          | Notes                                                   |
|-------------------------|---------------|---------------------------------------------------------|
| `id`                    | VARCHAR(64)   | Client-generated UUIDv7; server only checks non-empty.   |
| `title`                 | TEXT          |                                                         |
| `url`                   | TEXT          |                                                         |
| `note`                  | TEXT          |                                                         |
| `tags`                  | TEXT (JSON)   | String array, `[]` allowed.                             |
| `created_at`            | DATETIME      | Server-assigned on first accept.                        |
| `updated_at`            | DATETIME      | Server-assigned on every accept.                        |
| `deleted_at`            | DATETIME NULL | Soft-delete tombstone.                                  |

Server stamps `created_at` and `updated_at`; client tracks a separate
internal `local_dirty_at` to drive sync. Each local dirty row also
stores `base_updated_at` for conflict detection.

### 3.2 User (meta DB)

| Field           | Type         | Notes                                      |
|-----------------|--------------|--------------------------------------------|
| `id`            | INTEGER PK   |                                            |
| `username`      | TEXT UNIQUE  |                                            |
| `password_hash` | TEXT         | argon2id (64MB / 1 iter / 2 parallelism).  |
| `active`        | BOOLEAN      | Default true.                              |
| `created_at`    | DATETIME     |                                            |

### 3.3 Tokens (meta DB)

| Field         | Type         | Notes                                                  |
|---------------|--------------|--------------------------------------------------------|
| `id`          | INTEGER PK   |                                                        |
| `user_id`     | INTEGER FK   |                                                        |
| `type`        | TEXT         | `access` or `refresh`.                                 |
| `value`       | TEXT         | Opaque random token (32-byte hex, unique).             |
| `expires_at`  | DATETIME     | Access 15 min, refresh 30 d.                           |
| `revoked_at`  | DATETIME NULL| Set on rotation or logout.                             |
| `created_at`  | DATETIME     |                                                        |

Refresh tokens are rotated on every `/auth/refresh` call (old token
revoked, new pair issued). No anti-theft IP hints.

### 3.4 Sessions

There is no separate `sessions` table. A **session is a row in the
`tokens` table**: every successful `/auth/login` or `/auth/refresh`
inserts a new `(access, refresh)` pair, and the access token is the
session reference for non-auth routes. Multiple sessions per user
co-exist naturally as multiple non-revoked `tokens` rows for that
`user_id`. Server holds no per-session state beyond the row itself.
Revocation is per-token via `revoked_at`.

Cross-token actions:

- `user set-password` → revoke **all** access + refresh tokens for the
  user.
- `user deactivate` → revoke **all** tokens + reject further logins.
- `user delete` → revoke all tokens, drop meta rows, delete per-user DB
  file.

### 3.5 Local web DB (sql.js)

Mirror of entry fields plus local sync columns:

- `local_dirty_at DATETIME NULL`
- `base_updated_at DATETIME NULL`

Schema version tracked locally; rebuilt on corruption with a sidecar
copy.

## 4. Project Layout

```
.
├── api/                # github.com/<you>/linkomat/api
│   ├── go.mod
│   ├── main.go
│   └── ...
├── cli/                # github.com/<you>/linkomat/cli
│   ├── go.mod
│   └── ...
├── shared/             # github.com/<you>/linkomat/shared
│   ├── go.mod
│   └── ...
├── web/                # Vite + TS PWA
│   ├── package.json
│   ├── vite.config.ts
│   └── src/
├── doc/                # Documentation (this file lives here)
├── go.work
├── Makefile
└── README.md
```

## 5. Server

### 5.1 Stack

- Go 1.22+, `net/http` + `http.ServeMux` (no third-party HTTP framework).
- GORM v2 with per-user SQLite DBs.
- Meta DB at `data/meta.db`; per-user DBs at `data/users/<username>.db`.
- DSN flags: `_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on`.
- Migrations: GORM `AutoMigrate` gated by a `schema_version` row.
- Logs: `slog` JSON to stderr; request lines only (`method`, `path`,
  `status`, `user_id`, `duration_ms`, `request_id`). Auth events logged
  with `event=login|refresh|logout|revoke`. No token contents logged.
  No entry-write audit logs in v1.

### 5.2 Configuration

Env vars:

- `LINKOMAT_LISTEN` (default `:8080`).
- `LINKOMAT_DATA_DIR` (default `./data`, mode 0700).
- `LINKOMAT_COOKIE_SECURE` (default true; cookies are always
  `SameSite=Strict`).
- `LINKOMAT_BEHIND_PROXY` (trusts `X-Forwarded-For` if set).
- `LINKOMAT_CORS_ORIGINS` (empty by default; same-origin served).

The server does **not** terminate TLS itself. It is expected to run
behind a TLS-terminating reverse proxy (Caddy, nginx, Traefik, etc.)
that handles HTTPS to the client. `LINKOMAT_BEHIND_PROXY` should be
set so client IPs and forwarded headers are honored, and the
`Secure` cookie flag can be kept at its `true` default since the
proxy serves the cookies over HTTPS even though the server-to-proxy
hop is plain HTTP.

### 5.3 Cookies

| Cookie            | Path             | Value         | Flags                              |
|-------------------|------------------|---------------|------------------------------------|
| `linkomat_auth`   | `/`              | access token  | `HttpOnly`, `SameSite=Strict`, `Secure` |
| `linkomat_refresh`| `/`              | refresh token | `HttpOnly`, `SameSite=Strict`, `Secure` |

Browser clients send both cookies automatically. CLI sends
`Authorization: Bearer <access_token>` for non-auth routes and uses the
refresh token in the JSON body for `/auth/refresh`.

### 5.4 Endpoints

Auth:

- `POST /auth/login` → JSON `{access_token, refresh_token, expires_in,
  username}`; also sets both cookies.
- `POST /auth/refresh` → rotates cookies + body. Body includes the
  refresh token (CLI sends in JSON; browser relies on cookie).
- `POST /auth/logout` → revokes current refresh token, clears cookies.

Entries (all require auth via header or `linkomat_auth` cookie):

- `GET /entries?limit=200&offset=0` → `{entries: [...], server_time}`.
- `GET /entries?changed_since=<RFC3339>` → `{entries: [...], server_time}`.
- `GET /entries/:id` → entry.
- `PUT /entries/:id` → full body overwrite; server stamps
  `updated_at = now()`; last-write-wins based on client intent
  (always accepted).
- `DELETE /entries/:id` → soft-delete, 204.

Misc:

- `GET /healthz` → `{status, version, server_time}`, no auth.
- `GET /version` → build info, no auth.

All `/api/*` (everything under `/entries`, `/auth/refresh`,
`/auth/login`, `/auth/logout`, `/healthz`, `/version`) responses carry
`Cache-Control: no-store, private`.

Default response headers:

- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: no-referrer`
- `Content-Security-Policy: default-src 'self'; script-src 'self'
  'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline';
  img-src 'self' data:; connect-src 'self'; base-uri 'none';
  frame-ancestors 'none'`
- `Strict-Transport-Security` (TLS only).

Errors are RFC 7807-shaped: `{type, title, status, detail}`.

### 5.5 Rate Limiting

In-memory token bucket on `/auth/login`: 5 attempts per minute per
(IP, username). 429 on excess.

### 5.6 User Management CLI

`linkomat-server` subcommands:

- `serve` (default).
- `user create <username>` (prompts password).
- `user set-password <username>`.
- `user deactivate <username>` (revokes tokens).
- `user activate <username>`.
- `user delete <username>` (hard delete: removes the user DB file in
  v1).
- `user list`.
- `user purge` (manual cleanup; v1 leaves expired tokens in place).
- `version`.

Deactivated users: `/auth/login` returns 403, existing tokens are
revoked.

## 6. Sync Protocol

### 6.1 Model

- Server is authoritative for `updated_at`.
- Clients track `local_dirty_at` and `base_updated_at` locally.
- Cursor-based delta sync using `changed_since`.

### 6.2 Flow

1. On app start, web client reads OPFS db, runs `PRAGMA integrity_check`.
   On failure, copy the file aside as `corrupt-<timestamp>.db` and
   create a fresh empty DB.
2. If first launch: push any dirty local entries (none), then pull
   `GET /entries` paginated with `limit=200&offset=0` until a short
   page arrives. Cursor starts at the `server_time` of the last page.
3. Subsequent syncs: push dirty entries (FIFO via `local_dirty_at`).
   Each push is a single `PUT /entries/:id`. Server stamps `updated_at`.
4. Then `GET /entries?changed_since=<cursor>` to pull deltas.
   `changed_since` is exclusive on the server.
5. Apply deltas: upsert by `id`, set local `updated_at` from server.
   If `deleted_at` is non-null, set `deleted_at` locally and hide from
   the UI.
6. Advance cursor to the `server_time` from the response.

### 6.3 Reachability

- Web client polls `navigator.onLine` and `GET /healthz` every 10 s.
- Only when both report "online" does the sync engine run.
- Retry policy on push failures: exponential backoff with jitter
  (1s → 60s cap). Reset on first success.
- Persistent failure > 5 min surfaces a "retry now" button.

### 6.4 Conflict Resolution

Last-save-wins by client intent. `PUT /entries/:id` always accepted
even if the server has changed since `base_updated_at`. Clients
overwrite without merge UI.

## 7. Web App

### 7.1 Stack

- Vue 3 SPA, TypeScript, Vite, `vite-plugin-pwa`.
- Pure HTML / browser components; planned in-house component framework
  later — keep components small and isolated.
- `sql.js` (pure WASM) for the local entry database.
- sql.js binary persisted in OPFS (no IDB fallback in v1).
- Self-hosted under `web/public/sqlite/`.

### 7.2 Persistence

- sql.js file at OPFS path `linkomat.db`.
- All entry queries go through sql.js.
- If OPFS read fails on startup: log to console + UI banner; the user
  is prompted to re-sync. (IDB fallback is intentionally not implemented
  in v1 per design.)
- `PRAGMA integrity_check` runs on startup; corrupt DB → sidecar copy +
  fresh DB.

### 7.3 PWA

- `manifest.webmanifest`: `name=Linkomat`, `short_name=Link`, theme +
  background colors, icons (192, 512, maskable).
- Service worker (workbox-generated) precaches app shell + sql.js
  assets only. No `/api/*` caching.
- No Background Sync API usage (iOS Safari gaps).
- Light mode only.

### 7.4 Auth on the Web

- Server sets `linkomat_auth` (Path `/`) and `linkomat_refresh`
  (Path `/`) cookies. Client never reads them in JS.
- Authorization header is **not** sent by the web app.
- Single in-flight refresh promise per session. On 401 from any
  non-auth route, attempt `/auth/refresh` once; on success, retry the
  original request; on failure, redirect to `/login`.
- On successful `/auth/refresh`, server rotates both cookies.

### 7.5 Routes

- `/login` — username + password form.
- `/` — home: virtualized list of recent entries (ordered
  `updated_at DESC`), search bar, FAB to add.
- `/entries/:id` — note editing view: plain `<textarea>`, `title`,
  `url`, `tags` chips, save-on-blur, delete button.
- `/settings` — username display, logout button, version info.

### 7.6 Sync Engine

**Not a Worker.** Despite the historical name "sync worker", the engine
(`web/src/sync/engine.ts`) runs on the **main thread** as a module-level
singleton. Nothing in the app constructs a `Worker`, and the workbox
Service Worker sets `runtimeCaching: []`, so `/entries` and `/healthz`
are never intercepted by it. All sync traffic is ordinary `fetch()` from
`web/src/api/client.ts` and is fully visible in the DevTools Network tab.

- One in-flight sync job per session.
- Triggered by:
  - App start.
  - `online` event.
  - Visibility change to visible.
  - Interval (5 min default) when online.
  - Local DB change (debounced 2 s) — a save or delete pushes shortly
    after the edit instead of waiting for the interval.
  - Manual "sync now" button.
- Drains the dirty queue serially, FIFO. Each entry shown with a tiny
  status dot: green (synced), yellow (unsynced), spinner (syncing).
  Header chip: `pending: N`.

### 7.7 Search

- Client-side over local sql.js DB.
- FTS3 virtual table (`entries_fts`) mirroring entries via
  `entries_ai`/`entries_ad`/`entries_au` triggers. Tags JSON syntax
  (`[`, `]`, `"`, `,`) is stripped in the trigger before insertion
  so that tokens inside the JSON string are indexed correctly
  (e.g. `["rust","web"]` indexes as `rust web`). The plan originally
  specified FTS5; `sql.js@1.14.2`'s WASM is compiled with
  `-DSQLITE_ENABLE_FTS3` only — FTS5 needs `-DSQLITE_ENABLE_FTS5`
  and an EMSDK fork build per `node_modules/sql.js/CONTRIBUTING.md`.
  FTS3 ships in the npm artefact and gives us proper tokenisation,
  implicit-AND, explicit-OR, and prefix matching at zero infra cost.
  LIKE-based search was rejected as the fallback because FTS3 was
  already available.
- Debounced 200 ms. Match against `title`, `url`, `note`, `tags`.
  Query syntax: implicit AND across tokens (`rust lang` matches docs
  with both); explicit `OR` / `NOT` operators supported by FTS3 are
  reserved for future use. The client appends `*` to each
  non-operator token so partial-word search works without typing
  `*`.

### 7.8 Editor

- Plain `<textarea>` for note (no autogrow in v1).
- Save-on-blur per field.
- No rich text, no preview in v1.

## 8. CLI

### 8.1 Stack

- Go 1.22+, cobra (single dependency kept low).
- `github.com/google/uuid` for UUIDv7 generation.
- `fsnotify` for editor file watching.
- HTTP client: `net/http`.

### 8.2 Configuration

- Server URL via `LINKOMAT_SERVER` env var (default
  `https://app.linkomat.example.com`).
- Credentials at `~/.config/linkomat/credentials.json`, mode 0600:
  ```json
  {
    "server": "...",
    "username": "...",
    "access_token": "...",
    "access_expires_at": "RFC3339",
    "refresh_token": "..."
  }
  ```
- No password cache.

### 8.3 Subcommands

- `linkomat login` — prompts username + password; stores tokens.
- `linkomat logout` — calls `/auth/logout`; clears local file.
- `linkomat add [args...]` — auto-detect positional args:
  - one positional URL → title = URL, url = URL.
  - two positionals → first is title, second is URL.
  - then prompts: `tags (one line, words separated):` (single line)
    and `note (multi-line, ctrl-d to finish):` (multi-line). Flags
    `--tags` / `--note` override prompts.
- `linkomat search <query>` — tabular output, ordered by `updated_at`
  desc.
- `linkomat ls [--tag ...]` — list entries, optional tag filter.
- `linkomat get <id>` — print a single entry as JSON.
- `linkomat edit <id-or-text>` — if input looks like an id, edit that
  entry directly; otherwise search title/url, present a selection
  list if multiple matches. Snapshot to a temp file with a `.md`
  template (`# @tag1 @tag2` header for tags, body for note), open in
  `$EDITOR`, watch with `fsnotify`, debounce 500 ms, PUT to server.
  Empty file → confirm delete.
- `linkomat rm <id>` — `DELETE /entries/:id`.
- `linkomat sync` — no-op for v1 (online-only).

Exit codes: `0` success, `1` server error, `2` auth required,
`3` invalid args, `4` not found.

## 9. Auth Contract Summary

| Client   | `/auth/login` | Non-auth routes     | `/auth/refresh`           |
|----------|---------------|---------------------|---------------------------|
| Web PWA  | Cookies       | `linkomat_auth` cookie | `linkomat_refresh` cookie |
| CLI      | JSON body     | `Authorization: Bearer <access_token>` | JSON body with refresh token |

Cookies are always `HttpOnly`, `SameSite=Strict`. `Secure` flag follows
`LINKOMAT_COOKIE_SECURE` (default true).

## 10. Build Checklist

Order of implementation (manual dev loop, no CI, no tests). The CLI
is intentionally the last step so the API surface stays stable while
the web app — the primary v1 client — drives the design first.

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
   9.3 status UI + backoff + retry (**next — see §13**).
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

### Session 1 — repo scaffold (step 1 of §10)
**Status**: done, committed (`1cab067`).
`go.work` (Go 1.26.1) wires `./api`, `./cli`, `./shared`. Makefile
with `build-server`/`build-cli`/`web-build`/`dev`/`tidy`/`all`/`clean`
(npm-touching targets source `$(HOME)/.nvm/nvm.sh` and `nvm use`).
`api/main.go` and `cli/main.go` print `shared.Version`. `web/`
scaffold: Vite 8 + Vue 3 + TS 6 + `vite-plugin-pwa` 1; PWA manifest
`theme_color=#2563eb`/`background_color=#ffffff`, light only; icons
generated from `favicon.svg` via `sips` (placeholders). Module path
owner `bylexus`. TS pinned to 6.0.3 because TS 7 dropped
`./lib/tsc` that `vue-tsc@3.3.10` needs.
**Next**: step 2.

### Session 2 — shared/ DTOs (step 2 of §10)
**Status**: done, committed (`c51fa80`).
`shared/entry.go` (`Entry` per §3.1), `shared/auth.go`
(`AuthResponse`), `shared/error.go` (`ErrorResponse` RFC 7807-shaped),
`shared/cookies.go` (cookie names + paths), `shared/schema.go`
(`CurrentSchemaVersion = 1`). `api`/`cli` `go.mod` declare
`require` + local `replace` for `./shared` so `make tidy` works on
workspace modules.
**Next**: step 3.

### Session 3 — server auth + meta DB + user CLI (step 3 of §10)
**Status**: done, committed (`2acc1c7`).
`api/config.go` (env-var `Config`), `api/models.go`
(`User`/`Token`/`SchemaVersion` GORM models), `api/meta.go`
(`data/meta.db` with §5.1 DSN flags), `api/auth.go` (argon2id
64MB/1/2, opaque 32-byte hex tokens), `api/cookies.go`,
`api/ratelimit.go` (5/min per (IP,user) on `/auth/login`),
`api/server.go` (security headers X-Content-Type-Options/
Referrer-Policy/X-Request-Id + slog JSON request log),
`api/handlers_auth.go` (`/auth/login`/`refresh`/`logout`),
`api/cmd_user.go`+`cmd_serve.go`+`cmd_version.go` (cobra,
`golang.org/x/term` password prompts), Bruno collection
`doc/bruno-api-tests/`, Makefile `api-test`/`api-test-req`/
`api-test-reset`.
Discrepancies:
- §3.3 had no `value` column on `tokens`; added `value TEXT`
  (opaque random hex, unique index) so presented tokens can be
  looked up. §3.3 table updated.
- §5.3 / §7.4 originally scoped `linkomat_refresh` to
  `Path /auth/refresh`; RFC 6265 cookies don't get sent to paths
  outside the cookie path, so `/auth/logout` couldn't revoke the
  browser's refresh. Changed to `Path /`; HttpOnly + Secure +
  SameSite=Strict still bound the exposure.
**Next**: step 4.

### Session 4 — drop server-side TLS termination (small change)
**Status**: done, committed (`01d74d3`).
Removed `TLSCert`/`TLSKey` config and the TLS-coupled `Secure`
cookie validation; `ListenAndServe` always plain HTTP. Server runs
behind TLS-terminating proxy; `Secure` cookie flag stays at its
`true` default. HSTS still ships on every response.
**Next**: step 4 (was deferred by this small change).

### Session 5 — per-user DB cache + entries endpoints (step 4 of §10)
**Status**: done, committed (`0c5bd1f`).
`api/userdb.go` (`data/users/<username>.db` with `UserDBs` lazy-open
mutex-guarded cache; `Entry` model with JSON-encoded `tags` TEXT;
`entryToDTO` UTC-normalises timestamps), `api/middleware_auth.go`
(`needsAuth` for `/entries*`; cookie first then `Bearer`), five
`/entries` handlers per §5.4 with `Cache-Control: no-store, private`,
`api/cmd_user.go` (`user delete` also removes `-shm`/`-wal`),
`shared/entries.go` (`EntriesListResponse`), Bruno `entries/` folder,
Makefile + env updates.
Decisions: tags as JSON in TEXT (no normalised table); list ordering
`updated_at ASC, id ASC` (stable for cursor sync); `changed_since`
exclusive; PUT un-deletes soft-deleted; body id may be empty or
match URL; `UserDBs` cache has no eviction in v1.
Discrepancies:
- List endpoint caps `limit` at 1000 and rejects `limit=0`/negative
  `offset` with 400 (defensive default; not in plan).
- `GET /entries/{id}` returns soft-deleted rows (with `deleted_at`
  populated) so the delta sync can replicate tombstones. Matches
  §6.2 step 5.
**Next**: step 5.

### Session 6 — reorder build steps, defer CLI
**Status**: plan update, committed.
§10 renumbered: CLI moved from step 6 to step 11. Web app drives the
API contract first; CLI lands against a stable contract.
**Next**: step 5 (was deferred).

### Session 7 — server embedded web assets + meta endpoints + security headers (step 5 of §10)
**Status**: done, committed.
`api/web_assets.go` (`//go:embed all:web_dist`; custom `ServeContent`
handler + SPA fallback; `mime.AddExtensionType(".webmanifest", ...)`
in `init()`), `api/handlers_health.go` (`/healthz`),
`api/handlers_version.go` (`/version`), `api/server.go` (full §5.4
security header set — HSTS always, CSP per §5.4; `Cache-Control`
moved out of middleware into `apiHeaders` + `writeError` so web
assets aren't anti-cached; `remote_ip` dropped from request log),
Makefile `web-embed` target (`cp -R web/dist/. api/web_dist/`),
`.gitignore` for `api/web_dist/`, Bruno `meta/` folder.
Discrepancies:
- `Cache-Control` not on web assets (workbox precache needs HTTP
  caching).
- HSTS always sent (browsers ignore on plain HTTP, travels through
  proxy).
- `remote_ip` log field removed to match §5.1 strictly.
Decisions: custom asset handler (not `http.FileServer`) to avoid the
`index.html` → canonical-redirect; trim trailing `/` before `fs.Stat`
because `embed.FS` rejects trailing-slash paths with "invalid
argument"; auth middleware inverted to positive `needsAuth`
(`/entries*`).
**Next**: step 6.

### Session 8 — fold Cache-Control Set calls into apiHeaders middleware (small cleanup)
**Status**: done, committed.
Per-route `apiHeaders` middleware wrapping instead of scattered `Set`
calls. `writeError` still sets `Cache-Control` because `authenticate()`'s
401/403 writes bypass the mux.
**Next**: step 6.

### Session 9 — web PWA scaffold + sql.js wrapper + OPFS persistence + integrity check (step 6 of §10)
**Status**: done, committed.

**What landed**

- `web/package.json` — added `sql.js@^1.14.2` dep and `postinstall`
  hook.
- `web/scripts/copy-sqljs-wasm.mjs` (new) — copies the **browser**
  build `sql-wasm-browser.wasm` (NOT `sql-wasm.wasm`, which is the
  Node build) from `node_modules/sql.js/dist/` into
  `web/public/sqlite/`. Output is gitignored.
- `web/.gitignore` — excludes generated `public/sqlite/sql-wasm-browser.wasm`.
- `web/src/sql.js.d.ts` (new) — local TypeScript declarations for
  `sql.js` (the package no longer ships its own types).
- `web/src/db/schema.ts` (new) — `CURRENT_SCHEMA_VERSION = 1`,
  `entries` DDL with `local_dirty_at` + `base_updated_at`, OPFS
  filename `linkomat.db`, sidecar prefix `corrupt-`.
- `web/src/db/opfs.ts` (new) — `opfsAvailable`, `readOpfsFile`,
  `writeOpfsFile`, `renameOpfsFile`, `removeOpfsFile`, `sidecarName`.
- `web/src/db/sqlite.ts` (new) — `LocalDb` with `open()`/`persist()`/
  `close()`. `open()` runs `PRAGMA integrity_check` after load; on
  failure or schema mismatch the file is sidecar-renamed and a fresh
  DB is created. Surfaces a `DbState` discriminated union (`pending`
  / `opfs-unavailable` / `ready` / `error`).
- `web/src/App.vue` — status panel driven by `DbState` with
  `data-test` hooks for future selectors.
- `web/vite.config.ts` — dropped `wasm` from `globPatterns` (SW no
  longer precaches the WASM); added
  `navigateFallbackDenylist: [/^\/sqlite\//]`.
- `api/server.go` — CSP `script-src` amended to
  `'self' 'wasm-unsafe-eval'` (required by `WebAssembly.instantiate`
  for sql.js).

**Discrepancies from the plan (called out per §0.2)**

- §5.4 CSP `script-src` was `'self'`; now
  `'self' 'wasm-unsafe-eval'`. The Bruno `meta/security-headers`
  test asserts only `default-src 'self'` and `frame-ancestors 'none'`,
  so it stays green.

**Decisions made in this session**

- `SQLJS_ASSET_VERSION` constant in `sqlite.ts:23` plus a `?v=…`
  cache-buster on the `locateFile` URL. Belt-and-suspenders so a
  stale SW precache (with HTML cached for the URL) cannot match —
  workbox's default `ignoreURLParametersMatching` only strips
  `utm_*` and `fbclid`, not arbitrary `v`. Bump when sql.js upgrades.
- SW config: `globPatterns` excludes `wasm`;
  `navigateFallbackDenylist` excludes `/sqlite/*`. The SW never
  serves anything under `/sqlite/*` from cache.
- `LocalDb` exposes a `DbState` union instead of throwing.
  `App.vue` renders a banner per state — easy to extend later.
- The bug that ate most of the session: sql.js's WASM loader calls
  `locateFile('sql-wasm-browser.wasm', …)`, NOT
  `locateFile('sql-wasm.wasm', …)`. The first postinstall wrote the
  Node build, the fetch hit a 404, the server's SPA fallback
  returned `index.html`, sql.js compiled HTML as WASM, magic bytes
  mismatch, fail. Symptom looked like a SW problem because the error
  message mentioned the SW's precache URL. Fix: copy the browser
  build.

**Smoke checks performed**

- `npm install` → postinstall copies WASM. `npm run build` clean,
  precache 12 entries (no WASM).
- `make build-server` clean; WASM in `api/web_dist/sqlite/`.
- `curl /sqlite/sql-wasm-browser.wasm?v=2` → `200`,
  `application/wasm`, 658 410 bytes, magic `00 61 73 6d`.
- `curl /sw.js` → `grep sql-wasm` returns 0;
  NavigationRoute has the denylist.
- DevTools console: `linkomat.db exists, size = 16384` (verified via
  the `try { await navigator.storage.getDirectory().then(d =>
  d.getFileHandle('linkomat.db')) … }` snippet — `keys()` is not
  reliably async-iterable in this Chrome build).
- `make api-test` → baseline `24/25 / 126/129` (only the documented
  stateful `login-rate-limited` fails).

**Next**: step 7 of §10 — web home view + virtualized list + search.

### Session 10 — web home view + virtualized list + search (step 7 of §10)
**Status**: done, committed.

**What landed**

- `web/package.json` — added `vue-router@^4.4.5`, `vue-virtual-scroller@^2.0.0-beta.8`.
- `web/src/db/schema.ts` — `CURRENT_SCHEMA_VERSION = 2`; added `entries_fts`
  (FTS3) virtual table + `entries_ai`/`entries_ad`/`entries_au` triggers
  that strip JSON syntax from `tags` before indexing.
- `web/src/db/sqlite.ts` — added `Entry` type, `getRawDb()`,
  `listEntries()`, `searchEntries()` (FTS3 MATCH). Read paths only; no
  write API (that is step 8).
- `web/src/db/index.ts` (new) — `useDb()` singleton + reactive `state`
  ref. Exposes `window.__linkomatDb` in dev mode (see below).
- `web/src/router.ts` (new) — `vue-router` with the single `/` route →
  `HomeView`.
- `web/src/views/HomeView.vue` (new) — search bar (200 ms debounce),
  `RecycleScroller` (fixed `item-size=96`), empty-state banners, FAB
  with `console.warn` no-op, fixed bottom-right on mobile.
- `web/src/main.ts` — installs router + `VueVirtualScroller`; calls
  `useDb()`.
- `web/src/App.vue` — minimal `<RouterView />` plus light-mode reset.
- `web/src/sql.js.d.ts` — added `getAsObject()` to the local
  `Statement` declaration.
- `web/.env.expose-dev` (new) — `VITE_EXPOSE_DEV=true` (committed
  build flag).
- `Makefile` — new `build-server-expose-dev` target.

**Discrepancies from the plan (called out per §0.2)**

- §7.7 said "FTS5 virtual table mirroring entries if the build
  supports FTS5; otherwise `LIKE`-based search." `sql.js@1.14.2`'s
  WASM is compiled with `-DSQLITE_ENABLE_FTS3` only (per
  `node_modules/sql.js/CONTRIBUTING.md`); FTS5 needs an EMSDK fork
  build. Falling back to FTS3 — built-in, no infra change, and
  delivers proper tokenisation, implicit AND, explicit OR, prefix
  matching. LIKE was rejected because FTS3 was already available.
  §7.7 updated accordingly.

**Decisions made in this session**

- `vue-router` with one route only (`/`) for step 7; `/login`,
  `/entries/:id`, `/settings` land in later steps.
- FAB is rendered but its click handler is `console.warn` until step
  8 wires it to the new-entry route.
- `vue-virtual-scroller` chosen over a hand-rolled windowed list
  (variable-height tolerance + battle-tested edge cases).
- `CURRENT_SCHEMA_VERSION` bumped to 2 so existing v1 OPFS DBs are
  sidecar-renamed and rebuilt with `entries_fts` (no data loss
  path; entries are persisted to a `corrupt-v1-<ts>.db` sidecar in
  OPFS).
- Dev global mechanism: `import.meta.env.DEV || import.meta.env.VITE_EXPOSE_DEV`.
  Two ways to expose the global:
  - `npm run dev` (Vite dev server on :5173) → `DEV = true`.
  - `make build-server-expose-dev` (Go server on :8080) → reads
    `.env.expose-dev` via `--mode expose-dev`, which sets
    `VITE_EXPOSE_DEV = true` in the embedded bundle.
  Default `make build-server` (production) leaves both flags false
  and the global is tree-shaken out.
- Tags JSON syntax (`[`, `]`, `"`, `,`) replaced with spaces in the
  FTS3 trigger so JSON punctuation doesn't tokenise as a single
  blob. `["rust","web"]` becomes ` rust web ` → tokens `rust` and
  `web`.
- `toFtsQuery()` strips stray `"` and `'` from each token and
  appends `*` for prefix matching; reserved FTS3 operators
  (`AND`/`OR`/`NOT`/`NEAR`) are preserved uppercase and not
  suffixed. Empty query falls back to `listEntries()`.
- Smoke script `web/scripts/fts3-impl-smoke.cjs` was used during
  development to verify FTS3 behaviour in the Node WASM build, then
  deleted before commit.

**Smoke checks performed**

- `npm run build` clean; bundle 172.2 kB / gzip 62.1 kB.
- `make build-server` clean; embedded 11 files; binary builds.
- `make build-server-expose-dev` clean; `grep __linkomatDb
  api/web_dist/assets/*.js` → 1.
- Server-side smoke: `/` → 200 (SPA HTML); `/manifest.webmanifest` →
  200; `/assets/index-…js` → 200; `/sqlite/sql-wasm-browser.wasm?v=2` →
  200, `application/wasm`, 658 410 bytes.
- `make api-test` → baseline `24/25 / 126/129` (unchanged; web-only
  change).
- Manual seed via `__linkomatDb.raw.exec(...)` + `__linkomatDb.persist()`
  + reload: three sample entries render; search `rust` / `db` /
  `sqlite` / `rust lang` / `rust OR vue` all match expected rows;
  soft-delete via `UPDATE entries SET deleted_at = …` removes the
  row from the list (FTS index still holds it; the `deleted_at IS
  NULL` filter excludes it).

**Next**: step 8 of §10 — web note editing view + save-on-blur +
delete. Context for the next session:

- The edit route is `/entries/:id` (per §7.5); register it in
  `web/src/router.ts`. The dev-only no-op FAB in `HomeView.vue`
  gets wired to `router.push(`/entries/${newId()}`)` where
  `newId()` mints a UUIDv7 client-side (no API call yet).
- Reuse `LocalDb.listEntries()` / `searchEntries()` for the
  entry list (no new read APIs needed). Add `LocalDb.upsertEntry(entry)`
  and `LocalDb.softDeleteEntry(id)` — both stamp
  `local_dirty_at = now()` and `base_updated_at = current
  updated_at` per §6.1; no server call yet (sync is step 9).
- Save-on-blur per field: title input, url input, `<textarea>`
  for note, tags chips. No rich text, no preview (§7.8).
- Delete button on the edit view; confirmation prompt.
- Search bar / FAB continue to work; no changes to `HomeView.vue`
  other than wiring the FAB.
- Keep `import.meta.env.DEV || import.meta.env.VITE_EXPOSE_DEV`
  gate in `useDb()`; the dev global is still useful in step 8 for
  manual entry inspection.
- `CURRENT_SCHEMA_VERSION` stays at 2 unless a new column / table
  is needed for the edit view.

### Session 11 — web note editing view + save-on-blur + delete + URL link (step 8 of §10)
**Status**: done.

**What landed**

- `web/src/utils/uuid.ts` (new) — `uuidv7()` using `crypto.getRandomValues()`,
  48-bit ms timestamp, version 7 nibble, variant 10 nibble.
- `web/src/utils/url.ts` (new) — `normalizeUrl(input)`: empty → empty;
  existing scheme (`http`, `https`, `ftp`, `mailto`, `javascript`, …)
  → unchanged; leading `//` → prepended with `https:`; else prepended
  with `https://`. Trimmed.
- `web/src/db/sqlite.ts` — adds `getEntry(id)`, `upsertEntry(input)`,
  `softDeleteEntry(id)`. `upsertEntry`/`softDeleteEntry` auto-persist
  and dispatch `window` `CustomEvent('linkomat-db-change')` on
  success. Exports `DB_CHANGE_EVENT` for listeners. New row on insert:
  `created_at = updated_at = local_dirty_at = now`, `deleted_at = NULL`,
  `base_updated_at = NULL`. On update: `updated_at = local_dirty_at =
  now`, `base_updated_at = previous updated_at`. Soft delete:
  `deleted_at = updated_at = local_dirty_at = now`,
  `base_updated_at = previous updated_at`.
- `web/src/db/index.ts` — `__linkomatDb` dev global now exposes
  `getEntry`, `upsertEntry`, `softDeleteEntry` alongside the read
  methods.
- `web/src/router.ts` — adds `/entries/:id` route → `EditView`.
- `web/src/views/EditView.vue` (new) — title input, url input,
  tags chips (Enter / `,` / blur to add, `×` to remove), note
  `<textarea>` (12 rows). Save-on-blur per field via `onUrlBlur` /
  `saveField`. New entry: nothing is written until the first non-empty
  blur (`isFormEmpty()` early-return). Delete button only shown after
  first save; new-but-unsaved entries discard on Delete without a
  confirm dialog. Back arrow + router.push to `/`. Errors surface in
  a single inline line. Minimal styling — basic borders, focus
  outline, system fonts.
- `web/src/views/HomeView.vue` — FAB now
  `router.push(\`/entries/\${uuidv7()}\`)`; rows are clickable
  (`onRowClick` → EditView); URL line rendered as
  `<a :href="normalizeUrl(item.url)" target="_blank" rel="noopener noreferrer" @click.stop>`.
  `click.stop` keeps row-click → EditView from firing when the link
  itself is clicked. Empty URL falls back to `<span>` for layout.
  `.row-url` styled `color:#2563eb; text-decoration:underline`.
  `onMounted` now calls `reload()` (Vue `watch` does not fire on
  initial mount; without this, HomeView showed blank after navigating
  back from EditView). Listens for `DB_CHANGE_EVENT` and reloads on
  remote writes too.

**Discrepancies from the plan (called out per §0.2)**

- §7.5 says `save-on-blur per field`; this is implemented for title,
  url, note and the tag chip add/remove path. Tags use Enter / `,` /
  blur to commit a typed tag, then `saveField()` runs.
- §7.5 mentions a `tags chips` UI but does not define interaction;
  the "type-and-Enter / `,` / blur to add, `×` to remove" pattern was
  chosen and is now part of the contract.
- URL normalization is applied on blur in `EditView` AND defensively
  on render in `HomeView` (so legacy entries stored without a scheme
  still open the right URL). The plan did not call for display-side
  normalization; flag it as an additional behaviour.
- `upsertEntry` / `softDeleteEntry` auto-persist and dispatch a
  `linkomat-db-change` window event so HomeView refreshes without a
  shared reactive store. Plan said "stamps local_dirty_at = now() and
  base_updated_at = current updated_at"; the persist + event are
  added glue, not schema changes.

**Decisions made in this session**

- UUIDv7 implemented client-side in `web/src/utils/uuid.ts`. No new
  dependency. `crypto.getRandomValues()` is available in every
  browser that has OPFS, which the rest of the app already requires.
- `click.stop` on the URL link keeps the row's `onRowClick`
  (`router.push(/entries/:id)`) from firing when the user clicks the
  link. The link's default action (open href in a new tab via
  `target="_blank"`) is not prevented.
- `target="_blank"` + `rel="noopener noreferrer"` — standard
  safe-new-tab attributes.
- HomeView reloads on `linkomat-db-change` even when the source of the
  event is the current session. Cheap (a `listEntries()` call) and
  means a single code path covers both local and remote mutations.
- EditView `onDelete` on a never-saved new entry skips the confirm
  dialog — there is nothing to lose.
- Tag input uses Enter, `,` and blur as commit triggers; all three
  funnel through `addTag()` which dedupes and then calls
  `saveField()`.
- `CURRENT_SCHEMA_VERSION` stays at 2 — no schema change.

**Smoke checks performed**

- `npm run build` clean (vue-tsc + vite); 179.20 kB / gzip 64.23 kB.
- `make build-server` clean, 12 precache entries, 11 embedded files.
- `make api-test` baseline `24/25 / 126/129` (unchanged; web-only).
- One-off Node 24 smoke (since-deleted) confirmed UUIDv7 shape,
  `base_updated_at` preservation across update, `local_dirty_at`
  stamping, soft-delete tombstone, FTS3 indexing, schema v2.
- `normalizeUrl` smoke run: `"" → ""`, `"foo" → "https://foo"`,
  `"www.google.ch/foo" → "https://www.google.ch/foo"`,
  `"//foo.com/path" → "https://foo.com/path"`,
  `"https://foo" → "https://foo"`, `"  foo  " → "https://foo"`.
- Manual: create / edit / tag-add / tag-remove / delete / URL
  normalization on blur / click URL → new tab / click row → EditView
  / navigate back → list reloads without page refresh.

**Next**: step 9 of §10 — web sync worker (dirty queue, delta pull,
full pull on first launch, retry/backoff, status UI).

### Session 12 — web sync worker part 1: API client, auth flow, Login view, per-user OPFS (step 9.1 of §10)
**Status**: in progress (9.1 implementation done; awaiting review).

Per the plan written in §13, step 9 is split into 9.1 (auth + login view +
per-user OPFS), 9.2 (sync worker + cursor + reachability), and 9.3 (status
UI + backoff). Sub-steps approved.

**Files added (9.1)**
- `web/src/api/client.ts` — fetch wrapper with single in-flight refresh on
  401; `setLogoutHandler` callback; `apiGet`/`apiPost`/`apiPut`/`apiDelete`
  return `ApiResult` (`ok` / `http-error` / `network-error`).
- `web/src/api/auth.ts` — `login`, `logout`, `probe` (single
  `GET /entries?limit=1`).
- `web/src/state/auth.ts` — reactive `useAuth()` with `AuthState`
  (`unknown` / `logged-in` / `logged-out`), `username` ref, persisted
  via `localStorage` key `linkomat.current_username`. `setLogoutHandler`
  is installed at module load.
- `web/src/views/LoginView.vue` — username + password form, error line,
  submit disabled while pending.

**Files modified (9.1)**
- `web/src/db/schema.ts` — replaces `OPFS_DB_FILENAME` /
  `OPFS_SIDECAR_PREFIX` constants with `opfsDbFilename(username)` /
  `opfsSidecarPrefix(username)` helpers.
- `web/src/db/sqlite.ts` — `LocalDb(username)`; `persist()` uses the
  per-user filename.
- `web/src/db/index.ts` — `useDb(username: string | null)` recreates the
  singleton on username change; new `closeDb()`.
- `web/src/views/HomeView.vue` — temporary logout button in header
  (home for /settings, which is a separate step).
- `web/src/views/EditView.vue` — accepts the username.
- `web/src/router.ts` — `/login` route.
- `web/src/App.vue` — auth guard + loading splash.
- `web/src/main.ts` — loads `state/auth` for the side-effect of
  installing the logout handler.

**Decisions**
- Auth state module is loaded before any view in `main.ts` so the
  `setLogoutHandler` callback is in place before the first API call.
- `tryRefresh` releases `refreshInFlight` in `finally` so a failed
  refresh doesn't poison the next attempt.
- `markLoggedOut` is the single logout sink; both `logout()`
  (user-initiated) and the API client's `setLogoutHandler` (refresh
  failed) call it.

**Known limitations**
- Direct browser navigation (refresh) to `/entries/:id` returns 401
  because the Go mux matches `/entries/{id}` before the SPA fallback.
  The SPA's pushState navigation is unaffected. A proper fix would
  route on `Accept: text/html`; deferred.
- The "Username" / "Logout" header chip in HomeView is a temporary
  placement. The plan's `/settings` route (with username display,
  logout, version) is a separate step.

**Smoke checks**
- `npm run build` clean (Node 24). Bundle 183.95 kB / gzip 65.74 kB.
- `make build-server` clean, 11 embedded files.
- `make api-test` baseline `24/25 / 126/129` (the documented stateful
  login-rate-limited failure).
- curl: login → 200 + cookies; list with cookies → 200; logout → 204;
  refresh (cookie) → 200; refresh (JSON body) → 200.
- SPA web assets: `/` → 200, `/login` → 200, `/entries/abc` → 401
  (known limitation), `/manifest.webmanifest` → 200,
  `/sqlite/sql-wasm-browser.wasm?v=2` → 200.

**Next**: step 9.2 of §10 — sync worker + cursor + reachability.

### Session 13 — web sync engine (step 9.2 of §10)
**Status**: done, all sub-steps reviewed and accepted 2026-08-16.

Step 9.2 landed in one implementation pass followed by a fix round
(9.2a–9.2f) after manual testing found the sync silently applying
nothing. See §13 → Step 9.2 for the full record, including the two
traps worth remembering:

- **sql.js rejects `undefined` bind parameters** (only `null` maps to
  SQL NULL). A Go DTO with `json:"…,omitempty"` on a nil pointer omits
  the key entirely, which arrives as `undefined` and throws
  `Wrong API use : tried to bind a value of an unknown type`.
- **Do not expose a Vue state snapshot for debugging.** `getSyncState()`
  returned `state.value`, and the dev global captured it once, so every
  console reading was frozen at login time and actively misleading.

**Files added**: `web/src/api/entries.ts`, `web/src/state/sync.ts`,
`web/src/sync/reachability.ts`, `web/src/sync/engine.ts`.

**Files modified**: `web/src/db/schema.ts` (schema v3 + `sync_state`),
`web/src/db/sqlite.ts` (cursor / dirty-queue / server-apply methods),
`web/src/main.ts`, `web/src/sql.js.d.ts`, `shared/entry.go`,
`api/handlers_entries.go`, `doc/bruno-api-tests/entries/entries-create.bru`,
`doc/bruno-api-tests/entries/entries-update.bru`.

**Smoke checks**: `make build-server-expose-dev` clean;
`make api-test` baseline `24/25 · 126/129`; full manual acceptance run
per §13 Step 9.2 passed.

**Next**: step 9.3 of §10 — status UI, backoff, retry. §13 → Step 9.3
carries the hand-off context.
---

## 13. Step 9 Plan — Web sync engine

**Ticket/Source:** doc/PLANNING.md §10 step 9
**Branch:** none
**Status:** in progress — 9.1 and 9.2 done and reviewed; **9.3 is the next
step** (status UI, backoff, retry).

### Goal
Wire the local sql.js DB to the server: auth flow, periodic sync, dirty-queue
push, delta pull, full pull on first launch, reachability gating, backoff on
push failures, and status UI.

### Scope
**In scope:**
- Per-user OPFS file keying (`linkomat.<username>.db`).
- Login view + reactive auth state + redirect to `/login` on auth failure.
- API client with single in-flight refresh on 401 (per §7.4).
- Sync state (cursor, pending count, online, syncing, last error,
  persistentFailure).
- Sync engine: push dirty FIFO; delta pull (single `changed_since` call);
  full pull on first launch (paginated `limit=200&offset=0` until short page).
- Reachability: `navigator.onLine` + `/healthz` poll every 10 s (§6.3).
- Triggers: app start, `online` event, visibility change, 5 min interval,
  local DB change (2 s debounce), manual "sync now" (§7.6).
- Backoff with jitter (1 s → 60 s cap) on push failures; reset on success.
- "Retry now" button after 5 min continuous failure.
- Status UI: pending-count chip in home header + per-row status dot
  (green / yellow / spinner).
- Dev-mode global `__linkomatSync` exposing the sync engine for manual
  inspection.

**Out of scope:**
- Real-time push from server (no WebSocket / SSE).
- Multi-device awareness, per-device locks (§11 deferred).
- Bulk sync endpoint (§11 deferred).
- `/whoami` endpoint — auth state is implicit.
- Soft-delete push semantics: server-side entry always wins (last-save-wins
  per §6.4). Delta pull overwrites local dirty rows including dirty flags.

### Decision log
- **Sub-step breakdown:** 3 sub-steps (9.1 auth, 9.2 sync engine, 9.3 UI).
  9.2 additionally needed a fix round (9.2a–9.2f) after manual testing;
  those are folded into the Step 9.2 record below.
- **Local DB per user:** `linkomat.<username>.db` in OPFS. Username from
  `localStorage` key `linkomat.current_username`. Logout clears the
  localStorage key but leaves the OPFS file in place (next login as the
  same user reuses it).
- **Cursor storage:** `sync_state` table inside `linkomat.<username>.db`.
  Schema v3. First sync (empty cursor) triggers full pull; subsequent syncs
  use `GET /entries?changed_since=<cursor>`.
- **Auth state detection:** implicit via the first sync attempt. No
  `/whoami` endpoint. In 9.1, a minimal one-shot `GET /entries?limit=1`
  probe stands in for the engine; in 9.2 the sync engine takes over.
- **Sync push for soft-deleted entries:** send `DELETE /entries/:id`
  (not `PUT`). Local rows with `deleted_at` set are pushed as
  DELETE; others are PUT. 404 from DELETE is treated as success.
- **Delta pull:** single call per plan. Server returns all rows with
  `updated_at > cursor` (no limit on the changed_since path).
- **Status UI placement:** status dot in the top-right of each row;
  pending count chip in the home header next to the search bar.
- **Testing strategy:** manual smoke checks per sub-step. No automated
  tests in v1 (§1).
- **`deleted_at` null contract (Session 15):** fixed on **both** sides —
  the server drops `omitempty` so the key is always present, and the
  client binds `entry.deleted_at ?? null`. — *Rationale:* the explicit
  wire contract documents the intent for the CLI too, and the client-side
  guard means the local DB layer never trusts the JSON shape again.
- **Bruno assertions for the new contract (Session 15):** `entries-create`
  and `entries-update` change `deleted_at: isUndefined` to `isNull`
  rather than dropping the assertion. — *Rationale:* keeps the coverage
  and makes the suite document the new contract.
- **Dev-global sync state (Session 15):** `__linkomatSync.state` exposes
  the reactive `Ref`; the console idiom is `__linkomatSync.state.value`.
  — *Rationale:* matches `__linkomatSync.reachability.value`, which
  already worked that way; the previous snapshot froze at login and
  misled a whole debugging session.
- **`lastSkipReason` (Session 15):** `syncOnce()`'s four early returns
  record why they skipped (`in-flight` / `no-username` / `offline` /
  `db-not-ready`); cleared on a successful cycle. — *Rationale:* the
  silent returns were the main reason the failure was opaque; 9.3's
  status chip can read the field too.
- **Batch apply (Session 15):** `applyServerEntries(entries[])` does one
  transaction, one `persist()`, one `DB_CHANGE_EVENT`. — *Rationale:*
  per-entry `persist()` exported the whole database once per row.
  `clearDirty()` deliberately keeps its per-entry persist so a crash
  mid-push does not replay accepted writes.
- **UPDATE-or-INSERT instead of `INSERT OR REPLACE` (Session 15):**
  — *Rationale:* REPLACE does not fire the `AFTER DELETE` trigger
  (recursive triggers off) and reallocates the rowid, orphaning
  `entries_fts` docids on every sync.
- **Push on local save (Session 15):** a local DB change schedules a
  `syncOnce()` debounced by 2 s, added as a sixth trigger in §7.6.
  — *Rationale:* save-on-blur editing produced no network traffic for up
  to 5 minutes, which read as a broken sync. Debounced rather than
  immediate so one editing session coalesces into a single push instead
  of one per blurred field.
- **Debounce loop suppression (Session 15):** `onDbChange()` skips
  scheduling while `syncInFlight`; `syncOnce()` re-arms only on a
  *successful* cycle that still has dirty rows. — *Rationale:* the sync
  path dispatches `DB_CHANGE_EVENT` itself, so without the first rule
  every cycle would schedule the next. Gating the re-arm on success
  avoids a 2-second retry loop while the server is down; backoff is
  9.3's job.
- **`sync/worker.ts` → `sync/engine.ts` (Session 15):** — *Rationale:*
  the module runs on the main thread and constructs no `Worker`; the name
  sent the developer looking for a worker's network traffic that never
  existed. §7.6 now states the threading model explicitly.
- **Sync tracing declined (Session 15):** no `console.debug` cycle
  logging and no ring buffer. — *Rationale:* the Network tab plus
  `__linkomatSync.state.value` is sufficient now that the state ref is
  live and skips are labelled.
- **`server_time` stamped before the query (Session 15):** — *Rationale:*
  stamping after the query loses any entry written in the gap, because
  `changed_since` is exclusive. Re-pulling a row already held is
  harmless; the apply path is idempotent.

### Steps

#### Step 9.1 — API client, auth flow, Login view, per-user OPFS
- **Status:** done (reviewed 2026-08-16)
- **What:**
  - `web/src/api/client.ts` — fetch wrapper with single in-flight
    refresh on 401; `setLogoutHandler` callback so the auth state can
    react to refresh failures. `apiGet`/`apiPost`/`apiPut`/`apiDelete`
    return a typed `ApiResult` (`ok` / `http-error` / `network-error`).
  - `web/src/api/auth.ts` — `login()`, `logout()`, `probe()`.
    Login throws with a human-readable message for 401/403/429.
  - `web/src/state/auth.ts` — reactive `useAuth()` returning
    `{ state: 'unknown' | 'logged-in' | 'logged-out', username,
    login(), logout(), probe(), markLoggedOut() }`. Username persisted
    in `localStorage` under `linkomat.current_username`. On logout the
    LocalDb is closed via `closeDb()`.
  - `web/src/views/LoginView.vue` — username + password form, error
    line, submit disabled while pending. `data-test` hooks
    `login-form`, `login-username`, `login-password`, `login-submit`,
    `login-error`.
  - `web/src/router.ts` — adds `/login` route.
  - `web/src/App.vue` — auth guard: `watch` on `[state, route.path]`
    redirects to `/login` when logged-out and elsewhere, redirects to
    `/` when logged-in and on `/login`. Shows a centered "Loading…"
    while `state === 'unknown'`. Triggered `probe()` on mount.
  - `web/src/db/schema.ts` — replaces `OPFS_DB_FILENAME` / `OPFS_SIDECAR_PREFIX`
    constants with `opfsDbFilename(username)` / `opfsSidecarPrefix(username)`
    helpers. `CURRENT_SCHEMA_VERSION` stays at 2.
  - `web/src/db/sqlite.ts` — `LocalDb` constructor takes a username;
    `persist()` and `open()` read the per-user filename / sidecar
    prefix from the helpers.
  - `web/src/db/index.ts` — `useDb(username: string | null)` recreates
    the singleton when the username changes. New `closeDb()` closes the
    current instance, resets the state to `pending`.
  - `web/src/views/HomeView.vue` — adds a temporary logout button in
    the header (`home-meta`, `home-username`, `home-logout` selectors)
    so a /settings route is not required for 9.1. `data-test` hook for
    the username.
  - `web/src/views/EditView.vue` — calls `useDb(username.value)`.
  - `web/src/main.ts` — loads `./state/auth` for the side-effect of
    installing the logout handler on the API client.
- **Acceptance criteria:**
  - `npm run build` clean (Node 24). Bundle 183.95 kB / gzip 65.74 kB.
  - `make build-server` clean.
  - `make api-test` baseline `24/25 / 126/129` (the documented
    stateful rate-limit failure).
  - Manual: log in as alice → cookies set, redirect to `/`,
    HomeView shows local DB (empty).
  - Manual: log out → cookies cleared, redirect to `/login`.
  - Manual: clear cookies, reload → probe 401, stays on `/login`.
  - Manual: log in as alice, create a local entry, log out, log in
    as bob → HomeView shows empty list (separate OPFS file).
  - Manual: log out bob, log back in as alice → HomeView shows the
    entry.
- **Notes:**
  - `LocalDb` exposes `username` as a readonly field on the instance
    so tests / debugging can inspect which user owns the DB.
  - Auth state module loads before any view in `main.ts` so the
    `setLogoutHandler(markLoggedOut)` call installs the handler
    before the first API call.
  - The `apiFetch` `tryRefresh` clears `refreshInFlight` in a
    `finally` so a failed refresh doesn't poison the next attempt.
  - Direct browser navigation to `/entries/:id` (refresh) currently
    returns 401 (the mux matches `/entries/{id}` before the SPA
    fallback). The SPA's pushState navigation is unaffected. A
    proper fix would route on `Accept: text/html`; deferred.
  - The "Username" / "Logout" header chip in HomeView is a temporary
    placement. The plan's `/settings` route (with username display,
    logout, version) is a separate step.

#### Step 9.2 — Sync engine, cursor, reachability
- **Status:** done (reviewed 2026-08-16)
- **What landed:**
  - `web/src/db/schema.ts` — `CURRENT_SCHEMA_VERSION = 3`; `sync_state`
    table (single row, `id = 1`, `cursor` TEXT).
  - `web/src/db/sqlite.ts` — `getCursor()`, `setCursor()`,
    `listDirtyEntries()` (FIFO by `local_dirty_at`), `countDirtyEntries()`,
    `applyServerEntries(entries[])` / `applyServerEntry(entry)`,
    `clearDirty(id, serverUpdatedAt)`. `ServerEntry` type exported.
  - `web/src/api/entries.ts` — `listEntries({ limit, offset, changed_since })`,
    `putEntry(id, body)`, `deleteEntry(id)`, all returning `ApiResult`.
  - `web/src/sync/reachability.ts` — module-level `online` ref; polls
    `navigator.onLine` + `/healthz` (5 s timeout) every 10 s; honours
    `online` / `offline` window events.
  - `web/src/state/sync.ts` — reactive `SyncState` (`online`, `syncing`,
    `pendingCount`, `lastError`, `lastSkipReason`, `lastSyncAt`,
    `persistentFailure`). `syncStateRef()` returns the **Ref**;
    `useSyncState()` a computed view; `setSyncState(patch)` to update.
  - `web/src/sync/engine.ts` — `initSync()` singleton. `syncOnce()` runs
    push-then-pull; `__linkomatSync` exposed in dev builds with
    `syncNow`, `reachability`, `state`.
  - `shared/entry.go`, `api/handlers_entries.go` — see the two server
    changes under **Decisions** below.
  - `web/src/main.ts` — calls `initSync()` before mount.

- **Sub-steps** (all reviewed 2026-08-16): 9.2a instruments · 9.2b
  `deleted_at` contract · 9.2c batch apply + stable rowid · 9.2d server
  cursor race · 9.2f push-on-save + rename · 9.2e full acceptance run.

- **The failure that cost a session, and why:** after the first
  implementation pass the HomeView stayed empty while the server held
  entries. Root cause: `shared.Entry.DeletedAt` was
  `json:"deleted_at,omitempty"`, so a non-deleted entry serialised
  **without** the key; that arrived as `undefined` and **sql.js throws on
  `undefined` bind parameters** (only `null` maps to SQL NULL) —
  `Wrong API use : tried to bind a value of an unknown type (undefined).`
  The pull threw on its first row and `syncOnce()`'s catch swallowed it.
  Diagnosis was blocked because `getSyncState()` returned a plain object
  *snapshot* that the dev global captured once at login, so every console
  reading was frozen and misleading. **Both traps are easy to
  re-introduce — do not expose a Vue state snapshot for debugging, and do
  not trust an optional JSON key to arrive as `null`.**

- **Decisions taken during the step:** all recorded in the **Decision log**
  above (the `deleted_at` contract, `server_time` stamping, UPDATE-or-INSERT,
  batch apply, the 2 s push debounce and its loop suppression, and the
  `worker` → `engine` rename). Two that are engine-local:
  - Push semantics: rows with `deleted_at` set go out as
    `DELETE /entries/:id` (404 treated as success); all others as
    `PUT /entries/:id`. `applyServerEntry` overwrites local dirty flags
    (§6.4 last-save-wins by server).
  - `waitForDbReady()` watches the LocalDb state ref with a 30 s timeout;
    returns `null` on `error` / `opfs-unavailable`.

- **Verified:** `make build-server-expose-dev` clean; `make api-test`
  baseline `24/25 · 126/129` (only the documented stateful
  `login-rate-limited` failure). Manual: first launch full pull with
  cursor saved; local create → `pendingCount` 1 → push → 0 and present on
  the server; server-side edit and delete replicate down; server kill →
  `online: false` and `lastSkipReason: "offline"`, restart → resumes
  within the 10 s poll; save → one `PUT` within ~2 s with no repeating
  `/entries` traffic afterwards; logout/login re-attaches listeners.

#### Step 9.3 — Status UI, backoff, retry
- **Status:** open — **this is the next step. Start here.**

- **What:**
  - `web/src/sync/engine.ts` — backoff with jitter on push failures:
    delay = `min(60s, 2^attempt * 1s)` ± 20 % jitter; reset on first
    success. Track `firstFailureAt`; set `persistentFailure = true` once
    `now - firstFailureAt > 5 min`.
  - `web/src/state/sync.ts` — `persistentFailure` already exists in
    `SyncState`; wire it from the backoff timer rather than from the
    current `catch` block (which sets it on the first failure — that is
    a 9.2 placeholder and **must be corrected** in 9.3).
  - `web/src/views/HomeView.vue` — header chip area next to the search
    bar: `pending: N` when `pendingCount > 0`, plus a state label
    (`online` / `offline` / `syncing` / `error`). "Retry now" button
    shown when `persistentFailure`. Per-row status dot in the row's
    top-right: green (synced, `local_dirty_at IS NULL`), yellow (dirty),
    spinner (syncing).
  - `web/src/views/EditView.vue` — header echo of the pending count and
    sync state for the current entry.

- **Acceptance criteria:**
  - `npm run build` / `make build-server` clean.
  - `make api-test` baseline `24/25 · 126/129`.
  - Create 3 entries with the server stopped → chip shows `pending: 3`;
    start the server → spinner during push, count drops to 0.
  - Stop the server → push fails, chip shows `error`, retries visibly
    space out (1s → 2s → 4s … capped at 60s, not a tight loop); after
    5 min the "Retry now" button appears and forces a cycle.
  - Row dots: green for synced, yellow for dirty, spinner during sync.
  - Edit an existing entry → its dot turns yellow; after the 2 s
    debounced push it returns to green.

- **Hand-off notes for the next session:**
  - **Read first:** §7.6 (triggers + the "not a Worker" note), §6.3
    (reachability + retry policy), then §13 Step 9.2 above for the
    decisions the engine already encodes.
  - **Imports you need:** `syncNow` and `initSync` from
    `web/src/sync/engine.ts`; `useSyncState()` from
    `web/src/state/sync.ts` (returns a `ComputedRef<SyncState>` — use
    this in components). `syncStateRef()` returns the raw Ref and is what
    the dev global publishes; prefer `useSyncState()` in views. A
    `useSync()` wrapper existed in 9.2 and was removed as unused — wire
    the "Retry now" button straight to `syncNow()`.
  - **The dirty flag for the row dot** is `local_dirty_at IS NOT NULL` on
    the `entries` row; `LocalDb.listEntries()` already returns
    `local_dirty_at` on every `Entry`, so no new query is needed.
  - **HomeView already reloads** on `DB_CHANGE_EVENT`
    (`web/src/db/sqlite.ts` exports `DB_CHANGE_EVENT`), so dots refresh
    without extra plumbing.
  - **Do not add a second retry timer.** The 2 s push debounce
    (`schedulePush()` / `cancelPush()`) and the 5 min interval already
    exist in `engine.ts`; backoff must sit in the same place and respect
    `syncInFlight`, or the loop-suppression invariants documented in
    Step 9.2 break.
  - **Dev global:** `__linkomatSync.state.value` (live Ref),
    `__linkomatSync.reachability.value`, `await __linkomatSync.syncNow()`.
    Local DB: `__linkomatDb.raw.exec("SELECT id,title,local_dirty_at FROM entries")`.
    Both require `make build-server-expose-dev` (or `npm run dev`).
  - **Verifying by hand:** restart `bin/linkomat-server serve` after every
    `make build-server-expose-dev`, then hard-reload the browser (the SW
    is `autoUpdate`, but a hard reload avoids chasing a stale bundle).
    Filter the Network tab to `entries|healthz` to watch sync traffic —
    it is all plain `fetch` on the main thread.

- **Notes:** (filled in after implementation)

### Open questions
- `persistentFailure` is currently set on the **first** push failure by
  `syncOnce()`'s catch block. That is a 9.2 placeholder; 9.3 must drive
  it from the backoff timer (`now - firstFailureAt > 5 min`) instead.
- Direct browser navigation (address bar / reload) to `/entries/:id`
  returns 401 because the Go mux matches `/entries/{id}` before the SPA
  fallback. SPA pushState navigation is unaffected. A proper fix would
  route on `Accept: text/html`. Deferred, carried over from 9.1.
- The `/settings` route (§7.5: username, logout, version) does not exist
  yet; HomeView carries a temporary logout button. Not owned by step 9.