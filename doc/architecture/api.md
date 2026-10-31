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
| `api/userdb.go`              | `UserDBs` (lazy-open mutex-guarded cache) + per-user `UserDB` + `Entry` and `PendingEntry` models. |
| `api/auth.go`                | argon2id hash/verify, opaque token mint, `MetaDB.IssueTokenPair` / `FindActiveToken` / `RevokeToken` / `RevokeAllUserTokens`. |
| `api/cookies.go`             | `setAuthCookies` / `clearAuthCookies` / `readCookie`.                          |
| `api/handlers_auth.go`       | `/auth/login`, `/auth/refresh`, `/auth/logout`.                                |
| `api/handlers_entries.go`    | `GET/PUT/DELETE /entries[/:id]`; `entryToDTO()`.                               |
| `api/handlers_share.go`      | `POST /entries/:id/share`, `GET /pending-entries`, `POST /pending-entries/:id/accept`, `DELETE /pending-entries/:id`; `shareError` / `writeShareError`, meta helpers `readMeta()` / `encodeMeta()` / `metaToRaw()`. |
| `api/handlers_friendships.go`| `GET/POST /friendships`, `POST /friendships/:id/accept`, `…/decline`, `DELETE /friendships/:id`; `newUUIDv4()`, `friendshipStatus()`. |
| `api/handlers_health.go`     | `GET /healthz` → `{status, version, server_time}`.                             |
| `api/handlers_version.go`    | `GET /version` → build info.                                                   |
| `api/middleware_auth.go`     | `authenticate` middleware: 401/403/500 short-circuits for `/entries*`, `/friendships*` and `/pending-entries*`. |
| `api/ratelimit.go`           | In-memory token bucket on `/auth/login` (5/min per IP+username).               |
| `api/web_assets.go`          | `//go:embed all:web_dist` + custom `ServeContent` handler with SPA fallback.   |
| `shared/*.go`                | DTOs (`Entry` incl. `meta`, `AuthResponse`, `ErrorResponse`, `EntriesListResponse`; `shared/friendship.go`: `Friendship`, `FriendshipsListResponse`, `FriendshipCreateRequest`, status constants; `shared/pending_entry.go`: `PendingEntry`, `PendingEntriesListResponse`, `ShareEntryRequest`), cookie names, schema version. |

## Request lifecycle

1. `Server.middleware` (the only middleware wrapping the mux) runs
   for every request:
   - Mints a request ID, stores it in `ctx`.
    - Sets `X-Content-Type-Options`, `Referrer-Policy`,
      `X-Request-Id`, `Strict-Transport-Security`, and the
      `Content-Security-Policy`.
   - Calls `authenticate(rw, r)`. If the path matches `/entries*`,
     `/friendships*` or `/pending-entries*` (`needsAuth`) the middleware looks up the access token from the
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
  (`shared.CurrentSchemaVersion`, currently 3).
  Quiet GORM logger (`slogGormWriter` in `meta.go`).
  - `friends` table (model `Friendship`): `id` (server-generated
    UUIDv4), `requested_by_id`, `accepted_by_id`, `requested_at`,
    `accepted_at` (nullable), `declined_at` (nullable). Both user
    columns are FKs to `users.id` with `ON DELETE CASCADE`, so
    `user delete` removes the rows. Unique index `idx_friends_pair`
    on `(requested_by_id, accepted_by_id)`; no symmetric index (the
    handler checks both directions).
