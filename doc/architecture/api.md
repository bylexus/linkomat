# API server (`api/`)

Go 1.22+ HTTP server, `package api` (module
`github.com/bylexus/linkomat/api`). Owns persistence and auth; embeds
the web PWA so one binary serves both the API and the static SPA.

## Entry points

- `cmd/linkomat-server/main.go` — the binary's `main` (module
  `github.com/bylexus/linkomat/cmd`). Calls `api.NewRootCmd().Execute()`
  and exits 1 on error. Builds to `linkomat-server`.
- `api/root.go` — `NewRootCmd()`: `cobra` root. Subcommands: `serve`
  (default), `user` (CRUD), `version`. The only exported entry point
  of the `api` package.
- `api/cmd_serve.go` — `serve`: opens meta DB, opens per-user DB
  cache, builds the `Server`, blocks on `ListenAndServe`.
- `api/cmd_user.go` — `user create | set-password | activate |
  deactivate | delete | list | purge | friends | remove-friendship`.
  Prompts for passwords via `golang.org/x/term` so they don't leak
  to a process table. `friends <username>` lists all friendship rows
  of a user (any state); `remove-friendship <id> | <user1> <user2>`
  deletes one row by ID or the row between two users (either
  direction).
- `api/cmd_version.go` — `version`: prints `shared.Version`.

## Type map

| File                         | Responsibility                                                                 |
|------------------------------|--------------------------------------------------------------------------------|
| `api/server.go`              | `Server` struct, route table, middleware chain (request ID, security headers, slog request log). |
| `api/config.go`              | `Config` loaded from `LINKOMAT_*` env vars.                                    |
| `api/models.go`              | GORM models: `User`, `Token`, `Friendship` (table `friends`), `SchemaVersion`. |
| `api/meta.go`                | `MetaDB` wrapper around `data/meta.db`.                                        |
| `api/userdb.go`              | `UserDBs` (lazy-open mutex-guarded cache) + per-user `UserDB` + `Entry` model. |
| `api/auth.go`                | argon2id hash/verify, opaque token mint, `MetaDB.IssueTokenPair` / `FindActiveToken` / `RevokeToken` / `RevokeAllUserTokens`. |
| `api/cookies.go`             | `setAuthCookies` / `clearAuthCookies` / `readCookie`.                          |
| `api/handlers_auth.go`       | `/auth/login`, `/auth/refresh`, `/auth/logout`.                                |
| `api/handlers_entries.go`    | `GET/PUT/DELETE /entries[/:id]`.                                               |
| `api/handlers_friendships.go`| `GET/POST /friendships`, `POST /friendships/:id/accept`, `…/decline`, `DELETE /friendships/:id`; `newUUIDv4()`, `friendshipStatus()`. |
| `api/handlers_health.go`     | `GET /healthz` → `{status, version, server_time}`.                             |
| `api/handlers_version.go`    | `GET /version` → build info.                                                   |
| `api/middleware_auth.go`     | `authenticate` middleware: 401/403/500 short-circuits for `/entries*` and `/friendships*`. |
| `api/ratelimit.go`           | In-memory token bucket on `/auth/login` (5/min per IP+username).               |
| `api/web_assets.go`          | `//go:embed all:web_dist` + custom `ServeContent` handler with SPA fallback.   |
| `shared/*.go`                | DTOs (`Entry`, `AuthResponse`, `ErrorResponse`, `EntriesListResponse`; `shared/friendship.go`: `Friendship`, `FriendshipsListResponse`, `FriendshipCreateRequest`, status constants), cookie names, schema version. |

## Request lifecycle

