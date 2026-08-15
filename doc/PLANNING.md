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
   - Go module path: `github.com/<you>/linkomat/{api,cli,shared}`
     — replace `<you>` with the actual GitHub owner on first commit.
   - Default server URL in CLI: `https://app.linkomat.example.com`
     — update to the operator’s domain before any release.
   - Default PWA theme/background colors: pick during web scaffold.
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
- `LINKOMAT_TLS_CERT` / `LINKOMAT_TLS_KEY` (optional).
- `LINKOMAT_COOKIE_SECURE` (default true; cookies are always
  `SameSite=Strict`).
- `LINKOMAT_BEHIND_PROXY` (trusts `X-Forwarded-For` if set).
- `LINKOMAT_CORS_ORIGINS` (empty by default; same-origin served).

Server refuses to start with `Secure` cookies and `LINKOMAT_TLS_*`
unset.

### 5.3 Cookies

| Cookie            | Path             | Value         | Flags                              |
|-------------------|------------------|---------------|------------------------------------|
| `linkomat_auth`   | `/`              | access token  | `HttpOnly`, `SameSite=Strict`, `Secure` |
| `linkomat_refresh`| `/auth/refresh`  | refresh token | `HttpOnly`, `SameSite=Strict`, `Secure` |

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
  (Path `/auth/refresh`) cookies. Client never reads them in JS.
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
- Background sweepers for expired tokens.