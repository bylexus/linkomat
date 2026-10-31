# Friends (friendship requests between users)

**Ticket/Source:** none (feature description by the developer in the planning session, 2026-10-31)
**Branch:** `feature/friends`
**Status:** done

## Goal

A user can invite another user as a "friend". The invited user must accept
before the friendship exists. Future features need this friend relationship
(e.g. sharing). This feature only builds the relationship itself: data model,
API, admin CLI, and web UI to request / accept / decline / remove friendships.

Flow:
1. User A sends a request to user B (by exact username).
2. B sees the request on the home view (and on `/friends`) and accepts or declines.
3. Accepted → A and B are friends. Either one can remove the friendship (row is deleted).
4. Declined → A sees "B declined your request" on the home view with an "OK"
   button. "OK" deletes the row.

## Scope

**In scope:**
- Meta DB table `friends` + GORM model, schema version bump.
- REST endpoints under `/friendships` (server).
- Admin CLI: `linkomat-server user friends …` and `linkomat-server user remove-friendship …`.
- Web PWA: `/friends` view, people icon button on the home view with badge,
  friend request panel on the home view.
- Architecture docs (`doc/architecture/*.md`, `AGENTS.md` table).

**Out of scope:**
- Bruno requests in `doc/bruno-api-tests/` (explicitly excluded by the developer).
- End-user Go CLI (`cli/`, still a placeholder).
- Any feature that *uses* friendships (sharing etc.).
- User search / autocomplete. The username is typed as plain text.
- Spam protection / cooldown after a decline.
- Symmetric uniqueness index (A→B and B→A rows at the same time from a race). YAGNI.
- Local (offline) storage of friendships in the PWA's sql.js DB.
- Automated tests.

## Rough Architecture / Concept

- **Server-only data.** Friendships live in the meta DB (`data/meta.db`), table
  `friends`, model `Friendship` in `api/models.go`. Users are referenced by
  `users.id` with FKs `ON DELETE CASCADE`, so `user delete` removes the rows
  without extra code (DSN already has `_foreign_keys=on`).
- **REST API** in a new `api/handlers_friendships.go`, routes in
  `api/server.go`, auth via the existing `authenticate` middleware (prefix
  `/friendships` added to `needsAuth`). DTO in new `shared/friendship.go`.
  The API returns usernames, never user IDs.
- **Status is derived** from the timestamps: `declined_at != null` → `declined`;
  `accepted_at != null` → `accepted`; else `pending`. No status column.
- **Web is online-only for this feature.** The PWA calls `GET /friendships`
  directly through the `apiGet` / `apiPost` / `apiDelete` wrappers in `web/src/api/client.ts`. No sync engine
  changes. HomeView loads on mount and whenever `syncState.lastSyncAt`
  changes (the sync engine runs every 1 min + on online/focus/local edits).
- **API path is `/friendships`, not `/friends`**, because the SPA route is
  `/friends`. The existing `/entries/:id` SPA route and `GET /entries/{id}`
  API share a path and only work because the service worker's
  `navigateFallback` serves `index.html`; a first visit / hard reload without
  the SW would get JSON. Different paths avoid this for the new feature.

Why: smallest change that matches existing patterns (GORM model + AutoMigrate
like `Token`, handler file per resource like `handlers_entries.go`, thin API
module like `web/src/api/entries.ts`, inline-SVG icon components like
`LogoutIcon.vue`). No new dependencies.

## Context for a new agent

- **Sources read:** `AGENTS.md`, `README.md`, `doc/architecture/overview.md`,
  `doc/architecture/api.md`, `doc/architecture/web.md`, `doc/architecture/build.md`.
  Key facts:
  - Go server (`api/`, Go 1.26, `net/http` mux with method patterns, GORM +
    `gorm.io/driver/sqlite` / mattn). Meta DB = users, tokens, schema version.
    Per-user DBs `data/users/<username>.db` hold entries.
  - `authenticate` (`api/middleware_auth.go`) only checks paths matched by
    `needsAuth` (currently prefix `/entries`). It writes 401/403/500 itself
    and returns `nil`. Context values: `usernameFromCtx`, `ctxKeyUserID`
    (uint) in `api/server.go`.
  - All API routes are wrapped by `s.apiHeaders` (`Cache-Control: no-store, private`).
  - `writeJSON` / `writeError(w, status, type, title, detail)` live in
    `api/handlers_auth.go` (RFC 7807-like `shared.ErrorResponse`).
  - Web: Vue 3 + TS + Vite PWA. `ApiResult<T>` = `ok | http-error | network-error`;
    callers check `kind`. 401 → one refresh → retry → logout handler.
  - No code comments unless asked. No summary at the end of a task.
