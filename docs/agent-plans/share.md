# Share entries with friends

**Ticket/Source:** none (feature description by the developer in the planning session, 2026-10-31)
**Branch:** `feature/share`
**Status:** done

## Goal

A user can send ("share") one of their entries to a friend. The friend sees the
shared entry in a "Received" section on the home view and accepts or declines it.
Accept copies the entry into the friend's own entries (per-user DB). Decline drops it.

Sharing is "fire and forget" for the sender: the entry is put into the receiver's
`pending_entries` table and that is all. The sender's entry keeps a log ("sent to bob
at …") in a new JSON column `meta`. The receiver's imported entry keeps "imported from
alice at …" in the same column. The detail view shows this log at the bottom.

Only users with an **accepted** friendship (feature `docs/agent-plans/friends.md`) can
share entries with each other.

## Scope

**In scope:**
- Server: `meta` column on `entries` (per-user DB), new per-user table `pending_entries`,
  `shared.CurrentSchemaVersion` 2 → 3, share endpoint, pending-entries endpoints, DTOs.
- Web: local schema v3 → v4 (`meta` column), share icon in list rows and the detail
  header, share dialog (friend selection), meta log footer in the detail view,
  "Received" section with Accept / Decline on the home view.
- Architecture docs: `doc/architecture/api.md`, `doc/architecture/web.md`, `AGENTS.md` table.

**Out of scope:**
- Bruno requests in `doc/bruno-api-tests/` (developer decision).
- Admin CLI commands for pending entries (developer decision).
- Automated tests (AGENTS.md: no automated tests in v1).
- Sharing to several friends in one dialog action (single select only).
- Duplicate protection (the same entry can be pending several times at the same receiver).
- Cleanup of pending rows when a friendship is removed.
- Sharing offline or sharing an unsynced entry.
- Local (offline) storage of pending entries in the PWA's sql.js DB.
- An in-place migration of the web local DB (the existing "sidecar and recreate" mechanism is used).
- Notifications / badges for received entries.

## Rough Architecture / Concept

- **Server owns `meta`.** `entries.meta` is a TEXT column holding a JSON object
  (default `'{}'`). Only the server writes it (share handler, accept handler).
  `PUT /entries/{id}` ignores any `meta` in the body and keeps the stored value. The
  client reads `meta` through the normal sync pull (it is part of the `shared.Entry` DTO).
  The server reads the JSON into a `map[string]any`, changes only its own key
  (`shares` or `imported`), and keeps all other keys.
- **Share writes `updated_at = now` on the sender's entry**, so the delta pull
  (`updated_at > cursor`) gets the new `meta` to the sender's other devices. Side effect
  (accepted): the shared entry moves to the top of the home list after the next reload.
- **Pending entries are per-user-DB rows** in the receiver's DB
  (`data/users/<receiver>.db`, table `pending_entries`). They are a snapshot of the
  sender's server row at share time (title, url, note, tags).
- **Two DB files, no cross-DB transaction.** The share handler first inserts the pending
  row into the receiver's DB, then updates the sender's entry. If the second write fails,
  the handler answers 500 and the pending row stays (accepted: share is fire-and-forget).
- **Accept** runs one transaction in the receiver's DB: create a new entry (new
  server-generated UUIDv4, `created_at = updated_at = now`, `meta.imported` set), delete
  the pending row. Returns the new entry. **Decline** = delete the pending row.
- **Web: share only when online and synced.** The share icon is disabled when the
  client is offline or the entry has `local_dirty_at`. Because the entry is synced, the
  server row equals the local row, so the client sends only the target username; the
  server copies its own row.
- **Web: pending entries are online-only**, same pattern as friendships: HomeView
  fetches them on mount and whenever `SyncState.lastSyncAt` changes. Not in the local DB,
  no sync engine change.
- **Web: responses go straight into the local DB.** After share, the returned sender
  entry (new `meta`, new `updated_at`) is written with `LocalDb.applyServerEntry`. After
  accept, the returned new entry is written the same way, so it shows at once without
  waiting for the next pull.

