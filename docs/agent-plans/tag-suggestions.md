# Tag suggestions in the tag input

**Ticket/Source:** none (developer request in chat, 2026-11-08)
**Branch:** none (stay on `main`)
**Status:** done

## Goal

When the user types into the tag input of the entry detail view
(`EditView`), a dropdown shows tags that the user has already used on other
entries and that contain the typed text. Selecting one adds it as a tag.
If the user ignores the dropdown and keeps typing, the existing tag-entry
behaviour (space / comma / Enter / blur create a tag) stays unchanged.
Goal: consistent tag spelling, less typing on mobile.

## Scope

**In scope:**
- Web PWA only.
- New component `web/src/components/TagInput.vue` that takes over the tag
  chips, the tag input and the existing separator logic from
  `web/src/views/EditView.vue`, plus the new suggestion dropdown.
- New `LocalDb.listDistinctTags()` in `web/src/db/sqlite.ts`, exposed on
  the dev object `window.__linkomatDb` in `web/src/db/index.ts`.
- Keyboard navigation of the dropdown on desktop.
- Short updates in `doc/architecture/web.md` and the `AGENTS.md`
  "Where to make changes" table. The architecture doc stays high-level.

**Out of scope:**
- Server / API changes. No new endpoint.
- Local DB schema change. No new table, no `CURRENT_SCHEMA_VERSION` bump.
- HomeView search / filter by tag.
- Tag management (rename, delete, merge tags).
- Automated tests / CI.

## Rough Architecture / Concept

Tags live as JSON-in-TEXT in `entries.tags` (e.g. `["go","web"]`) in the
local sql.js DB. `sql.js@1.14.2` ships SQLite 3.49.1, where `json_each()`
is built in (verified with `node -e` against the installed package). One
query gets the unique tag list directly from the JSON:

```sql
SELECT DISTINCT j.value AS tag
FROM entries e, json_each(e.tags) j
WHERE e.deleted_at IS NULL AND j.type = 'text'
```

`TagInput.vue` receives the current tags via `v-model` and a
`suggestionSource: () => string[]` prop (EditView passes
`() => db.listDistinctTags()`). On input focus it calls the source once
and keeps the list in memory. On every input change it filters that list
in JS (case-insensitive via `toLocaleLowerCase()`, so umlauts match),
drops tags the entry already has, sorts prefix matches first then the
rest, each group alphabetical (`localeCompare`), and shows at most 8.
The dropdown is an absolutely positioned overlay under the tag box.
Items react on `pointerdown.prevent` so the input does not blur (the
existing `@blur` would otherwise commit the half-typed text as a tag).
EditView stays responsible for persisting: TagInput emits `change` after
every tag add / remove, EditView calls `saveField()`.

## Context for a new agent

- **Sources read:** `AGENTS.md` (no code comments, no automated tests,
  `LocalDb` is the only sql.js consumer, FTS3 only), `doc/architecture/web.md`
  (component tree, `db/sqlite.ts` is the only file touching sql.js),
  `docs/agent-plans/home-load-more.md` (plan style reference).
- **Key files and directories:**
  - `web/src/views/EditView.vue` — entry detail view; today holds the tag
    chips + input (`tagInput` ref, `TAG_SEPARATOR = /[,\s]+/`,
    `commitTags`, `addTag`, `onTagInput`, `removeTag`, CSS classes
    `.tags`, `.tag-chip`, `.tag-remove`, `.tag-input`, data-test ids
    `edit-tags`, `edit-tag-input`, `edit-tag-remove-<tag>`). `saveField()`
    persists the whole form via `db.upsertEntry`.
  - `web/src/components/TagInput.vue` — new, target of the extraction.
  - `web/src/db/sqlite.ts` — `LocalDb` class; add `listDistinctTags()`.
    Pattern: `this.db.prepare(...)`, `stmt.step()`, `stmt.getAsObject()`,
    `stmt.free()`, `if (!this.db) throw new Error('db not open')`.
  - `web/src/db/index.ts` — `exposeDev()` builds `window.__linkomatDb`.
  - `web/src/db/schema.ts` — schema, `entries.tags TEXT NOT NULL DEFAULT '[]'`.
- **Existing patterns to follow:** components in `web/src/components/`
  (e.g. `ShareDialog.vue`), `<script setup lang="ts">`, scoped CSS with
  plain hex colors.
- **Mobile input facts (from the separator fix done just before this
  feature):** mobile keyboards send `keydown` with `key = "Unidentified"`,
  and `v-model` defers updates during IME composition. Therefore the input
  uses `:value` + `@input` (`onTagInput`) that splits on `TAG_SEPARATOR`;
  only Enter uses `@keydown`. Keep this.
