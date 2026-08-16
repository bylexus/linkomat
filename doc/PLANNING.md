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
   - No Background Sync API usage.
   - Light mode only.
   - Embedded web assets in the server binary.
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
- `Content-Security-Policy: default-src 'self'; script-src 'self';
  style-src 'self' 'unsafe-inline'; img-src 'self' data:;
  connect-src 'self'; base-uri 'none'; frame-ancestors 'none'`
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
- Only when both report "online" does the sync worker run.
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

### 7.6 Sync Worker

- One in-flight sync job per session.
- Triggered by:
  - App start.
  - `online` event.
  - Visibility change to visible.
  - Interval (5 min default) when online.
  - Manual "sync now" button.
- Drains the dirty queue serially, FIFO. Each entry shown with a tiny
  status dot: green (synced), yellow (unsynced), spinner (syncing).
  Header chip: `pending: N`.

### 7.7 Search

- Client-side over local sql.js DB.
- FTS5 virtual table mirroring entries if the build supports FTS5;
  otherwise `LIKE`-based search.
- Debounced 200 ms. Match against `title`, `url`, `note`, `tags`.

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

Order of implementation (manual dev loop, no CI, no tests):

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
6. CLI: `login`, `logout`, `add`, `ls`, `search`, `get`, `rm`,
   `edit` (with `fsnotify`).
7. Web: PWA scaffold, sql.js wrapper, OPFS persistence, integrity
   check.
8. Web: home view + virtualized list + search.
9. Web: note editing view + save-on-blur + delete.
10. Web: sync worker (dirty queue, delta pull, full pull on first
    launch, retry/backoff, status UI).
11. End-to-end manual pass on mobile (PWA install) and CLI.

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

**What landed**:

- `go.work` (Go 1.26.1) wires `./api`, `./cli`, `./shared`.
- `Makefile` with targets `build-server`, `build-cli`, `web-build`,
  `dev`, `tidy`, `all`, `clean`. npm-touching targets source
  `$(HOME)/.nvm/nvm.sh` and run `nvm use` first.
- `api/main.go` and `cli/main.go`: minimal stubs that print version and
  `os.Exit(0)`.
- `shared/shared.go`: single `Version` constant.
- `web/` scaffold: Vite 8 + Vue 3 + TS 6 + `vite-plugin-pwa` 1.
  - `package.json` pins: `vue@^3.5.41`, `@vitejs/plugin-vue@^6.0.8`,
    `typescript@~6.0.3`, `vite@^8.2.1`, `vite-plugin-pwa@^1.3.0`,
    `vue-tsc@^3.3.10`, `@types/node@^26.2.0`.
  - `web/.nvmrc` pins node 24 LTS.
  - PWA manifest: `name=Linkomat`, `short_name=Link`,
    `theme_color=#2563eb`, `background_color=#ffffff`, light only.
  - Icons `pwa-192x192.png`, `pwa-512x512.png`,
    `pwa-maskable-512x512.png` generated from `favicon.svg` via `sips`
    (placeholders — replace with real artwork later).
  - `public/sqlite/.gitkeep` reserves the sql.js self-host directory
    (filled in step 7).
- `.gitignore` extended with `bin/`, `node_modules/`, `web/dist/`,
  `data/`, `*.db*`, `web/*.tsbuildinfo`.

**Decisions made in this session**:

- Module path owner: `bylexus` (from the existing `go.mod` files).
- PWA palette: blue `#2563eb` on white `#ffffff`. Change in
  `web/vite.config.ts` when needed.
- TypeScript pinned to 6.0.3 instead of the latest 7.0.2 — TS 7 removed
  the `./lib/tsc` subpath export that `vue-tsc@3.3.10` (latest) needs.
  Revisit when `vue-tsc` ships TS 7 support.

**Smoke checks performed**:

- `make build-server` → `./bin/linkomat-server` runs, prints version.
- `make build-cli` → `./bin/linkomat` runs, prints version.
- `make web-build` → `vue-tsc` + `vite build` succeed; PWA precache 12
  entries.
- `npm run dev` → Vite serves on `:5173`, page shows “Scaffold ok”.

### Session 2 — shared/ DTOs (step 2 of §10)