Why: smallest change that follows existing patterns (GORM model + `AutoMigrate` in the
per-user DB, handler file per resource, online-only server data like friendships, native
`<dialog>` like `FriendsView.vue`, inline-SVG icon components). No new dependencies.
One writer for `meta` (the server) avoids merge problems with the last-write-wins sync.

## Context for a new agent

- **Sources read:** `AGENTS.md`, `README.md`, `doc/architecture/overview.md`,
  `doc/architecture/api.md`, `doc/architecture/web.md`, `doc/architecture/build.md`,
  `docs/agent-plans/friends.md`. Key facts:
  - Go server (`api/`, `net/http` mux with method patterns like `"POST /entries/{id}"`,
    GORM + sqlite). Meta DB (`data/meta.db`) = users, tokens, friends, schema version.
    Per-user DBs (`data/users/<username>.db`) = entries + `entry_schema_versions`.
  - `UserDBs.For(username)` (`api/userdb.go`) opens / caches a per-user DB. It can open
    **another** user's DB (needed to write the receiver's pending row).
  - `UserDB.migrate()` runs `AutoMigrate(&Entry{}, &EntrySchemaVersion{})` and stores
    `shared.CurrentSchemaVersion` (`shared/schema.go`, currently 2). The meta DB uses
    the same constant.
  - `authenticate` (`api/middleware_auth.go`) checks only paths matched by `needsAuth`
    (prefixes `/entries`, `/friendships`). It writes 401/403/500 itself and returns
    `nil`. A new prefix (`/pending-entries`) **must** be added there.
  - Context helpers in `api/server.go`: `usernameFromCtx`, `userIDValueFromCtx`.
  - `writeJSON` / `writeError(w, status, type, title, detail)` (RFC 7807-like).
  - All API routes are wrapped with `s.apiHeaders` in `Server.routes()`.
  - `newUUIDv4()` is in `api/handlers_friendships.go`.
  - `Friendship` model (`api/models.go`, table `friends`): `RequestedByID`,
    `AcceptedByID`, `AcceptedAt`, `DeclinedAt`. Accepted = `accepted_at != NULL AND
    declined_at IS NULL`. Either direction counts (A→B or B→A row).
  - `PUT /entries/{id}` (`handleEntryPut`, `api/handlers_entries.go`) overwrites
    title/url/note/tags, sets `updated_at = now`, un-deletes. `entryToDTO` builds the DTO;
    `encodeTags` / `decodeTags` handle the tags JSON.
  - Web: Vue 3 + TS + Vite PWA, local DB sql.js on OPFS. `web/src/db/schema.ts`
    `CURRENT_SCHEMA_VERSION` (currently 3) is a **web-local** constant; on mismatch
    `LocalDb.open()` moves the old file aside (sidecar) and creates a fresh DB, then the
    sync engine does a full pull. Unpushed local edits stay only in the sidecar file
    (accepted risk).
  - `LocalDb.writeServerRow` / `applyServerEntries` / `applyServerEntry`
    (`web/src/db/sqlite.ts`) write server rows, clear `local_dirty_at`, persist and
    dispatch `DB_CHANGE_EVENT`.
  - HomeView's `DB_CHANGE_EVENT` handler (`onChange`) only refreshes already-loaded rows
    by id. New rows need `reloadKeepingPages()`.
  - `ApiResult<T>` (`web/src/api/client.ts`) = `ok | http-error | network-error`.
    `errorTitle(body)` is in `web/src/api/friendships.ts`.
  - No code comments unless asked. No summary at the end of a task.