- **Per-user DBs** (`data/users/<username>.db`) — entries, soft-delete
  tombstones, pending (received) entries, per-user schema version.
  Opened on first request via `UserDBs.For(username)`, cached for the
  server's lifetime (`UserDBs` has no eviction in v1). `migrate()` runs
  `AutoMigrate(&Entry{}, &PendingEntry{}, &EntrySchemaVersion{})` and
  writes `EntrySchemaVersion` if missing or older than
  `shared.CurrentSchemaVersion`. `tags` is stored as a JSON string in
  a single `TEXT` column (not a normalised table); `entryToDTO` /
  `decodeTags` handle the round-trip.
  - `entries.meta` — `TEXT NOT NULL DEFAULT '{}'`, a JSON object
    owned by the server (written only by the share and accept
    handlers). `PUT /entries/:id` ignores `meta` in the body and keeps
    the stored value; new rows from `PUT` get `{}`. `entryToDTO` emits
    it via `metaToRaw` (empty / invalid stored JSON → `{}`; re-encoded,
    so keys come out sorted).
  - `pending_entries` table (model `PendingEntry`): `id` (server-generated
    UUIDv4, PK), `from_username`, `source_entry_id`, `title`, `url`,
    `note`, `tags` (JSON text, default `[]`), `shared_at` (indexed). A
    snapshot of the sender's server row at share time. The sender is
    stored by username, not user ID (no FK from a per-user DB to the
    meta DB).

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
- `PUT /entries/:id` — full body overwrite of `title`, `url`, `note`,
  `tags`. Server stamps `updated_at = now()`. Last-write-wins by client
  intent (always accepted). PUT against a soft-deleted entry un-deletes
  it. Body `id` may be empty or match the URL. `meta` in the body is
  ignored; the stored `meta` is kept.
- `DELETE /entries/:id` — soft-delete. 204 on success.

Every entry DTO (`shared.Entry`) carries `meta` — always a JSON
object, `{}` when empty — so `meta` changes reach the client through
the normal pull.

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

## Share / pending-entries endpoints

A *sender* shares one of their entries with a *receiver* (an accepted
friend). The share is a row in the receiver's per-user DB
(`pending_entries`); no sync, the web client reads it online-only.
Fire and forget: the sender sees no accept / decline status.

- `POST /entries/:id/share` — body `{"username": "…"}` (trimmed, exact
  match). `200` with the sender's updated `shared.Entry` (incl. `meta`).
  Checks in this order:
  - body not parseable → `400`; empty username → `400` "Username is
    required."; own username → `400` "You cannot share with
    yourself.";
  - entry not found or soft-deleted → `404` "Entry not found.";
  - target unknown or inactive → `404` "User not found.";
  - no accepted friendship (`accepted_at` set, `declined_at` null,
    either direction) → `403` "Not friends.".

  Writes, in this order (two DB files, **no cross-DB transaction**):
  1. Insert a `PendingEntry` into the receiver's DB (new UUIDv4,
     `from_username` = caller, `source_entry_id`, copy of `title` /
     `url` / `note` / `tags` from the sender's server row,
     `shared_at = now`).
  2. Append `{"to": "<receiver>", "at": "<now>"}` to `meta.shares` of
     the sender's entry and set its `updated_at = now`, so the delta
     pull delivers the new `meta` to the sender's other devices (the
     entry moves to the top of the list).

  If step 2 fails the handler answers `500` and the pending row
  stays. Duplicates are allowed: each share creates a new pending
  row.
- `GET /pending-entries` — `{pending_entries: [...]}`, the caller's
  rows, ordered `shared_at DESC, id ASC`, no pagination. DTO
  `shared.PendingEntry`: `id`, `from`, `source_entry_id`, `title`,
  `url`, `note`, `tags` (array), `shared_at`.
- `POST /pending-entries/:id/accept` — one transaction in the caller's
  DB: create a new entry (new server-generated UUIDv4, `created_at =
  updated_at = now`, content from the pending row, `meta.imported`
  set) and delete the pending row. `200` with the new `shared.Entry`;
  not found → `404` "Pending entry not found.".
- `DELETE /pending-entries/:id` — decline: delete the pending row.
  `204`; not found → `404` "Pending entry not found.".

Pending rows are looked up only in the caller's own DB, so another
user's pending ID gives `404`. The friendship is checked only at
share time; a pending row stays (and can still be accepted or
declined) when the friendship is removed later.

`meta` JSON shape (times RFC3339 UTC):

```json
{ "shares": [ { "to": "bob", "at": "2026-10-31T10:00:00Z" } ] }
{ "imported": { "from": "alice", "shared_at": "2026-10-31T10:00:00Z", "at": "2026-10-31T12:00:00Z" } }
```

`shares` is a list (one item per share). `imported` is one object
(`shared_at` from the pending row, `at` = accept time). An imported
entry that is shared again has both keys. The handlers read the
stored JSON into a `map[string]any` (`readMeta`), change only their
own key and keep all other keys.

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
- "Who writes `entries.meta`?" — only `api/handlers_share.go`
  (`handleEntryShare`, `handlePendingEntryAccept`). `handleEntryPut`
  keeps it.