**Status**: done, committed (`c51fa80`).

**What landed**:

- `shared/entry.go`: `Entry` struct per §3.1 — fields `ID`, `Title`,
  `URL`, `Note`, `Tags []string`, `CreatedAt`, `UpdatedAt`,
  `DeletedAt *time.Time` with matching JSON tags.
- `shared/auth.go`: `AuthResponse` per §5.4 — `AccessToken`,
  `RefreshToken`, `ExpiresIn`, `Username`.
- `shared/error.go`: `ErrorResponse` per §5.4 (RFC 7807-shaped) —
  `Type`, `Title`, `Status`, `Detail`.
- `shared/cookies.go`: `CookieAuth`, `CookieRefresh`, `CookiePathAuth`
  (`/`), `CookiePathRefresh` (`/auth/refresh`) per §5.3.
- `shared/schema.go`: `CurrentSchemaVersion = 1`.
- `api/go.mod` and `cli/go.mod` now declare a `require` + local
  `replace` directive for `./shared` so `make tidy` works on workspace
  modules.
- `api/main.go` and `cli/main.go` now import `shared` and use
  `shared.Version` instead of a duplicated local constant.

**Decisions made in this session**:

- Workspace-local Go modules need an explicit `require` + `replace` in
  the consumer's `go.mod` (`=> ../shared`) so `go mod tidy` does not try
  to resolve them over the network. `go.work` alone covers builds but
  not tidy. Keep this pattern for any future internal module split.

**Smoke checks performed**:

- `make tidy` → clean across all three modules.
- `make build-server` → `./bin/linkomat-server` prints
  `linkomat-server 0.1.0`.
- `make build-cli` → `./bin/linkomat` prints `linkomat 0.1.0`.
- `go vet ./...` clean for each module.

**Next**: step 3 of §10 — server meta DB, user CRUD CLI, password
hashing, `/auth/login`, `/auth/refresh`, `/auth/logout`, login rate
limit.

### Session 3 — server auth + meta DB + user CLI (step 3 of §10)

**Status**: done, committed (`2acc1c7`).

**What landed**:

- `api/config.go`: `Config` loaded from `LINKOMAT_*` env vars. Defaults
  match §5.2 (later amended to drop `LINKOMAT_TLS_*` and the Secure/
  TLS coupling — see Session 4).
- `api/models.go`: `User`, `Token`, `SchemaVersion` GORM models per §3.2
  and §3.3.
- `api/meta.go`: `MetaDB` opens `data/meta.db` with the §5.1 DSN
  flags; `AutoMigrate` runs and the `schema_version` row is upserted
  to `shared.CurrentSchemaVersion`.
- `api/auth.go`: argon2id hash/verify with the §3.2 params
  (`m=64MB, t=1, p=2`). Opaque 32-byte hex tokens; helpers to issue a
  pair, look up an active token, revoke one token, revoke all tokens
  for a user.
- `api/cookies.go`: `setAuthCookies` / `clearAuthCookies` use the §5.3
  cookie names/paths and follow `cfg.CookieSecure`.
- `api/ratelimit.go`: in-memory token bucket per `(remote_ip|username)`
  on `/auth/login`, 5/min per §5.5. Cleanup goroutine started in
  `serve`.
- `api/server.go`: `Server` wires the mux, sets `X-Content-Type-Options`,
  `Referrer-Policy`, `X-Request-Id`, runs slog JSON request log
  (§5.1 — full security headers + HSTS deferred to step 5).
- `api/handlers_auth.go`: `/auth/login`, `/auth/refresh` (rotates),
  `/auth/logout`. JSON body for CLI, cookies for browser. Errors are
  `shared.ErrorResponse` (RFC 7807-shaped).
- `api/cmd_user.go` + `cmd_serve.go` + `cmd_version.go`: cobra
  subcommands — `user create|set-password|activate|deactivate|delete|
  list|purge`, `serve`, `version`. Password prompts use
  `golang.org/x/term` in TTY mode and a shared `bufio.Reader` in
  non-TTY mode.
- `api/main.go`: cobra root with `SilenceUsage`/`SilenceErrors` set;
  errors go to stderr and the process exits 1.
