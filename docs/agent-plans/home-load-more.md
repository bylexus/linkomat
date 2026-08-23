# Home view: paginate result list (load more)

**Ticket/Source:** none
**Branch:** none (stay on `main`)
**Status:** done

## Goal

The home view's result list is currently unpaged — `HomeView.vue:43` dumps the
entire result set from `db.searchEntries(q)` / `db.listEntries()` into a
`RecycleScroller`. This change limits the initial render to 50 entries and adds
a "Load more" button that fetches 50 more entries per click, using a
cursor-based pagination on the existing `updated_at DESC, id ASC` sort.
Re-searching resets the list to page 1. The loaded pages persist across
navigation away and back to the home view.

## Scope

**In scope:**
- Web PWA only. `web/src/db/sqlite.ts`, `web/src/views/HomeView.vue`,
  `web/src/state/homeSearch.ts` (renamed), `web/src/state/auth.ts`,
  `doc/architecture/web.md`.
- DB layer: cursor-based pagination of `listEntries` and `searchEntries`.
- HomeView UI: Load more button, cursor state, navigation-round-trip state
  preservation, DB-change refresh-by-id.
- Rename `usePreservedHomeSearch` → `usePreservedHomeState` and extend it to
  carry the whole home page state.
- Update the doc/architecture documentation where needed. Do not introduce new documentation chapters. the architecture doc should stay a high-level document, not document every small implementation detail.

**Out of scope:**
- Server / API changes. The server endpoints stay as they are.
- CLI changes.
- Local DB schema changes. No new tables, no `CURRENT_SCHEMA_VERSION` bump.
- Automated tests / CI. Manual smoke checks only (per `AGENTS.md`).
- Total-count display ("X of Y"). Not requested.
- Auto-load on scroll. Button only.
- Sticky / floating Load more button. Plain button below the scroller.

## Rough Architecture / Concept

The sort key for both `listEntries` and `searchEntries` is
`updated_at DESC, id ASC` (`web/src/db/sqlite.ts:199` and `:306`). The
cursor is the `(updated_at, id)` pair of the last row in the current
page. The next page query adds a single WHERE predicate:

```sql
WHERE (updated_at < ?) OR (updated_at = ? AND id > ?)
```

combined with the existing `deleted_at IS NULL` (and `entries_fts MATCH ?`
for search). `LIMIT 51` (page size 50 + 1) lets the method detect
`hasMore` without a separate `COUNT(*)`: if 51 rows came back, drop the
extra and `hasMore = true`; else `hasMore = false`. No total count is
shown in the UI.

The home page state — `query`, `entries`, `cursor`, `hasMore` — moves
into a module-level Vue ref in a renamed `state/homeState.ts`. On
`onBeforeRouteLeave` to an `/entries/*` route, HomeView snapshots the
state into that ref. On `onMounted`, the snapshot is restored directly
(no DB round-trip). On logout, the snapshot is cleared.

DB-change events no longer trigger a full list reload. Instead, for each
currently-loaded entry id, HomeView re-fetches the row from the DB and
splices the fresh fields in place. The list structure (which entries are
shown, in what order) does not change on DB change; the user sees a
fresh list only on next navigation mount or next search.

## Decision log

- **Cursor shape:** `(updated_at, id)` tuple.
  *Rationale:* pure `updated_at` is unsafe — two entries written in the
  same second collide at the cursor boundary and would be skipped or
  duplicated. Matches the existing two-key `ORDER BY`.
- **Initial page size:** 50. **Load more increment:** +50.
  *Rationale:* requested by the developer. Small enough for a mobile PWA
  first paint, large enough that re-search resets are not jarring.
- **Has-more detection:** `LIMIT 51` per fetch; `hasMore = (raw length === 51)`.
  *Rationale:* avoids a separate `COUNT(*)` query. UI does not show a
  total count, so `COUNT(*)` would be wasted work.
- **Button state during fetch:** disabled + label `Loading…`.
  *Rationale:* prevents double-clicks and race conditions on the in-memory
  list.
- **Re-search behavior:** reset `cursor = null`, `hasMore = false`,
  fetch page 1.
  *Rationale:* requested by the developer.
- **DB-change behavior:** re-fetch each currently-loaded entry by id from
  the DB and update its fields in place; do not add/remove rows.
  *Rationale:* the developer chose this over both "truly noop" and
  "reset to first page". Keeps dirty/sync dots fresh without surprising
  the user with a list restructure mid-session.