- **Key files and directories:**
  - `api/models.go` — add `Friendship` model.
  - `api/meta.go` — `MetaDB.migrate()`: add `&Friendship{}` to `AutoMigrate`.
  - `shared/schema.go` — `CurrentSchemaVersion` 1 → 2.
  - `shared/friendship.go` — new DTOs.
  - `api/handlers_friendships.go` — new handlers.
  - `api/server.go` — routes.
  - `api/middleware_auth.go` — `needsAuth` gets `/friendships`.
  - `api/cmd_user.go` — new admin subcommands.
  - `web/src/api/friendships.ts` — new API module.
  - `web/src/views/FriendsView.vue` — new view.
  - `web/src/router.ts` — route `/friends`.
  - `web/src/components/icons/FriendsIcon.vue` — new inline-SVG people icon.
  - `web/src/views/HomeView.vue` — icon button, badge, request panel.
- **Existing patterns to follow:**
  - Model: `Token` in `api/models.go`. Handler: `api/handlers_entries.go`.
  - Admin command: `newUserDeleteCmd` / `newUserListCmd` (tabwriter) in `api/cmd_user.go`.
  - API module: `web/src/api/entries.ts`. Icon: `web/src/components/icons/LogoutIcon.vue`
    (accepts `width`/`height` props). Button style: `.logout-btn`, FAB `.fab` in `HomeView.vue`.
- **Commands:**
  - Build server (incl. web embed): `make build-server` → `bin/linkomat-server`
  - Run: `LINKOMAT_COOKIE_SECURE=false ./bin/linkomat-server serve` (listens on `:8080`, data in `./data`)
  - Dev (Vite :5173 + `go run ./api` :8080): `make dev`
  - Go build only: `cd api && go build ./...` ; vet: `cd api && go vet ./...`
  - Web type check / build: `cd web && nvm use && npm run build`
  - Users: `./bin/linkomat-server user create alice` (prompts password)
- **Environment / prerequisites:** Node 24 via nvm (`web/.nvmrc`). At least
  three test users (e.g. `alice`, `bob`, `carol`) for manual tests.
- **Domain terms:**
  - *requester* = `requested_by` user (sends the request).
  - *addressee* = `accepted_by` user (accepts or declines).
  - *home panel* = friend request panel on the home view.
- **External references:** none.

## Decision log

- **User reference in table:** user IDs with FK to `users.id`, `ON DELETE CASCADE`.
  Columns `requested_by_id`, `accepted_by_id`. — *Rationale:* DB enforces cleanup on
  `user delete`; matches `Token.UserID`. *Rejected:* usernames (spec draft) — needs manual cleanup, no integrity.
- **Table / columns:** table `friends`: `id` (UUID string, PK, generated on the server),
  `requested_by_id`, `accepted_by_id`, `requested_at` (not null), `accepted_at` (nullable),
  `declined_at` (nullable). Unique index over `(requested_by_id, accepted_by_id)`.
- **UUID generation:** small UUIDv4 helper with `crypto/rand` in `api/`, no new dependency.
- **Symmetric uniqueness index:** not added (YAGNI). The application checks both
  directions before insert; a race between parallel A→B and B→A is accepted.
- **Schema version:** bump `shared.CurrentSchemaVersion` 1 → 2. — *Rationale:* developer:
  it is a schema migration even if the per-user tables do not change.
  (`web/src/db/schema.ts` `CURRENT_SCHEMA_VERSION` is a different, web-local constant — do NOT bump it.)
- **API base path:** `/friendships` (SPA route stays `/friends`). — *Rationale:* avoid
  SPA/API path collision (see Concept).
- **Endpoints:**
  - `GET /friendships` → `{"friendships": [Friendship…]}`. All rows where the user is
    requester or addressee, **except** rows where the user is the addressee and the row is
    declined (the decliner does not see them again). No pagination. Order: `requested_at DESC`.
  - `POST /friendships` body `{"username": "bob"}` → creates a pending row, `201`.
  - `POST /friendships/{id}/accept` and `POST /friendships/{id}/decline` → `200` with the row.
  - `DELETE /friendships/{id}` → `204`.