- `api/go.mod`: new direct deps — `gorm.io/gorm`, `gorm.io/driver/sqlite`,
  `github.com/spf13/cobra`, `golang.org/x/crypto`, `golang.org/x/term`.
- `doc/bruno-api-tests/`: Bruno collection covering the auth surface.
  Includes `script:post-response` blocks on login/refresh to capture
  rotated tokens into env vars, and `script:pre-request` blocks that
  delete `linkomat_refresh` from the cookie jar so refresh-body and
  refresh-invalid actually exercise the body path.
- `Makefile`: `api-test`, `api-test-req REQ=<name>`, `api-test-reset`
  targets. Default `BRUNO_*` overrides; help text updated.
- `doc/bruno-api-tests/README.md`: import instructions, run order, var
  reference, plus a "secrets and the auto-format caveat" section.

**Discrepancies from the plan (called out per §0.2)**:

- §3.3 listed no `value` column on `tokens`. Implementation needs a way
  to look up a presented token, so I added `value TEXT` (opaque random
  hex, unique index). The §3.3 table above is updated to match.
- §5.3 and §7.4 originally scoped `linkomat_refresh` to `Path
  /auth/refresh`. RFC 6265 cookies are not sent to paths outside the
  cookie's path, which meant `/auth/logout` could never revoke the
  browser's refresh token. Changed `shared.CookiePathRefresh` and the
  plan §5.3 / §7.4 entries to `Path /`. HttpOnly + Secure +
  SameSite=Strict still bound the exposure.

**Decisions made in this session**:

- Workspace-local Go modules need an explicit `require` + `replace` in
  the consumer's `go.mod` (`=> ../shared`) so `go mod tidy` does not try
  to resolve them over the network. (Carried over from session 2.)
- `api/` uses `github.com/spf13/cobra` for subcommand dispatch (same
  library chosen for the CLI module per §8.1). Keeps the operator
  surface consistent; the alternative was a hand-rolled subcommand
  parser, which would have been more code for no real benefit.
- Bruno collection lives under `doc/` rather than a top-level
  `tests/` so it stays discoverable next to the design doc.
- `vars:secret` only protects *runtime* token vars (`lastAccessToken`,
  `lastRefreshToken`). The `bogusRefreshToken` fixture stays in the
  regular `vars` block because Bruno CLI 4.x wipes values declared in
  `vars:secret` on every run, which would break `refresh-invalid`.

**Smoke checks performed**:

- `make tidy` and `make build-server build-cli` clean.
- `go vet ./...` clean for each module.
- `linkomat-server version` prints `linkomat-server 0.1.0`.
- `linkomat-server user create alice` + `user list` round-trip.
- `linkomat-server serve` accepts `POST /auth/login`, returns
  `shared.AuthResponse` JSON, sets both cookies with correct
  `Path=/` / `HttpOnly` / `SameSite=Strict` (path fix verified).
- `POST /auth/login` with wrong password → 401 `invalid-credentials`.
- 6th login attempt within a minute → 429 `rate-limited`.
- `POST /auth/refresh` with the cookie rotates the token pair
  (new access + refresh); refresh after logout → 401 `invalid-token`.
- `POST /auth/logout` returns 204 and clears both cookies; the cleared
  cookie path now reaches `/auth/logout` so the server actually
  revokes the refresh row.
- `user set-password` flips login to the new password and revokes the
  previous tokens; message shows the revoked count.
- `user deactivate` → login returns 403 `account-deactivated`.
- `user activate` → login returns 200 again.
- `user create alice` again → error `user "alice" already exists`.
- `make api-test` against a running server: 7 of 9 requests pass
  (47/53 assertions). The 2 failures are the documented stateful
  requests — `login-deactivated` (needs `charlie` created and
  deactivated) and `login-rate-limited` (needs 5 prior failed attempts
  in the same minute).
- `make api-test-req REQ=refresh-invalid` passes twice in a row
  (verifies `bogusRefreshToken` persists across runs since it is no
  longer in `vars:secret`).

**Next**: step 4 of §10 — server per-user DB cache, `GET/PUT/DELETE
/entries`, soft-delete, `AutoMigrate` + `schema_version` for per-user
DBs.

### Session 4 — drop server-side TLS termination (small change)

**Status**: done, committed (`01d74d3`).