- **Commands:**
  - Type check: `cd web && npx vue-tsc --noEmit -p .`
  - Dev server: `cd web && npm run dev`
  - Build: `cd web && npm run build`
- **Environment / prerequisites:** a logged-in user with some entries that
  have tags; the Go server for login/sync (see `doc/architecture/build.md`).
- **Domain terms:** *entry* = saved link (title, url, note, tags);
  *LocalDb* = sql.js wrapper in the browser (OPFS-persisted).
- **External references:** SQLite JSON functions
  https://sqlite.org/json1.html (built in since 3.38; installed 3.49.1).

## Decision log

- **Data source:** local sql.js DB via `json_each`. — *Rationale:* offline-first
  app, no network latency, no server change. *Rejected:* new server API
  endpoint (needs online, adds route + DTO).
- **Matching:** case-insensitive substring, done in JS with
  `toLocaleLowerCase().includes()`. — *Rationale:* SQLite `lower()` only
  folds ASCII, so `ä` would not find `Ärger`. *Rejected:* SQL `instr(lower())`;
  case-sensitive match.
- **Trigger:** dropdown shows from 1 typed character.
- **Already-set tags:** hidden from the dropdown. — *Rationale:* selecting
  them would be a no-op.
- **Order / limit:** prefix matches first, then other substring matches;
  each group alphabetical (`localeCompare`); max 8 items. — *Rationale:* fits
  above the mobile keyboard. *Rejected:* usage-count order; unlimited list.
- **Keyboard:** Arrow Down / Up move the highlight (no highlight by
  default), Enter adds the highlighted tag, Enter without highlight keeps
  the current behaviour (adds typed text), Esc closes the dropdown.
  *Rejected:* mouse / tap only.
- **UI:** overlay dropdown, absolutely positioned under the tag box.
  *Rejected:* inline list (layout jumps); horizontal chip row.
- **Structure:** extract a `TagInput.vue` component. *Rejected:* keep
  everything in `EditView.vue` (already ~650 lines).
- **Load time:** read the distinct tags once on input focus, filter in
  memory per keystroke. *Rejected:* query per keystroke; load on mount +
  `DB_CHANGE_EVENT`.
- **Selecting from the dropdown:** clear the input, keep focus, close the
  dropdown, emit `change`.
- **Tag input styling:** keep the current look (the tag input has its own
  border inside the bordered tag box, a result of `.field input` beating
  `.tag-input` in the old EditView CSS). *Rejected:* borderless input.
- **Label bug (existed before this feature):** fixed. The Tags field in
  `EditView.vue` was a `<label class="field">`; a click on chip text, the
  `.tags` padding or the "Tags" text clicked the first labelable
  descendant = the first chip's × button and removed the first tag. Now a
  `<div class="field">`; the input in `TagInput.vue` has
  `aria-label="Tags"` to keep its accessible name.
- **Implementation mode:** developer approved implementing all steps
  without a review gate between steps, using sub-agents (parallel where
  independent). Steps 1 + 2 run in parallel; Steps 3 + 4 in one agent
  (same file); Step 5 after.

## Testing strategy

No automated tests (per `AGENTS.md`). Manual review on desktop browser and
a mobile device, using the acceptance criteria below.

## Steps

### Step 1 — Extract `TagInput.vue`
- **Status:** done (reviewed 2026-11-08)
- **What:** Move tag chips, input, `TAG_SEPARATOR`, `commitTags`,
  `addTag`, `onTagInput`, `removeTag` and the tag CSS from `EditView.vue`
  into `web/src/components/TagInput.vue`. Props: `modelValue: string[]`.
  Emits: `update:modelValue`, `change`. EditView uses
  `<TagInput v-model="form.tags" @change="saveField" />`. Keep data-test
  ids. No behaviour change.
- **Acceptance criteria:** space, comma, Enter and blur add tags on
  desktop and mobile; × removes a tag; each change persists (reload the
  page, tags remain).
- **Notes:** EditView uses `<TagInput v-model="form.tags" @change="saveField" />`
  inside the existing `<label class="field">`. Component `v-model` updates
  the parent synchronously, so `form.tags` is current when `saveField` runs.
  `change` is declared in `defineEmits`, so native `change` events of the
  inner input do not fall through to the root and do not call `saveField`
  twice. Styling: in the old EditView CSS, `.field input` (specificity
  0,2,1) won over `.tag-input` (0,2,0), so the tag input actually had a
  border, padding, `width: 100%` and a focus outline. Those computed values
  were copied into `.tag-input` / `.tag-input:focus` in `TagInput.vue` to
  keep the look identical (see Open questions).