- **DTO (`shared.Friendship`):** `id`, `requested_by` (username), `accepted_by` (username),
  `status` (`pending|accepted|declined`), `requested_at`, `accepted_at` (nullable),
  `declined_at` (nullable). All times RFC3339 UTC. — *Rationale:* developer rejected a
  `direction` field (incoming/outgoing) as unclear; the client compares usernames with its own.
- **Create rules (`POST /friendships`):** username is trimmed and matched exactly
  (case-sensitive, same as login in `api/handlers_auth.go`).
  - empty username or own username → `400`.
  - user not found or inactive → `404` "user not found" (username enumeration accepted).
  - row A→B exists (any state) → `409`.
  - reverse row B→A exists:
    - pending → auto-accept: set `accepted_at = now` on that row, return it with `200`.
    - declined (A declined B earlier, B has not clicked OK yet) → A changed their mind:
      clear `declined_at`, set `accepted_at = now`, return `200`.
    - accepted → `409` "already friends".
- **Accept / decline rules:** only the addressee; only on pending rows.
  Not involved / not found → `404`; caller is the requester → `403`; not pending → `409`.
- **Delete rule:** requester or addressee may delete in any state. Not involved / not found → `404`.
  Used for remove friendship, withdraw pending request, and "OK" on a declined request.
- **Re-request after decline:** allowed once the requester clicked "OK" (row gone). No cooldown.
- **Home reload trigger:** HomeView calls `GET /friendships` on mount and when
  `syncState.lastSyncAt` changes. *Rejected:* fetching inside `syncOnce()` (couples friend errors to sync backoff).
- **Offline / errors:** home panel keeps last loaded data, else shows nothing (no error).
  `/friends` view shows "Friends are not available offline." (network error) or the error
  text. Action buttons show an inline error if their call fails.
- **Home friends button:** icon-only button with a new inline-SVG people icon, in the meta row
  left of the logout button, routes to `/friends`. Badge with count = number of home panel items
  (incoming pending + own declined); hidden when 0.
- **Home panel:** below the sticky header, above the list (scrolls away). One compact card
  per item: "bob wants to be your friend" [Accept][Decline]; "carol declined your request" [OK].
  Hidden when empty.
- **`/friends` view:** back link to `/`, title "Friends". Rows:
  accepted → [Remove] (native `window.confirm()` first);
  own pending → "requested by you" [Withdraw];
  incoming pending → "wants to be friends" [Accept][Decline];
  own declined → not shown here (only on home with OK).
  FAB "+" (same style as home FAB) opens a native `<dialog>`: username field, [Send], [Cancel].
  Success → close + reload list. Error (400/404/409) → dialog stays open with the error text.
- **Admin CLI:**
  - `linkomat-server user friends <username>` — tabwriter list: ID, REQUESTED_BY, ACCEPTED_BY, STATUS, REQUESTED_AT, ACCEPTED_AT, DECLINED_AT.
  - `linkomat-server user remove-friendship <id>` or `… remove-friendship <user1> <user2>` — deletes by UUID, or the row between both users in any direction.
- **Bruno requests:** out of scope (developer decision).
- **Shared button CSS:** `.btn*` rules are duplicated in `HomeView.vue` and `FriendsView.vue`.
  Kept as copy for now (developer decision, Step 4). *Rejected for now:* a shared stylesheet.
- **Testing:** no automated tests (AGENTS.md v1 rule); manual checks per step.

## Testing strategy

No automated tests, manual review only. Each step lists manual checks (curl,
admin CLI, browser with two users in two browser profiles / private window).

## Steps

### Step 1 — Backend API
- **Status:** done (reviewed 2026-10-31)
- **What:** `Friendship` model (table `friends`, `TableName()`), FK constraints with
  `ON DELETE CASCADE`, unique index `(requested_by_id, accepted_by_id)`; add to
  `AutoMigrate`; bump `shared.CurrentSchemaVersion` to 2; UUIDv4 helper; DTOs in
  `shared/friendship.go` (`Friendship`, `FriendshipsListResponse`, `FriendshipCreateRequest`);
  `api/handlers_friendships.go` with the 5 endpoints and all rules in the decision log;
  routes in `api/server.go` (wrapped by `apiH`); `/friendships` prefix in `needsAuth`.
- **Acceptance criteria:** with users alice, bob, carol: all endpoint rules and status
  codes from the decision log behave as described; carol gets 404 on alice/bob rows;
  unauthenticated → 401; `user delete bob` removes bob's rows; meta DB `schema_versions` shows 2.