**What changed**:

- `api/config.go`: dropped `TLSCert` / `TLSKey` fields and the two
  validations that coupled `LINKOMAT_COOKIE_SECURE` to TLS and that
  required `LINKOMAT_TLS_CERT` and `LINKOMAT_TLS_KEY` to be set as a
  pair. Server now starts cleanly with `LINKOMAT_COOKIE_SECURE=true`
  and no TLS env vars.
- `api/server.go`: `ListenAndServe` no longer branches on TLS config —
  it always calls `srv.ListenAndServe()` on plain HTTP.
- `doc/PLANNING.md` §5.2: removed `LINKOMAT_TLS_CERT` /
  `LINKOMAT_TLS_KEY` env vars and the "refuses to start with Secure
  cookies and `LINKOMAT_TLS_*` unset" rule. Added a paragraph noting
  the server runs behind a TLS-terminating proxy and that the
  `Secure` cookie flag stays at its `true` default.

**Decision rationale**:

- The server is always deployed behind a TLS-terminating proxy in v1
  (per the deployment model). Keeping `ListenAndServeTLS` in the
  binary meant an unused code path, env vars nobody sets, and a
  startup check that fights operators who want the `Secure` cookie
  flag without managing TLS certs in the Go process.
- HSTS still ships on every response (`Strict-Transport-Security`
  header, added in step 5) so the cookie path to the browser stays
  HTTPS-only as long as the proxy passes the header through.

**Smoke checks performed**:

- `go build ./api` and `go vet ./api/...` clean.
- Server starts with `LINKOMAT_COOKIE_SECURE=true` and no TLS env
  vars; `POST /auth/login` returns 200 and the `Set-Cookie` headers
  carry `Secure` as expected.

**Next**: step 4 of §10 — server per-user DB cache, `GET/PUT/DELETE
/entries`, soft-delete, `AutoMigrate` + `schema_version` for per-user
DBs.

### Session 5 — per-user DB cache + entries endpoints (step 4 of §10)

**Status**: done, uncommitted.

**What landed**:

- `api/models.go` (existing): no changes — the `Entry` model landed
  in `api/userdb.go` because it's per-user DB only, not meta DB.
- `api/userdb.go` (new): `Entry` GORM model per §3.1
  (`id`/`title`/`url`/`note`/`tags`/`created_at`/`updated_at`/
  `deleted_at`), with `Tags` stored as a JSON-encoded TEXT column
  and converted to `[]string` on the way out via `entryToDTO` /
  `encodeTags` / `decodeTags`. `EntrySchemaVersion` mirrors the
  meta DB's pattern. `UserDB` opens `data/users/<username>.db`
  with the same DSN flags as the meta DB, runs `AutoMigrate` +
  schema-version upsert, and `Close` returns the underlying
  `*sql.DB.Close()` error. `UserDBs` is a `sync.Mutex`-guarded
  `map[string]*UserDB` cache with lazy open (`For`); `Close` walks
  all open DBs and returns the first error.
- `api/middleware_auth.go` (new): `authenticate(rw, r)` returns the
  request with `ctxKeyUserID` + `ctxKeyUsername` set on success or
  nil if it already wrote a 401 / 403. Reads `linkomat_auth` cookie
  first, then `Authorization: Bearer <token>` header. Skips
  `/auth/*` entirely. Validates the token against the meta DB
  (`FindActiveToken`) and confirms the user is `Active`.
- `api/handlers_entries.go` (new): five handlers per §5.4.
  - `GET /entries?limit=&offset=` — paginated list ordered
    `updated_at ASC, id ASC` (stable for cursor sync); defaults
    `limit=200`, cap 1000.
  - `GET /entries?changed_since=<RFC3339>` — exclusive cursor,
    same ordering.
  - `GET /entries/{id}` — single entry, returns 404 if not found.
  - `PUT /entries/{id}` — full overwrite. Body may omit `id` or
    match URL; mismatched non-empty body id → 400. Creates if
    missing, otherwise updates and bumps `updated_at`. Restores
    a soft-deleted entry (sets `deleted_at = NULL`).
  - `DELETE /entries/{id}` — soft-delete with `deleted_at = now()`
    and `updated_at = now()`; returns 204. The row stays in the DB
    so the tombstone can be replicated via delta sync.
  - All responses carry `Cache-Control: no-store, private`.
  - `entryToDTO` normalizes timestamps to UTC for the wire — GORM's
    `Save` reloads the row from the SQLite driver, which parses
    datetimes as local time, so the second PUT on the same row would
    have shown `+02:00` unless we `.UTC()` on the way out.