- **State preservation across navigation:** module-level ref in
  `state/homeState.ts` carrying `{ query, entries, cursor, hasMore }`.
  Restored directly on mount (no re-fetch). Cleared on logout.
  *Rationale:* requested by the developer. Existing
  `usePreservedHomeSearch` was renamed and extended to carry the full
  page state, since "home search" was no longer an accurate name.
- **Sort order:** unchanged — `updated_at DESC, id ASC`.
  *Rationale:* preserves existing behavior; the cursor is defined in
  terms of this sort.
- **Scope:** web PWA only.
  *Rationale:* the developer specifically asked about the home view.
  Server/CLI unaffected.
- **Branch:** none — work lands directly on `main`.
  *Rationale:* requested by the developer.
- **Plan file:** `docs/agent-plans/home-load-more.md`.
  *Rationale:* standard feature-workflow location.

## Testing strategy

No automated tests. Manual smoke checks only (per `AGENTS.md`).
The dev runs `make dev` (or `make dev-server`),
seeds the local DB via the dev global, and walks the scenarios listed in
Step 3's acceptance criteria.

## Steps

### Step 1 — DB layer: paginate `listEntries` and `searchEntries`
- **Status:** done (reviewed 2026-08-23)
- **What:**
  - In `web/src/db/sqlite.ts`, change the signatures:
    - `listEntries({ cursor?: { updated_at: string; id: string } | null }): { entries: Entry[]; hasMore: boolean }`
    - `searchEntries(query: string, { cursor?: { updated_at: string; id: string } | null }): { entries: Entry[]; hasMore: boolean }`
  - When `cursor` is null: existing query, but `LIMIT 51` instead of
    unbounded. When set: append `AND ((updated_at < ?) OR (updated_at = ? AND id > ?))`
    to the WHERE clause (same `LIMIT 51`).
  - Drop the 51st row from the result. Return
    `{ entries: out, hasMore: out.length === 51 }` after truncating.
  - In `web/src/views/HomeView.vue`, update `reload()` to read
    `.entries` from the new return shape. UI looks identical (still
    shows everything ≤ 51 rows, which is the entire seeded test set
    in dev).
- **Acceptance criteria:**
  - `db.listEntries()` and `db.searchEntries(q)` still return the
    seeded entries in the existing order.
  - Inspecting the methods in DevTools (or reading the source) confirms
    the new shape: `{ entries: Entry[]; hasMore: boolean }`.
  - With < 51 rows seeded, `hasMore === false`. With 51+ rows seeded,
    `hasMore === true`.
  - Passing a cursor that matches the last row of the previous page
    returns the rows that come after it under the same sort.
- **Notes:**
  - Implemented as planned.
  - Two extra changes were needed during the step (raised by the
    developer during review):
    - **`Makefile`** — `make dev-server` did not expose
      `window.__linkomatDb`, so the developer could not run the manual
      tests via DevTools. Added a new target `web-embed-expose-dev`
      (Vite `--mode expose-dev` build + copy into `api/web_dist/`)
      and changed `dev-server` to depend on it. `build-server`
      (production) still depends on plain `web-embed` so production
      bundles do not expose the dev global.
    - **`web/src/db/index.ts`** — the dev-global wrappers
      `listEntries: () => db.listEntries()` and
      `searchEntries: (q) => db.searchEntries(q)` silently dropped
      their argument, so cursor-paged DevTools calls returned page 1
      instead of the cursor's continuation. Updated both to forward
      `opts?: { cursor?: ... }`. Typecheck passes.