### Step 2 — `LocalDb.listDistinctTags()`
- **Status:** done (reviewed 2026-11-08)
- **What:** Add `listDistinctTags(): string[]` to `LocalDb` with the SQL
  above. Add it to `exposeDev()` in `web/src/db/index.ts`.
- **Acceptance criteria:** in dev, DevTools console
  `__linkomatDb.listDistinctTags()` returns each used tag once; tags only on
  soft-deleted entries are missing.
- **Notes:** Method placed after `getEntry` in `web/src/db/sqlite.ts`.
  Final SQL guards against invalid JSON (without the guard, one malformed
  `tags` row makes the whole query fail with `malformed JSON`):
  `SELECT DISTINCT j.value AS tag FROM (SELECT tags FROM entries WHERE deleted_at IS NULL AND json_valid(tags)) e, json_each(e.tags) j WHERE j.type = 'text' AND j.value <> ''`.
  Verified in node against the installed sql.js: unique tags, soft-deleted
  rows, empty strings, numbers and null are excluded. Known side effect: a
  JSON object (not array) in `tags` yields its text values; the app always
  writes arrays, so no extra guard.

### Step 3 — Suggestion dropdown
- **Status:** done (reviewed 2026-11-08)
- **What:** Prop `suggestionSource?: () => string[]` on `TagInput.vue`.
  Load on focus, filter / sort / limit as in the decision log, overlay
  dropdown, select via `pointerdown.prevent`. Hide on blur, on empty
  input, on no matches. EditView passes the DB source.
- **Acceptance criteria:** typing 1 char shows matching used tags (not the
  ones already on the entry); tapping one adds it and does not add the
  half-typed text; typing past the list and pressing space / comma works
  as before.
- **Notes:** EditView passes `:suggestion-source="tagSuggestions"`
  (returns `[]` when the DB is not ready or throws). Root of `TagInput.vue`
  is now `.tag-input-root` (`position: relative`); the `ul.tag-suggestions`
  is `position: absolute; top: 100%; z-index: 10`, shown with `v-show` so
  the `aria-controls` target always exists. Listbox id from Vue `useId()`.
  Input has `role="combobox"`, `aria-autocomplete`, `aria-expanded`,
  `aria-controls`, `aria-activedescendant`.
  **Deviation — selection event:** the item is selected on `click`, not on
  `pointerdown`. `pointerdown.prevent` + `mousedown.prevent` on the `ul`
  only keep focus in the input (so `@blur` does not commit the half-typed
  text). Reason: the component sits inside EditView's `<label class="field">`.
  A click on non-interactive content inside a label clicks the label's first
  labelable descendant = the first chip's × button → removes a tag.
  Selecting on pointerdown removes the `li` before the click arrives, the
  click then lands on a common ancestor inside the label and triggers that.
  Now the `li` handles the click while still in the DOM, and `@click.prevent`
  on the `ul` cancels the label activation.

### Step 4 — Keyboard navigation
- **Status:** done (reviewed 2026-11-08)
- **What:** Arrow Down / Up, Enter, Esc as in the decision log.
- **Acceptance criteria:** desktop: arrows move the highlight, Enter adds
  the highlighted tag, Enter without highlight adds the typed text, Esc
  closes the list.
- **Notes:** Implemented in `onKeydown` in `TagInput.vue`. Highlight is
  clamped (ArrowUp from 0 → -1 = no highlight). Esc sets `dismissed`; the
  next typed character opens the list again. `onTagInput` (space / comma
  split) is unchanged, it only also resets highlight and `dismissed`.

### Step 5 — Docs
- **Status:** done (reviewed 2026-11-08)
- **What:** `doc/architecture/web.md` component tree: add `TagInput.vue`
  and mention suggestions in the EditView line. `AGENTS.md` "Where to make
  changes": add a "Tag input / suggestions" row.
- **Acceptance criteria:** diff read by the developer.
- **Notes:** `doc/architecture/web.md`: `TagInput.vue` line in the
  component tree, EditView line mentions `TagInput, with suggestions`,
  `listDistinctTags()` bullet in the Local DB method list, FAQ entry "How do
  tag suggestions work?". `AGENTS.md`: bullet on SQLite JSON functions in
  "Web PWA specific", row `Tag input / suggestions` in "Where to make
  changes".

## Open questions

None.

## Current state / Next action

Feature done and approved by the developer on 2026-11-08 after manual
testing. All changes are uncommitted on `main`, together with the earlier
separator fix (space / comma create tags). No next action.