- `api/server.go`: routes added for the five new endpoints; the
  request middleware now calls `s.authenticate(rw, r)` and only
  dispatches the next handler if `r` is non-nil. The auth result
  is propagated to the request log via the same `r` reference, so
  `user_id` shows up in the slog line for `/entries/*` requests
  and is empty for failed-auth ones. `ctxKeyUsername` added to the
  context keys; `usernameFromCtx` helper added.
- `api/cmd_serve.go`: opens `UserDBs` and `defer`s `Close` between
  the meta DB close and the server start.
- `api/cmd_user.go`: `user delete` now also removes the user DB's
  `-shm` / `-wal` sidecar files (best-effort; the main file is
  the source of truth).
- `shared/entries.go` (new): `EntriesListResponse`
  `{entries, server_time}` for the list endpoint.
- `doc/bruno-api-tests/entries/` (new): 10 `.bru` requests covering
  - 401 paths (no auth, bad token)
  - 200 paths (list, get, put new, put update, changed-since,
    deleted get)
  - 204 path (delete)
  - 404 path (unknown id)
  - The two cookie-path entries test was removed in v1 because
    Bruno CLI's cookie jar is per-folder, so the entries folder
    would not see cookies from `auth/login`. The cookie auth path
    is exercised by `auth/refresh-cookie` (same middleware) and
    will be retested end-to-end when the PWA lands in step 7+.
- `doc/bruno-api-tests/README.md`: added `entries/` section, swapp
  ed `api-test-entries` for the dependency note that `bru run`
  shares `vars:secret` / runtime env vars only within one invocation.
- `Makefile`: `api-test` now runs `bru run auth entries` in one
  invocation so the `auth/login` `script:post-response` populates
  `lastAccessToken` for the entries tests. `api-test-entries`
  removed (would need a separate login run elsewhere — out of scope).
  `api-test-req` accepts `REQ=<folder>/<name>` so a single request
  from either folder can be run. `api-test-reset` writes the new
  `entryId` default.
- `doc/bruno-api-tests/environments/Local.bru`: `entryId` added
  (regular `vars`, default `01923456-7890-7abc-9def-0123456789ab`).
  `lastAccessToken` / `lastRefreshToken` stay in `vars:secret`.

**Decisions made in this session**:

- **Tags as JSON in TEXT, not a separate table.** §3.1 calls for
  `tags TEXT (JSON)`. The simplest encoding is a JSON string in
  a single column, converted to `[]string` on the way in / out.
  A normalized `entry_tags` table would have made the sync cursor
  more complex (per-tag inserts) and provided no query benefit for
  v1 (search is client-side in step 7).
- **List ordering is `updated_at ASC, id ASC`.** The list endpoint
  serves both the home view (client reverses in step 7) and the
  first-launch sync pull. ASC + `id` tiebreak gives a stable
  deterministic order that the cursor can advance through without
  skipping rows on ties.
- **`changed_since` is exclusive, list is inclusive.** The server
  treats `updated_at > cursor` as the delta; the full list uses
  `> -∞` (i.e. everything). Advancing the cursor from the last
  response's `server_time` works in both cases because the
  client never sees a row from before the cursor.
- **PUT un-deletes a soft-deleted row.** The plan calls PUT a
  "full body overwrite" and the server should always accept it
  (last-save-wins by client intent). Setting `deleted_at = NULL`
  on PUT matches that semantic; an entry you PUT to isn't a
  tombstone anymore.
- **PUT body id may match URL or be empty.** The body shape is
  `shared.Entry`, so `id` is present. We require either empty
  or matching URL; a non-empty mismatch is a 400 because the
  client likely has a bug. The URL is the source of truth.
- **DB cursor config.** The DSN flags for the per-user DB
  (`_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on`)
  match the meta DB per §5.1. No `_loc=UTC` — the driver's local
  parsing is normalized to UTC in `entryToDTO` so the wire format
  is always UTC regardless of the server's local timezone.