### Step 2 — HomeView UI: cursor, Load more button, navigation state preservation
- **Status:** done (reviewed 2026-08-23)
- **What:**
  - In `web/src/views/HomeView.vue`:
    - Add state: `cursor: { updated_at: string; id: string } | null`,
      `hasMore: boolean`, `isLoadingMore: boolean`.
    - `reload()`: set `cursor = null`, `hasMore = false` before
      fetching; read the first page from the new shape; set
      `entries`, `hasMore`, `cursor` (to the last entry's `(updated_at, id)`
      only when `hasMore` would be true after the call).
    - New `loadMore()`: if `!isLoadingMore && hasMore`, set
      `isLoadingMore = true`, fetch the next page with the current
      `cursor`, append `entries.value = [...entries.value, ...page]`,
      set `cursor` to the last appended entry's `(updated_at, id)`,
      update `hasMore` from the page result, then
      `isLoadingMore = false`. Wrap the body in `try { … } finally { isLoadingMore.value = false; }`
      so an unexpected throw does not strand the flag.
    - Render a `<button>` below `<RecycleScroller>` with class
      `load-more`, `data-test="home-load-more"`. Visible only when
      `hasMore`. Disabled + label `Loading…` when `isLoadingMore`.
  - In `web/src/state/homeSearch.ts`:
    - Replace the file contents with `web/src/state/homeState.ts`'s
      content (see Decision log).
    - File rename: `web/src/state/homeSearch.ts` →
      `web/src/state/homeState.ts`.
    - File content:
      ```ts
      import { ref, type Ref } from 'vue';
      import type { Entry } from '../db/sqlite';

      export type HomeState = {
        query: string;
        entries: Entry[];
        cursor: { updated_at: string; id: string } | null;
        hasMore: boolean;
      };

      const preserved = ref<HomeState | null>(null);

      export function usePreservedHomeState(): Ref<HomeState | null> {
        return preserved;
      }

      export function clearPreservedHomeState(): void {
        preserved.value = null;
      }
      ```
  - In `web/src/views/HomeView.vue`:
    - Replace `import { usePreservedHomeSearch } from '../state/homeSearch';`
      with `import { usePreservedHomeState } from '../state/homeState';`.
    - `onBeforeRouteLeave(to)`: if `to.path.startsWith('/entries/')`,
      snapshot `{ query: search.value, entries: entries.value, cursor: cursor.value, hasMore: hasMore.value }`
      into `preservedHomeState.value`.
    - `onMounted`: if `preservedHomeState.value` is non-null, restore
      `search.value`, `debouncedSearch.value`, `entries.value`,
      `cursor.value`, `hasMore.value` directly from the snapshot;
      then `preservedHomeState.value = null`. Otherwise call `reload()`.
  - In `web/src/state/auth.ts` `logout()`: call
    `clearPreservedHomeState()`.
- **Acceptance criteria:**
  - Seeded with 150 entries: HomeView shows 50, Load more button
    visible. Click → +50 entries appear (now 100), button still
    visible. Click → +50 (now 150), button hides.
  - Seeded with 50 entries: no Load more button.
  - Seeded with 51 entries: 50 visible, Load more button visible,
    click → 1 more appears, button hides.
  - Seeded with 200 entries, search a query that matches 120:
    HomeView shows 50 of the 120, Load more visible. Click → +50 (now
    100), button visible. Click → +20 (now 120), button hides.
  - Type a new search while the list has 200 entries loaded → list
    resets to first 50 of the new search.
  - Seeded with 250 entries, load all three pages (now 250 entries,
    no button). Click a row → EditView. Hit back → HomeView shows all
    250 entries, no button, no re-fetch.
  - Seeded with 250 entries, load all three pages, click a row →
    EditView, hit back → HomeView still shows 250. Type a different
    search → list resets to page 1 of the new search.
  - Seeded with 250 entries, load 100 (two pages), click a row →
    EditView, hit back → HomeView shows 100 with Load more visible.
    Type a different search → list resets.
  - Logout: preserved snapshot is cleared. Log back in → HomeView
    starts with whatever the current DB has at page 1, no
    restoration of the stale snapshot.
- **Notes:**
  - Implemented as planned, with three follow-up deviations raised
    during review:
    - **No `RecycleScroller`.** The developer asked to drop the
      fixed-height scroll container so the page scrolls naturally and
      the Load more button stays in view. Replaced
      `<RecycleScroller>` with a plain `<div class="list">` + `v-for`
      and removed the `vue-virtual-scroller` import. CSS: `.list` no
      longer has `height` / `overflow`; rows keep their `border-bottom`.
    - **Scroll-position preservation across navigation.** The
      developer asked for the page scroll position to survive
      leave-and-back. Added `scrollY` to the snapshot. On mount,
      after restoring entries, `await nextTick()` then a 50 ms
      `setTimeout` fires `window.scrollTo(0, snap.scrollY)`. The
      delay lets any browser/Vue-Router scroll side effects land
      first so the explicit `scrollTo` is the last word.
    - **Composable refactor: individual refs, not one object ref.**
      The developer asked for the composable to expose an object of
      individually-reactive refs (no `null` access). Final shape
      (`web/src/state/homeState.ts`): six refs — `query`, `entries`,
      `cursor`, `hasMore`, `scrollY`, `isPreserved` — properly
      initialized; `clearPreservedHomeState` resets them all.
      HomeView destructures them directly (with `query → search`,
      `scrollY → preservedScrollY` aliases) and writes through them,
      so there are no local duplicates of the preserved state. A
      `skipNextReload` counter (1 if state already `ready`, else 2)
      suppresses the watch-driven `reload()` that would otherwise
      overwrite the restored entries when `state` flips to `ready`
      after mount.
    - **Pending-debounce flush on leave.** `onBeforeRouteLeave`
      became `async`; if a search debounce timer is still pending
      when leaving for an `/entries/*` route, it is cleared,
      `debouncedSearch` is set to the latest typed value, and the
      callback `await nextTick()` so the watch fires (and
      `reload()` runs) before navigation commits. Without this, a
      type-then-immediate-click would leave the composable with the
      new query but stale entries.