- **Notes:**
  - New files: `api/handlers_friendships.go` (5 handlers, `newUUIDv4()` via `crypto/rand`,
    `answerFriendship` shared by accept/decline, `friendshipToDTO`, `friendshipStatus`,
    internal `friendshipError` type for transaction error mapping), `shared/friendship.go`
    (`Friendship`, `FriendshipsListResponse`, `FriendshipCreateRequest`, constants
    `FriendshipStatusPending|Accepted|Declined`).
  - Changed: `api/models.go` (`Friendship` with belongs-to `RequestedBy` / `AcceptedBy User`
    fields, `constraint:OnDelete:CASCADE`, unique index `idx_friends_pair`), `api/meta.go`
    (AutoMigrate), `api/server.go` (routes + helper `userIDValueFromCtx(ctx) uint`),
    `api/middleware_auth.go` (`needsAuth` matches `/friendships`), `shared/schema.go` (2).
  - Body limit for `POST /friendships` is 1024 bytes. Create/accept/decline run in
    `s.meta.Transaction`. Inserts use `Omit(clause.Associations)`; responses `Preload` the users.
  - Error type slugs: `invalid-request` (400), `forbidden` (403), `not-found` (404),
    `conflict` (409), `internal` (500).
  - Generated schema verified: both FKs `ON DELETE CASCADE`, `UNIQUE INDEX idx_friends_pair
    (requested_by_id, accepted_by_id)`. No extra index on `accepted_by_id` (not needed for the
    small user base).
  - Smoke-tested on a fresh temp DB with alice/bob/carol/dave: all rules of the decision log
    pass, `user delete` cascades, `Cache-Control: no-store, private` present. The developer
    reviewed manually with the existing `data/meta.db`.
  - A token of a user who is deactivated with `user deactivate` gets 401 (the command revokes all tokens). This is existing behaviour.

### Step 2 — Admin CLI
- **Status:** done (reviewed 2026-10-31)
- **What:** `user friends <username>` and `user remove-friendship <id> | <user1> <user2>` in `api/cmd_user.go`.
- **Acceptance criteria:** list shows rows of step 1 data; remove by UUID and by two
  usernames (both orders) works; unknown user / no row → clear error, exit code ≠ 0.
- **Notes:**
  - Only `api/cmd_user.go` changed: `newUserFriendsCmd` (`cobra.ExactArgs(1)`),
    `newUserRemoveFriendshipCmd` (`cobra.RangeArgs(1,2)`), helpers `findUserByName(meta, username)`
    and `formatOptionalTime(*time.Time)` (RFC3339 or `-`). Both registered in `newUserCmd`.
  - Status column reuses `friendshipStatus()` from `api/handlers_friendships.go`. Rows are
    loaded with `Preload("RequestedBy").Preload("AcceptedBy")`, ordered `requested_at desc`.
  - `remove-friendship` prints `removed friendship <id> (<requested_by> -> <accepted_by>)`.
    Errors: `user %q does not exist`, `friendship %q not found`,
    `no friendship between %q and %q`; exit code 1.
  - Known: GORM prints a `WARN … record not found` line on unknown users (same as `user create`).
    If the out-of-scope race ever creates two rows for one pair, `remove-friendship <u1> <u2>`
    deletes one row per call.

### Step 3 — Web `/friends` view
- **Status:** done (reviewed 2026-10-31)
- **What:** `web/src/api/friendships.ts`, `FriendsView.vue`, route `/friends`,
  `FriendsIcon.vue`, icon button in the HomeView meta row (no badge yet).
- **Acceptance criteria:** list with all row types and actions; "+" dialog incl. error
  display; Remove asks `confirm()`; offline shows the message.
- **Notes:**
  - New: `web/src/api/friendships.ts` (types `FriendshipStatus`, `Friendship`,
    `FriendshipsListResponse`; `listFriendships`, `createFriendship(username)`,
    `acceptFriendship(id)`, `declineFriendship(id)`, `deleteFriendship(id)`; helper
    `errorTitle(body)` → `ErrorResponse.title` or null), `web/src/components/icons/FriendsIcon.vue`
    (same props/defaults as `LogoutIcon.vue`), `web/src/views/FriendsView.vue`.
  - Changed: `web/src/router.ts` (route `/friends`), `web/src/views/HomeView.vue`
    (`.home-meta-actions` wrapper holding `[Friends][Logout]`; `.friends-btn` shares the
    `.logout-btn` CSS; `data-test="home-friends"`; `router.push('/friends')`). HomeView
    preserve logic unchanged.
  - FriendsView: back `RouterLink` "←" to `/`; "Loading…" while loading; "No friends yet."
    when empty. Rows as in the decision log; own declined rows hidden. Action errors shown
    under the failing row ("Not available offline." for network errors); all action buttons
    disabled while an action runs; list reloads after success. "+" FAB opens a native
    `<dialog>` (`showModal()`); input + error reset on open; Send disabled while busy or empty.
  - `make dev` (Vite :5173) has no API proxy, so `/friendships` (and `/entries`) do not work
    there. Test with `make build-server && LINKOMAT_COOKIE_SECURE=false ./bin/linkomat-server serve`
    on `http://localhost:8080`. `api/web_dist/` is git-ignored.
  - Developer tested in the browser with two users: works.