- **UserDBs cache has no eviction in v1.** Each user gets at
  most one open SQLite connection for the server's lifetime.
  For a single-operator-server this is fine. Eviction (LRU on
  a configurable cap) is a v2 concern.
- **`user delete` cleans up WAL/SHM.** The CLI's `os.Remove` of
  the main file leaves the WAL/SHM sidecars. Best-effort
  cleanup of those too, even though SQLite would tolerate a
  stale shm on next open. The server's `UserDBs` cache is
  *not* invalidated by this; that's a v2 limitation (the cache
  would need a server-side control channel to drop). Restart
  the server after `user delete` in v1.
- **Bruno CLI's cookie jar is per-folder.** Found by running
  `bru run auth` and then `bru run entries/entries-list.bru`
  separately — the auth login populated the auth folder's cookie
  jar but the entries folder had its own empty jar. Fix: run
  `bru run auth entries` in one invocation. The cookie auth path
  on `/entries` is still tested by the existing
  `auth/refresh-cookie` request (same middleware). A future
  step-7+ PWA smoke check will exercise the entries endpoints
  via cookies in a running browser.

**Discrepancies from the plan (called out per §0.2)**:

- §5.4's list endpoint lets the client pass `limit` and `offset`;
  the implementation rejects `limit=0` / negative `offset` with
  400 and caps `limit` at 1000 (the plan picks no upper bound;
  capping is a defensive default to avoid a single client
  pulling 50k rows in one call). The cap is not in the plan.
  Worth flagging for review.
- §5.4 says `GET /entries/:id` returns "entry" with no shape
  modifier. The implementation returns the entry unconditionally
  including soft-deleted rows (with `deleted_at` populated), so
  the delta sync can replicate tombstones. This matches §6.2
  step 5. The "404 vs entry-with-deleted-at" choice is implicit
  in the plan; the chosen behavior is what the client needs.

**Smoke checks performed**:

- `make tidy` and `make build-server build-cli` clean.
- `go vet ./api/...` and `go vet ./cli/...` clean.
- `rm -rf data` + `printf 'hunter2\nhunter2\n' | ./bin/linkomat-server
  user create alice` + `serve` round-trip.
- `POST /auth/login` → 200, captures access token.
- `GET /entries` (no auth) → 401 with `type` containing
  `unauthorized`.
- `GET /entries` with bogus Bearer → 401 with `invalid-token`.
- `PUT /entries/<uuid>` → 200 with full entry, server-stamped
  `created_at == updated_at`.
- `PUT /entries/<uuid>` again → 200, `updated_at` advances,
  `created_at` unchanged, `deleted_at` absent.
- `DELETE /entries/<uuid>` → 204.
- `GET /entries/<uuid>` after delete → 200 with `deleted_at`
  populated.
- `GET /entries` → list with the one entry.
- `GET /entries?changed_since=2000-01-01T00:00:00Z` → 200,
  entries include the soft-deleted row.
- `GET /entries?changed_since=3000-01-01T00:00:00Z` → 200,
  entries empty.
- `GET /entries?changed_since=not-a-time` → 400
  `invalid-request`.
- `GET /entries?limit=0` → 400 `invalid-request`.
- `PUT /entries/<uuid>` with mismatched body id → 400
  `invalid-request`.
- `GET /entries/<unknown-uuid>` → 404 `not-found`.
- All responses carry `Cache-Control: no-store, private` and
  the standard security headers from session 3.
- Request log includes `user_id=1` for authenticated requests
  and `user_id=""` for failed auth.
- `linkomat-server user delete alice` removes `alice.db`,
  `alice.db-shm`, `alice.db-wal` from `data/users/`.
- `make api-test` runs `auth + entries` together: 17 of 19
  requests pass, 96 of 102 assertions pass. The 2 failing
  requests are the documented stateful auth tests
  (`login-deactivated` needs a deactivated user, `login-rate-
  limited` needs 5 prior failed attempts in the same minute).

**Next**: step 5 of §10 — server embedded web assets, `/healthz`,
`/version`, security headers (CSP, HSTS), request logging
refinements.