### Step 3 — DB change handler: refresh-by-id + docs
- **Status:** done (reviewed 2026-08-23)
- **What:**
  - In `web/src/views/HomeView.vue`, replace
    `function onChange(): void { reload(); }` with a per-row refresh:
    for each `entry` currently in `entries.value`, call
    `db.getEntry(entry.id)`; if it returns a row, replace the
    in-memory entry with the fresh fields (so `local_dirty_at` and
    friends update); if it returns `null` (server-side soft-delete
    applied between pages), leave the in-memory entry alone — the
    row will vanish on the next real reload. Do not add or remove
    rows.
  - In `doc/architecture/web.md`, update the `LocalDb.searchEntries` /
    `listEntries` description to mention the cursor + `hasMore` shape
    and the new `state/homeState.ts` composable.
- **Acceptance criteria:**
  - Edit an entry in EditView (creates a local dirty mark). Return to
    HomeView. The row's dot turns orange. Wait for sync to push
    (~2 s debounce + a network round-trip). The dot turns green
    without leaving HomeView (the DB-change event fires from
    `clearDirty` → `notifyDbChange`).
  - With 250 entries fully loaded, trigger a sync that pulls a brand
    new entry from the server (e.g. create one via `linkomat add` on
    the CLI while the PWA is open on HomeView). The list does not
    restructure; the new entry is not visible. Navigate away and
    back → the new entry is at the top of the list (first 50).
  - `doc/architecture/web.md` mentions the new method shape and the
    new composable.
- **Notes:**
  - `onChange` implemented as planned (per-row refresh, no add/remove).
  - `doc/architecture/web.md` updated (listEntries / searchEntries
    bullet + new "Home state" bullet).
  - **Follow-up deviation raised during review (stale-data fix):**
    - The original "do nothing on DB change, next load shows new
      entries" path left a real bug: while HomeView is unmounted
      (user on EditView), LOCAL writes (add / edit / delete) are
      invisible to the snapshot-restore path. Restoring the
      snapshot verbatim re-showed pre-edit entries and missed
      locally-added entries.
    - Fix applied: drop `entries` / `cursor` / `hasMore` from the
      composable. Snapshot now carries `query` + `numPages` (≥ 1) +
      `scrollY` + `isPreserved`. On restore, HomeView re-fetches
      fresh data: `reload()` for page 1, then a `for` loop calling
      `loadMore()` `numPages - 1` more times. New entries from
      LOCAL add land at the top automatically (reload picks them
      up); LOCAL edits are reflected (each fetch returns current
      fields); LOCAL deletes vanish (the `deleted_at IS NULL`
      filter drops them). The loaded-pages count is preserved
      exactly. The natural sort order means the oldest entries get
      pushed off the bottom if newer ones arrived — the user can
      click Load more to see them. `onChange` (refresh-by-id) is
      kept for the in-session sync-clear case.

### Step 4 - Check if the architecture doc needs updated

- **Status:** done (reviewed 2026-08-23)
- Update the doc/architecture documentation where needed. Do not introduce new documentation chapters. the architecture doc should stay a high-level document, not document every small implementation detail.
- **Notes:** Done as part of Step 3 — `listEntries` / `searchEntries` bullet
  + new "Home state" bullet in `doc/architecture/web.md`. No new
  sections introduced.

## Open questions

- None at planning time. The `RecycleScroller`'s default scroll
  behavior after items are appended (keep the previous viewport
  bottom in place) is the expected UX; if it does not, we revisit
  in Step 2.
