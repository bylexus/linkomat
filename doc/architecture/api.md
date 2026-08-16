# API server (`api/`)

Go 1.22+ HTTP server. Owns persistence and auth; embeds the web PWA
so one binary serves both the API and the static SPA.

## Entry points

- `api/main.go` — `cobra` root. Subcommands: `serve` (default),
  `user` (CRUD), `version`.
- `api/cmd_serve.go` — `serve`: opens meta DB, opens per-user DB
  cache, builds the `Server`, blocks on `ListenAndServe`.
- `api/cmd_user.go` — `user create | set-password | activate |
  deactivate | delete | list | purge`. Prompts for passwords via
  `golang.org/x/term` so they don't leak to a process table.
- `api/cmd_version.go` — `version`: prints `shared.Version`.

## Type map

| File                         | Responsibility                                                                 |
|------------------------------|--------------------------------------------------------------------------------|
| `api/server.go`              | `Server` struct, route table, middleware chain (request ID, security headers, slog request log). |
| `api/config.go`              | `Config` loaded from `LINKOMAT_*` env vars.                                    |
| `api/models.go`              | GORM models: `User`, `Token`, `SchemaVersion`.                                 |
| `api/meta.go`                | `MetaDB` wrapper around `data/meta.db`.                                        |
| `api/userdb.go`              | `UserDBs` (lazy-open mutex-guarded cache) + per-user `UserDB` + `Entry` model. |
| `api/auth.go`                | argon2id hash/verify, opaque token mint, `MetaDB.IssueTokenPair` / `FindActiveToken` / `RevokeToken` / `RevokeAllUserTokens`. |
| `api/cookies.go`             | `setAuthCookies` / `clearAuthCookies` / `readCookie`.                          |
| `api/handlers_auth.go`       | `/auth/login`, `/auth/refresh`, `/auth/logout`.                                |
| `api/handlers_entries.go`    | `GET/PUT/DELETE /entries[/:id]`.                                               |
| `api/handlers_health.go`     | `GET /healthz` → `{status, version, server_time}`.                             |
| `api/handlers_version.go`    | `GET /version` → build info.                                                   |
| `api/middleware_auth.go`     | `authenticate` middleware: 401/403/500 short-circuits for `/entries*`.         |
| `api/ratelimit.go`           | In-memory token bucket on `/auth/login` (5/min per IP+username).               |
| `api/web_assets.go`          | `//go:embed all:web_dist` + custom `ServeContent` handler with SPA fallback.   |
| `shared/*.go`                | DTOs (`Entry`, `AuthResponse`, `ErrorResponse`, `EntriesListResponse`), cookie names, schema version. |

## Request lifecycle

1. `Server.middleware` (the only middleware wrapping the mux) runs
   for every request:
   - Mints a request ID, stores it in `ctx`.
   - Sets `X-Content-Type-Options`, `Referrer-Policy`,
     `X-Request-Id`, `Strict-Transport-Security`, the
     `Content-Security-Policy` from `PLANNING.md` §5.4.
   - Calls `authenticate(rw, r)`. If the path matches `/entries*`
     (`needsAuth`) the middleware looks up the access token from the
     `linkomat_auth` cookie or `Authorization: Bearer …` header and
     either attaches `user_id` / `username` to the context or writes
     401/403/500 directly and returns `nil`. Non-`/entries` paths
     pass through untouched.
   - If `authenticate` returned a request, hands it to the mux.
   - After the mux returns, emits a `slog` JSON line with `method`,
     `path`, `status`, `user_id`, `duration_ms`, `request_id`.
2. The mux dispatches via `Server.routes()` registrations. Each
   `/api/*` route is wrapped by `apiHeaders` which sets
   `Cache-Control: no-store, private`. The `/` (catch-all) route
   serves embedded web assets directly — no security headers, no
   anti-cache.
3. Handlers render responses with `writeJSON` (200/2xx) or
   `writeError` (RFC 7807-shaped: `{type, title, status, detail}`).

## Persistence

- **Meta DB** (`data/meta.db`) — users, tokens, schema version. DSN
  flags: `_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on`.
  Migrations via GORM `AutoMigrate` gated by a `schema_version` row.
  Quiet GORM logger (`slogGormWriter` in `meta.go`).