1. `Server.middleware` (the only middleware wrapping the mux) runs
   for every request:
   - Mints a request ID, stores it in `ctx`.
    - Sets `X-Content-Type-Options`, `Referrer-Policy`,
      `X-Request-Id`, `Strict-Transport-Security`, and the
      `Content-Security-Policy`.
   - Calls `authenticate(rw, r)`. If the path matches `/entries*`
     or `/friendships*` (`needsAuth`) the middleware looks up the access token from the
     `linkomat_auth` cookie or `Authorization: Bearer …` header and
     either attaches `user_id` / `username` to the context or writes
     401/403/500 directly and returns `nil`. Other paths pass
     through untouched.
   - If `authenticate` returned a request, hands it to the mux.
   - After the mux returns, emits a `slog` JSON line with `method`,
     `path`, `status`, `user_id`, `duration_ms`, `request_id`.
2. The mux dispatches via `Server.routes()` registrations. Each
   API route (every route except `/`) is wrapped by `apiHeaders` which sets
   `Cache-Control: no-store, private`. The `/` (catch-all) route
   serves embedded web assets. It gets the security headers from
   `Server.middleware` (which wraps the whole mux), but no
   `Cache-Control: no-store` — workbox precache needs HTTP caching.
3. Handlers render responses with `writeJSON` (200/2xx) or
   `writeError` (RFC 7807-shaped: `{type, title, status, detail}`).

## Persistence

- **Meta DB** (`data/meta.db`) — users, tokens, friendships, schema
  version. DSN flags: `_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on`.
  Migrations via GORM `AutoMigrate` gated by a `schema_version` row
  (`shared.CurrentSchemaVersion`, currently 2).
  Quiet GORM logger (`slogGormWriter` in `meta.go`).
  - `friends` table (model `Friendship`): `id` (server-generated
    UUIDv4), `requested_by_id`, `accepted_by_id`, `requested_at`,
    `accepted_at` (nullable), `declined_at` (nullable). Both user
    columns are FKs to `users.id` with `ON DELETE CASCADE`, so
    `user delete` removes the rows. Unique index `idx_friends_pair`
    on `(requested_by_id, accepted_by_id)`; no symmetric index (the
    handler checks both directions).
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

## Friendship endpoints

Server-only data in the meta DB; no per-user DB, no sync. The
*requester* is `requested_by`, the *addressee* is `accepted_by`.
Status is derived from the timestamps (`declined_at` set →
`declined`, `accepted_at` set → `accepted`, else `pending`); there is
no status column. The DTO carries usernames only, never user IDs.

- `GET /friendships` — `{friendships: [...]}`, all rows where the
  caller is requester or addressee, ordered `requested_at DESC`, no
  pagination. Declined rows are hidden for the addressee.
- `POST /friendships` — body `{"username": "…"}` (trimmed, exact
  match). `201` with the new pending row. Empty / own username →
  `400`; unknown or inactive user → `404`; caller→target row exists
  (any state) → `409`; reverse row is accepted → `409`. Reverse row
  pending or declined → auto-accept that row (`accepted_at = now`,
  `declined_at` cleared), `200`.
- `POST /friendships/:id/accept` and `…/decline` — addressee only,
  pending rows only. `200` with the row; not involved / not found →
  `404`; caller is the requester → `403`; not pending → `409`.
- `DELETE /friendships/:id` — requester or addressee, any state
  (remove friend, withdraw request, acknowledge a decline). `204`;
  not involved / not found → `404`.

The API path is `/friendships`, not `/friends`, because `/friends`
is a SPA route. A shared path (as with `/entries/:id`) only works
while the service worker's `navigateFallback` serves `index.html`;
a first visit or hard reload without the SW would get JSON.

## Where to look when…

- "Why is this request 401?" — `api/middleware_auth.go`,
  `api/handlers_auth.go`. `authenticate` writes the error directly
  and returns `nil`, so the mux never runs.
- "Where does this header come from?" — `Server.middleware` (security
  headers) and `Server.apiHeaders` (`Cache-Control` for all API routes).
- "How do I add a new web-facing endpoint?" — add the route in
  `Server.routes`, the handler in a `handlers_*.go` file, the DTO in
  `shared/`, and a Bruno request in `../bruno-api-tests/`.
- "How is the per-user DB file laid out?" — `api/userdb.go` (model)
  and `web/src/db/schema.ts` (the local mirror, with sync columns).