### Step 4 — Home request panel + badge
- **Status:** done (reviewed 2026-10-31)
- **What:** panel below the header in `HomeView.vue`, badge on the friends icon,
  load on mount + on `syncState.lastSyncAt` change, silent on error.
- **Acceptance criteria:** incoming request appears within ~1 min (or on focus / reload);
  Accept/Decline/OK work and update panel + badge; nothing shown when empty or offline.
- **Notes:**
  - Only `web/src/views/HomeView.vue` changed. `loadFriendships()` runs from a separate
    `onMounted` hook and from a separate watcher on `syncState.value.lastSyncAt` (non-null
    change). Existing `reloadKeepingPages` watcher, `onBeforeRouteLeave` / preserve logic and
    the sync engine are unchanged. Request counter `friendshipsLoadSeq` drops stale responses.
    Load errors are silent (keep last data).
  - Computed `friendRequests` (server order): incoming pending (`accepted_by === me`) and own
    declined (`requested_by === me`). Panel `<section data-test="home-friend-requests">` between
    `</header>` and the DB-state banners; cards `data-test="home-friend-request"`; buttons
    `home-friend-accept` / `home-friend-decline` / `home-friend-ok`; error
    `home-friend-request-error`. `friendBusyId` disables buttons during an action.
  - Badge `.friends-badge` (`data-test="home-friends-badge"`, red pill, top-right of
    `.friends-btn` which is now `position: relative`); aria-label/title "Friends (N open)".
  - `.btn`, `.btn-primary`, `.btn-danger` are copied from `FriendsView.vue` into HomeView's
    scoped styles. Developer decision: keep the copy for now (no shared stylesheet).
  - Developer reviewed and approved.

### Step 5 — Docs
- **Status:** done (reviewed 2026-10-31)
- **What:** update `doc/architecture/api.md` (type map, endpoints, persistence),
  `doc/architecture/web.md` (layout, routes, friends), `doc/architecture/build.md`
  (admin CLI commands), `doc/architecture/overview.md` if needed, `AGENTS.md` "Where to make changes" table.
- **Acceptance criteria:** developer reads the diff.
- **Notes:**
  - `api.md`: cmd_user subcommands, type map (`handlers_friendships.go`, `Friendship`,
    `shared/friendship.go`), `needsAuth` = `/entries*` + `/friendships*`, `friends` table under
    Persistence (schema version 2), new section "Friendship endpoints".
  - `web.md`: layout tree, 5 routes, new section "Friends".
  - `build.md`: admin commands `user friends`, `user remove-friendship`.
  - `overview.md`: diagram "meta DB (users/tokens/friends)", data-flow bullets say "of entries",
    friendships online-only.
  - `AGENTS.md`: web bullet (friendships server-only), `needsAuth` bullet, new table row.
  - On developer request, two pre-existing doc errors were also fixed: sync interval is 1 min
    (`SYNC_INTERVAL_MS`), not 5 min (`web.md`); the non-existent `/api/*` prefix is replaced by
    "all API routes (every route except `/`)" in `AGENTS.md`, `api.md`, `build.md`.

## Open questions

- Pre-existing: `MetaDB.migrate()` uses `Save(&SchemaVersion{Version: N})` with `Version` as
  PK, so a bump inserts a second row instead of updating; `First()` then returns the old row
  and re-saves on every start. Harmless, not part of this feature. Leave it unless the developer asks.
- Pre-existing, found and fixed in Step 5 (developer request): `api.md` said the `/` web-asset
  route gets no security headers (it does, via `Server.middleware`); `build.md` said the web on
  `:5173` calls `:8080` directly (it uses relative paths, Vite has no proxy; use `make dev-server`).

## Current state / Next action

Feature done 2026-10-31. All 5 steps done and reviewed. Committed on branch `feature/friends`.
No temporary changes to revert. Nothing remaining.