- **Per-user DBs** (`data/users/<username>.db`) — entries, soft-delete
  tombstones, per-user schema version. Opened on first request via
  `UserDBs.For(username)`, cached for the server's lifetime
  (`UserDBs` has no eviction in v1). `migrate()` writes
  `EntrySchemaVersion` if missing or older than
  `shared.CurrentSchemaVersion`. `tags` is stored as a JSON string in
  a single `TEXT` column (not a normalised table); `entryToDTO` /
  `decodeTags` handle the round-trip.

## Auth

- `User.password_hash` — argon2id (64MB / 1 iter / 2 parallelism).
- Tokens — `tokens` rows, 32-byte hex opaque value, type
  `access` (15 min) or `refresh` (30 d), `revoked_at` for rotation.
  `MetaDB.IssueTokenPair` inserts both rows. `RevokeAllUserTokens`
  is called on `set-password`, `deactivate`, and `delete`.
- Login rate limit — in-memory `loginLimiter`, 5 attempts/min per
  `(IP, username)`, 429 on excess, periodic GC goroutine started by
  `runRateLimitCleanup` in `cmd_serve.go`.

## Cookies

| Cookie             | Path | Value         | Flags                                    |
|--------------------|------|---------------|------------------------------------------|
| `linkomat_auth`    | `/`  | access token  | `HttpOnly`, `SameSite=Strict`, `Secure`  |
| `linkomat_refresh` | `/`  | refresh token | `HttpOnly`, `SameSite=Strict`, `Secure`  |

`Secure` follows `LINKOMAT_COOKIE_SECURE` (default `true`). Set by
`setAuthCookies`, cleared by `clearAuthCookies`.

## Web asset embedding

`api/web_assets.go` uses `//go:embed all:web_dist` to bundle the
built PWA into the server binary. The build pipeline (`Makefile`'s
`web-embed` target) copies `web/dist/*` into `api/web_dist/`
before `go build`. The custom `handleWebAssets`:

- Serves files via `http.ServeContent` so the `index.html` → canonical
  redirect that `http.FileServer` would issue is bypassed.
- Falls back to `index.html` for unknown paths (SPA history mode).
- Trims trailing `/` before `fs.Stat` (`embed.FS` rejects paths
  with trailing slashes).
- `init()` adds `mime.AddExtensionType(".webmanifest", …)` so the
  PWA manifest is served with the right content type.

## Sync endpoints (the contract the web client implements)

- `GET /entries?limit=200&offset=0` — paginated full list, ordered
  `updated_at ASC, id ASC` (stable for cursor sync). `limit` capped
  at 1000; `limit=0`/negative `offset` → 400.
- `GET /entries?changed_since=<RFC3339>` — delta pull. Cursor is
  **exclusive** on the server. Response is
  `{entries: [...], server_time}` — `server_time` is the next cursor.
- `GET /entries/:id` — single entry. **Returns soft-deleted rows
  with `deleted_at` populated** so the delta sync can replicate
  tombstones.
- `PUT /entries/:id` — full body overwrite. Server stamps
  `updated_at = now()`. Last-write-wins by client intent
  (always accepted). PUT against a soft-deleted entry un-deletes it.
  Body `id` may be empty or match the URL.
- `DELETE /entries/:id` — soft-delete. 204 on success.

## Where to look when…

- "Why is this request 401?" — `api/middleware_auth.go`,
  `api/handlers_auth.go`. `authenticate` writes the error directly
  and returns `nil`, so the mux never runs.
- "Where does this header come from?" — `Server.middleware` (security
  headers) and `Server.apiHeaders` (`Cache-Control` for `/api/*`).
- "How do I add a new CLI/web-facing endpoint?" — add the route in
  `Server.routes`, the handler in a `handlers_*.go` file, the DTO in
  `shared/`, and a Bruno request in `../bruno-api-tests/`.
- "How is the per-user DB file laid out?" — `api/userdb.go` (model)
  and `web/src/db/schema.ts` (the local mirror, with sync columns).