- **Key files and directories:**
  - `shared/schema.go` — `CurrentSchemaVersion` 2 → 3.
  - `shared/entry.go` — `Entry` DTO gets `Meta`.
  - `shared/pending_entry.go` — new DTOs (`PendingEntry`, `PendingEntriesListResponse`,
    `ShareEntryRequest`).
  - `api/userdb.go` — `Entry.Meta`, new `PendingEntry` model, add to `AutoMigrate`.
  - `api/handlers_entries.go` — `entryToDTO` adds `meta`; `handleEntryPut` keeps `meta`.
  - `api/handlers_share.go` — new: share, list / accept / decline pending entries.
  - `api/server.go` — routes. `api/middleware_auth.go` — `needsAuth` adds `/pending-entries`.
  - `web/src/db/schema.ts` — `meta` column, version 3 → 4.
  - `web/src/db/sqlite.ts` — `Entry.meta`, `ServerEntry.meta`, select lists, `writeServerRow`.
  - `web/src/api/entries.ts` — `ServerEntry.meta`, `shareEntry()`.
  - `web/src/api/pendingEntries.ts` — new: list / accept / decline.
  - `web/src/components/icons/ShareIcon.vue` — new inline-SVG share icon.
  - `web/src/components/ShareDialog.vue` — new reusable share dialog.
  - `web/src/views/HomeView.vue` — share icon in rows, dialog, "Received" section.
  - `web/src/views/EditView.vue` — share icon in header, dialog, meta footer.
- **Existing patterns to follow:**
  - Per-user model + AutoMigrate: `Entry` / `EntrySchemaVersion` in `api/userdb.go`.
  - Handlers + error type: `api/handlers_friendships.go` (`friendshipError`,
    `writeFriendshipError`, transactions with `udb.Transaction`).
  - API module: `web/src/api/friendships.ts`. Icon: `web/src/components/icons/FriendsIcon.vue`
    (`width`/`height` props). Native dialog + `.btn*` styles: `web/src/views/FriendsView.vue`.
    Online-only panel with busy id + inline error: friend request panel in `HomeView.vue`
    (`runFriendAction`, `friendBusyId`, `friendError`, `loadFriendships` with sequence counter).
- **Commands:**
  - Build server (incl. web embed): `make build-server` → `bin/linkomat-server`
  - Run: `LINKOMAT_COOKIE_SECURE=false ./bin/linkomat-server serve` (`:8080`, data in `./data`)
  - Dev (Vite :5173 + Go :8080): `make dev`
  - Go build / vet: `cd api && go build ./... && go vet ./...`
  - Web type check / build: `cd web && source ~/.nvm/nvm.sh && nvm use && npm run build`
  - Users: `./bin/linkomat-server user create alice` (prompts password)
  - Login with curl (cookie jar):
    `curl -s -c /tmp/alice.jar -H 'Content-Type: application/json' -d '{"username":"alice","password":"…"}' http://localhost:8080/auth/login`
- **Environment / prerequisites:** Node 24 via nvm (`web/.nvmrc`). Test users `alice`,
  `bob`, `carol`. alice and bob are accepted friends; carol is not a friend of alice.
  Two browser profiles (or one normal + one private window) for the web tests.
- **Domain terms:**
  - *sender* = user who shares the entry. *receiver* = target friend.
  - *pending entry* = row in the receiver's `pending_entries` table, waiting for accept / decline.
  - *meta* = JSON column on `entries`, server-owned log (`shares`, `imported`).
  - *Received section* = pending entries block on the home view.
- **External references:** none.

## Decision log

- **Owner of `meta`:** the server. Client reads only; `PUT` ignores `meta`. —
  *Rationale:* one writer, no merge problem with last-write-wins sync.
  *Rejected:* client-owned meta (stale client can overwrite the log; server must write
  the import info anyway).
- **`updated_at` on share:** server sets `updated_at = now` on the sender's entry. —
  *Rationale:* the delta pull must deliver the new `meta` to all devices; no sync
  contract change. Entry moving to the top is accepted.
  *Rejected:* separate `changed_at` sync column (changes the sync contract on both
  sides); no bump (other devices never see the log).
- **Share preconditions (web):** share only when online **and** the entry is synced
  (`local_dirty_at` is null). The share icon is disabled otherwise, tooltip
  "Offline" / "Sync first". The dialog also checks the entry is not dirty at [Send]
  time and shows "Entry is not synced yet." if it is (edge case: blur-save in EditView
  just before the icon click). A server 404 is shown as an error in the dialog.
  *Rejected:* client sends content in the share body (developer first chose it, then
  replaced it with "only synced / online"); sync-before-send.
- **Share request body:** only `{"username": "<target>"}`. The server copies its own
  row (equal to the local row because the entry is synced).
- **Web local DB migration:** bump `CURRENT_SCHEMA_VERSION` 3 → 4 (existing sidecar +
  recreate + full pull). Risk of losing unpushed edits at upgrade time is accepted.
  *Rejected:* in-place `ALTER TABLE` migration (new code path).
- **Server schema version:** `shared.CurrentSchemaVersion` 2 → 3 (per-user DB gets a
  column and a table).
- **Pending entries in web:** online-only, like friendships. Loaded on HomeView mount
  and on each `SyncState.lastSyncAt` change. *Rejected:* syncing them into the local DB.
- **Duplicates:** allowed. Every share creates a new pending row, even if the same entry
  is already pending at the same receiver. No 409.
- **Friendship removed while pending:** pending row stays; accept and decline still
  work. The friendship is checked only at share time.
- **`meta` JSON shape:**
  ```json
  { "shares": [ { "to": "bob", "at": "2026-10-31T10:00:00Z" } ] }
  { "imported": { "from": "alice", "shared_at": "2026-10-31T10:00:00Z", "at": "2026-10-31T12:00:00Z" } }
  ```
  `shares` is a list (append on each share). `imported` is one object. An imported
  entry that is shared again has both keys. Times RFC3339 UTC. Server keeps unknown keys.
  Empty / invalid stored JSON is treated as `{}`.
- **Imported entry:** new server-generated UUIDv4 id, `created_at = updated_at = accept
  time`, `deleted_at = NULL`, content from the pending row.
- **`pending_entries` table (model `PendingEntry`, per-user DB):** `id` (UUIDv4, PK),
  `from_username` (text, not null), `source_entry_id` (text, not null), `title`, `url`,
  `note` (text, not null, default `''`), `tags` (JSON text, default `[]`), `shared_at`
  (not null, index). Sender is stored by username (not user id: the per-user DB has no
  FK to the meta DB).
- **API:**
  - `POST /entries/{id}/share` body `{"username":"bob"}` → `200` with the sender's
    updated `shared.Entry` (incl. `meta`). Errors: empty / own username → `400`; entry not
    found or soft-deleted → `404` "Entry not found."; target unknown or inactive → `404`
    "User not found."; no accepted friendship (either direction) → `403` "Not friends.".
    Order of checks: body, username, entry, target user, friendship.
  - `GET /pending-entries` → `{"pending_entries":[PendingEntry…]}`, order `shared_at DESC`.
  - `POST /pending-entries/{id}/accept` → `200` with the new `shared.Entry`; not found → `404`.
  - `DELETE /pending-entries/{id}` → `204` (decline); not found → `404`.
  - `needsAuth` gets prefix `/pending-entries`.
  - *Rejected:* base path `/shares`.
- **`PendingEntry` DTO:** `id`, `from`, `source_entry_id`, `title`, `url`, `note`,
  `tags` (array), `shared_at`.
- **`shared.Entry` DTO:** new field `meta` (`json.RawMessage`, always a JSON object, `{}`
  when empty).
- **Share icon placement:** list row: small icon button at the bottom-right of the row
  (status dot stays top-right), `@click.stop`. Detail view: icon button in the header,
  left of [Delete], only for existing entries (`loaded` set).
- **Share dialog:** reusable `web/src/components/ShareDialog.vue`, native `<dialog>`.
  Title `Share "<entry title or url or (untitled)>"`. On open: `GET /friendships`, list
  accepted friends (the other username of each accepted row), sorted A–Z. Radio list,
  single select. [Send] disabled until a friend is selected and while sending. [Cancel]
  closes. Success → `applyServerEntry(response)` + close. Error → dialog stays open with
  the error text (`errorTitle(body)`, network error → "Not available offline.").
  No friends → "No friends yet." + link to `/friends`.
  *Rejected:* multi select; click-name-to-send.
- **Detail meta footer (EditView):** below the form, only if `meta` has `shares` or
  `imported`. One line per share: "Sent to bob on <date>"; one line for import:
  "Imported from alice on <date>". Date via `new Date(iso).toLocaleString()`.
- **Received section (HomeView):** below the friend request panel, above the entry list.
  Heading "Received (n)". Rows look like entry rows (title, url as link, tags) plus a line
  "from alice · <date>" and buttons [Decline] [Accept]. Rows are not clickable. Not
  filtered by the search. Hidden when empty. Decline has no confirm (same as friend
  decline). Accept → `applyServerEntry(response)`, `reloadKeepingPages()`, reload pending
  list. Inline error per row (same as friend request panel). Load errors are silent.
  *Rejected:* compact one-liners.
- **Testing:** no automated tests; manual checks per step.
- **Docs:** update `doc/architecture/api.md`, `doc/architecture/web.md`, `AGENTS.md`
  "Where to make changes" table. Bruno and admin CLI are out of scope.

## Testing strategy

No automated tests, manual review only. Each step lists manual checks (curl for the
server, browser with two users in two browser profiles for the web).

## Steps

### Step 1 — Server: schema, DTOs, share and pending-entries API
- **Status:** done (checked 2026-10-31)
- **What:**
  - `shared/schema.go`: `CurrentSchemaVersion` = 3.
  - `shared/entry.go`: `Meta json.RawMessage \`json:"meta"\``.
  - `shared/pending_entry.go`: `PendingEntry`, `PendingEntriesListResponse`
    (`pending_entries`), `ShareEntryRequest` (`username`).
  - `api/userdb.go`: `Entry.Meta string \`gorm:"not null;default:'{}';type:text"\``;
    new model `PendingEntry` (table `pending_entries`); add to `AutoMigrate`.
  - `api/handlers_entries.go`: `entryToDTO` sets `Meta` (`{}` if empty / invalid);
    `handleEntryPut` does not change `Meta` (new rows get `{}`).
  - `api/handlers_share.go` (new): `handleEntryShare`, `handlePendingEntriesList`,
    `handlePendingEntryAccept`, `handlePendingEntryDecline`, meta helpers
    (read map, append share, set imported).
  - `api/server.go`: 4 routes. `api/middleware_auth.go`: `needsAuth` adds `/pending-entries`.
- **Acceptance criteria:**
  - `cd api && go build ./... && go vet ./...` passes.
  - Server starts with existing data; per-user DBs get `meta` column and
    `pending_entries` table (`sqlite3 data/users/alice.db .schema`).
  - `GET /entries` returns `"meta":{}` for old entries.
  - alice → bob share: `200`, response `meta.shares` has bob; bob's
    `GET /pending-entries` lists it with `from: "alice"`.
  - alice → carol (not friend): `403`. Unknown user: `404`. Own username: `400`.
    Unknown / deleted entry id: `404`.
  - Share the same entry twice → two pending rows at bob.
  - bob accept → `200` new entry with `meta.imported.from = "alice"`; pending row gone;
    bob's `GET /entries` contains it. bob decline (`DELETE`) → `204`, row gone.
    Accept / decline unknown id → `404`.
  - `PUT /entries/{id}` with `"meta":{"x":1}` in the body does not change the stored meta.
  - Without cookie: `GET /pending-entries` → `401`.
- **Notes:** Implemented as planned. Developer asked to run all steps without a review
  gate (2026-10-31); checked by the agent against the acceptance criteria with curl on a
  scratch data dir / port 18080 — all pass. Extra error titles: "Username is required.",
  "You cannot share with yourself.", "Pending entry not found.". List order
  `shared_at desc, id asc`. Meta helpers `readMeta` / `encodeMeta` / `metaToRaw` in
  `api/handlers_share.go`; `metaToRaw` re-encodes, so keys come out sorted. Error type
  `shareError` + `writeShareError`. Accept / decline of another user's pending id → 404
  (rows are looked up only in the caller's DB). Known pre-existing quirk:
  `UserDB.migrate()` `Save`s a new `entry_schema_versions` row (table now has rows 2 and 3);
  harmless, not changed.

### Step 2 — Web: data layer
- **Status:** done (checked 2026-10-31)
- **What:**
  - `web/src/db/schema.ts`: `meta TEXT NOT NULL DEFAULT '{}'` on `entries`;
    `CURRENT_SCHEMA_VERSION` = 4.
  - `web/src/db/sqlite.ts`: `Entry.meta` and `ServerEntry.meta` (parsed object type
    `EntryMeta = { shares?: {to: string; at: string}[]; imported?: {from: string; shared_at: string; at: string} }`,
    stored as JSON text); all `SELECT` lists include `meta`; `writeServerRow` writes
    `meta`; `upsertEntry` / `softDeleteEntry` do not touch `meta`.
  - `web/src/api/entries.ts`: `ServerEntry.meta`; `shareEntry(id, username)` →
    `POST /entries/{id}/share`.
  - `web/src/api/pendingEntries.ts` (new): `PendingEntry` type, `listPendingEntries`,
    `acceptPendingEntry`, `declinePendingEntry`.
- **Acceptance criteria:**
  - `npm run build` passes (type check).
  - On first load after the update, the local DB is recreated (sidecar) and fully pulled.
  - DevTools (`make build-server-expose-dev` or `make dev`):
    `__linkomatDb` … `getEntry(id).meta` shows the server meta of an entry shared in Step 1.
  - Editing an entry still syncs; its `meta` is unchanged after the push / pull.
- **Notes:** Implemented as planned. `EntryMeta` is defined once in `web/src/db/sqlite.ts`;
  `web/src/api/entries.ts` imports it as a type. `ServerEntry` stays defined in both
  `api/entries.ts` and `db/sqlite.ts` (pre-existing duplication), both got
  `meta?: EntryMeta | null`. `rowToEntry` maps invalid JSON / null / arrays to `{}`.
  Checked in Chrome against a scratch server on 127.0.0.1:18080 (`make build-server-expose-dev`):
  build passes, v3 → v4 sidecar + full pull works, `getEntry(id).meta` shows the server
  `shares`, an edit syncs and keeps `meta`. The new API functions are only exercised by
  the views in Steps 3/4.

### Step 3 — Web: share icon, share dialog, detail meta footer
- **Status:** done (checked 2026-10-31)
- **What:**
  - `web/src/components/icons/ShareIcon.vue` (inline SVG, `width`/`height` props).
  - `web/src/components/ShareDialog.vue` (see decision "Share dialog"); exposes `open(entry)`.
  - `HomeView.vue`: share icon button bottom-right in each row (`@click.stop`), disabled
    when offline or dirty (tooltip), opens the dialog.
  - `EditView.vue`: share icon button in the header left of [Delete] (existing entries
    only), same disabled rule; meta footer below the form.
- **Acceptance criteria:**
  - As alice (online, synced entry): click share in a row → dialog lists bob (not carol)
    → select bob → [Send] → dialog closes. bob's `GET /pending-entries` shows it.
  - Row click on the icon does not open the entry.
  - Offline (DevTools "Offline") or right after an edit (dirty dot) → icon disabled.
  - Detail of the shared entry shows "Sent to bob on <date>". Second device / after
    reload shows it too.
  - User without friends → dialog shows "No friends yet." with link to `/friends`.
- **Notes:** Implemented as planned; all criteria checked in Chrome against a scratch
  server on 127.0.0.1:18080. Details: `ShareDialog.vue` exposes `open(entry)`, uses an
  `openSeq` counter to ignore late responses, de-duplicates friend names, shows "Loading…"
  while friends load, "Could not load friends." on an HTTP error without title, and the
  no-friends link text is "Add friends". HomeView row `padding-right` 1.75rem → 2.25rem so
  tags do not run under the icon (row height stays 96px). EditView header changed from
  flex to a `1fr auto 1fr` grid so the title stays centered with the share + Delete group.
  Disabled reason labels: "Offline" (checked first), "Sync first".

### Step 4 — Web: "Received" section on the home view
- **Status:** done (checked 2026-10-31)
- **What:** `HomeView.vue`: load pending entries (mount + `lastSyncAt`, sequence
  counter like `loadFriendships`), "Received (n)" section between friend request panel
  and the list, [Decline] / [Accept] with busy id + inline error; on accept
  `applyServerEntry` + `reloadKeepingPages()` + reload pending list.
- **Acceptance criteria:**
  - As bob: section shows alice's shared entry with title, url, tags, "from alice · <date>".
  - [Accept] → row leaves the section, entry appears in the list at once; its detail
    shows "Imported from alice on <date>".
  - [Decline] → row leaves the section; no entry is created.
  - Section hidden when no pending entries; not filtered by the search field.
  - Offline: [Accept] shows "Not available offline." under the row.
- **Notes:** Implemented as planned in `HomeView.vue` only; all criteria checked in
  Chrome against a scratch server on 127.0.0.1:18080. State: `pendingEntries`,
  `receivedBusyId`, `receivedError`, `pendingEntriesLoadSeq`; helpers
  `loadPendingEntries`, `runReceivedAction` (parallel to `runFriendAction`, because accept
  needs `applyServerEntry` + `reloadKeepingPages()`), `setReceivedError`. Items use the
  class `.received-item` (no fixed height) and reuse `.row-title` / `.row-url` /
  `.row-tags` / `.tag-chip`; tags wrap inside received items (`flex-wrap: wrap`) so they
  stay readable on narrow screens.

### Step 5 — Architecture docs
- **Status:** done (checked 2026-10-31)
- **What:** update `doc/architecture/api.md` (per-user DB: `meta`, `pending_entries`;
  share / pending-entries endpoints; `needsAuth` prefixes; type map), `doc/architecture/web.md`
  (layout: new files; local DB `meta`; share + Received section; schema v4),
  `AGENTS.md` (table row "Sharing", `needsAuth` prefix list).
- **Acceptance criteria:** docs match the code; a fresh agent can find all share code
  from `AGENTS.md`.
- **Notes:** Updated `doc/architecture/api.md` (type map, `needsAuth`, persistence:
  schema v3, `entries.meta`, `pending_entries`; new section "Share / pending-entries
  endpoints"), `doc/architecture/web.md` (layout tree, local DB `meta` / v4, new section
  "Sharing"; also corrected the old claim that `applyServerEntries` does not touch
  `local_dirty_at` — `writeServerRow` clears it), `AGENTS.md` (Web PWA bullet, `needsAuth`
  prefixes, table row "Sharing (API / web)"). `overview.md` unchanged. All named paths /
  functions / endpoints were grepped against the code. The pre-existing doc error in
  `overview.md` (`agent-plans/` under `doc/` instead of `docs/`) was fixed on `main` (`fa3f33c`).

## Open questions

None. The two pre-existing bugs found during testing (direct load of `/entries/:id`
showing an empty "New entry" form; first search change after returning from the detail
view not filtering) were fixed on branch `fix/edit-reload-and-search` (commit `fa3f33c`,
merged into `main`). `main` was then merged into `feature/share` (fast-forward of the
branch base, share work re-applied on top, no conflicts).

## Current state / Next action

Plan approved 2026-10-31. The developer asked to run all steps one after another
without review gates, each checked against its acceptance criteria.
All steps 1–5 done (2026-10-31). `main` (with the bug fixes, `fa3f33c`) merged into
`feature/share` on 2026-10-31; `go build`, `go vet` and `vue-tsc -b` pass after the
merge. Next action: developer approves the implementation, then commit on
`feature/share`; then bump the version to 0.3.0, commit and tag.
