# Header icons (logout + on/offline)

**Ticket/Source:** none
**Branch:** none (stay on `main`)
**Status:** done

## Goal

Replace the text "Logout" button on the home view with an icon-only
button and add matching inline-SVG icons for the on-/offline state.
Reorder the HomeView header so the user + logout row sits above the
on-/offline state row (currently the order is reversed).

## Scope

**In scope:**
- Web PWA only.
- Three new icon components under `web/src/components/icons/`:
  `LogoutIcon.vue`, `OnlineIcon.vue`, `OfflineIcon.vue`. Each renders
  the glyph directly as inline SVG (no `<img>`, no external asset,
  no icon library).
- `web/src/views/HomeView.vue`:
  - Header order swap: `home-meta` row (username + logout) moves
    above `sync-status` row (pending + state + retry).
  - Logout button becomes icon-only.
  - The on-/offline state in `home-status-state` becomes icon-only
    for the binary online/offline cases. Text labels for
    `Syncing…` and `Error: …` are unchanged.
- `doc/architecture/web.md`: extend the `components/` bullet under
  the top-level layout listing to mention the new `icons/` subfolder
  and the three files.
- No code comments.

**Out of scope:**
- Server / API changes. No new endpoints, no new Go files.
- CLI changes.
- Local DB schema changes. No `CURRENT_SCHEMA_VERSION` bump.
- Automated tests / CI. Manual smoke checks only.
- Recoloring or restyling the existing per-row status dot, the
  pending pill, or the "Retry now" button. They keep their current
  CSS.
- New `data-test` hooks on the icons themselves. The parent
  `home-logout`, `home-status-chip`, `home-status-state` hooks are
  already in place and remain sufficient for smoke tests.
- Removing the `stateLabel` text path for syncing/error. Only the
  online/offline case drops the text.
- A tooltip / popover showing `lastError` details on hover. The
  existing `Error: …` text remains the only surfacing of that
  message.
- Dark mode / theme toggles.
- Refactoring the state-computed `stateLabel` to be shared between
  icon and text branches. The decision is encoded inline in the
  template (one icon in two branches, text in two branches).
- Adding additional icon components (edit, delete, search, etc.).

## Rough Architecture / Concept

Three tiny single-file Vue 3 components, each containing exactly
one inline `<svg>` element. The SVG markup uses
`stroke="currentColor"` (and `fill="none"`) so the parent's
`color:` CSS drives the glyph colour — no per-component colour
hardcoding, no `:style` binding, no theme prop. Each SVG has a
fixed `width="1.25rem"` / `height="1.25rem"` and a 24×24 `viewBox`,
so the markup scales cleanly inside a small text-sized line.

`LogoutIcon` is the canonical door-and-arrow glyph (rectangle on
the left with an arrow exiting to the right). `OnlineIcon` is the
three-arc wifi cone over a small origin dot. `OfflineIcon` is the
same wifi cone with a diagonal slash overlaid (mirroring how the
home view's online indicator is currently rendered — a green dot for
online and grey for offline). All three are stroke-only with
`stroke-linecap="round"` / `stroke-linejoin="round"` for clean
small-size rendering.

The HomeView header change is a template re-order + two small
text-to-icon swaps. No new state, no new computed properties, no
script-logic changes. The existing `stateLabel` / `stateLabelClass`
computeds stay; the template just renders the icon inside
`home-status-state` instead of the text when the state is online or
offline.

## Decision log

- **Online/offline icon set: wifi arcs vs wifi-arcs-with-slash.**
  *Rationale:* developer choice. Universally recognized "radio
  connected vs not" pair at small sizes.
- **Files under `web/src/components/icons/`.**
  *Rationale:* developer choice. Keeps the existing components/
  root (currently `AppFooter.vue`) tidy; future icon additions
  have an obvious home.
- **Logout button a11y: `aria-label="Logout"` + `title="Logout"`.**
  *Rationale:* developer choice. Screen-reader announcement +
  hover affordance both work; matches the visible label we are
  removing.
- **Icon size: 1.25rem (20px) inline.**
  *Rationale:* developer choice. Sits comfortably next to the
  username text and balances with the on/offline icon. Smaller
  (1rem) felt too quiet; larger (1.5rem) would overpower the
  line.
- **Icon coloring: `stroke="currentColor"`.**
  *Rationale:* developer choice. Parent CSS already sets the right
  color via `.sync-state-online` (green) / `.sync-state-offline`
  (grey) / `.sync-state-active` (blue) / `.sync-state-error` (red)
  — icons inherit for free with zero new style code.
- **Online/offline icon-only; syncing/error stay text.**
  *Rationale:* developer choice. The binary online/offline state
  is the most common case shown most often; an icon is enough.
  Syncing is transient and reads better as the existing text.
  Error needs the human-readable `lastError` payload.
- **Visual order in the state line: icon only (no accompanying
  text).**
  *Rationale:* developer choice. The label `"Online"` / `"Offline"`
  is redundant when the icon is already colour-coded and shaped.
- **Header order: home-meta on top, sync-status below.**
  *Rationale:* developer choice. The user identity + logout action
  are primary; the sync chip is secondary.
- **`aria-hidden="true"` on the icons themselves; accessible label
  on the wrapping span for online/offline.**
  *Rationale:* the icon is decorative next to its parent button
  label. For the icon-only online/offline span, a parent
  `aria-label` ("Online" / "Offline") gives screen readers the
  state without forcing the user to interpret the SVG.
- **Decorative-only `aria-hidden="true"` on the logout icon inside
  the icon-only button.**
  *Rationale:* the button already carries `aria-label="Logout"`,
  so the inner SVG should not be announced separately.
- **`viewBox="0 0 24 24"` on all three icons.**
  *Rationale:* matches the existing `vue-virtual-scroller` row
  sizing convention (rows are 96px tall, with a 1rem ≈ 16px
  baseline). Standard heroicons / lucide / feather grid.
- **No `data-test` hooks on the icons themselves.**
  *Rationale:* existing parent hooks (`home-logout`,
  `home-status-state`) are sufficient for smoke tests.
- **Architecture doc: small bullet addition only.**
  *Rationale:* AGENTS.md says architecture docs are the source
  of truth on conflict; the new subfolder is worth one bullet.
  No new sections.
- **Branch: stay on `main`.**
  *Rationale:* developer choice. Matches the previous
  `footer-and-about-page.md` plan.
- **Plan file path: `docs/agent-plans/header-icons.md`.**
  *Rationale:* developer choice. Short and matches the previous
  naming convention (`footer-and-about-page.md`,
  `home-load-more.md`).
- **Out-of-scope items listed above.**
  *Rationale:* scope-creep guard. Each item is a small thing
  that could be added later without touching the present change.

## Testing strategy

No automated tests. Manual smoke checks only (per `AGENTS.md`).
Developer runs `make dev` (Vite + Go server) and walks the scenarios
in Step 1's acceptance criteria.

## Steps

### Step 1 — Three icon components + header layout + doc bullet

- **Status:** in progress
- **What:**
  - Create `web/src/components/icons/LogoutIcon.vue`:
    - `<template>` only; no `<script setup>` (matches the
      no-logic pattern of `AppFooter.vue` when no logic is
      needed — see footer for the established precedent).
    - Renders one `<svg xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24" width="1.25rem" height="1.25rem"
      fill="none" stroke="currentColor" stroke-width="1.75"
      stroke-linecap="round" stroke-linejoin="round"
      aria-hidden="true">`.
    - Path data: a door/box on the left with an arrow exiting
      right. Concretely:
        - `<rect x="3" y="3" width="7" height="18" rx="1" />`
        - `<path d="M10 12h11" />`
        - `<path d="m17 8 4 4-4 4" />`
    - No scoped styles; the icon inherits colour from the
      parent via `currentColor`.
  - Create `web/src/components/icons/OnlineIcon.vue`:
    - Same SVG shell (24×24, 1.25rem, `stroke="currentColor"`,
      `fill="none"`, `stroke-width="1.75"`,
      `stroke-linecap="round"`, `stroke-linejoin="round"`,
      `aria-hidden="true"`).
    - Path data: three ascending wifi arcs over a small origin
      dot:
        - `<path d="M5 12.55a11 11 0 0 1 14 0" />`
        - `<path d="M8.5 16.05a6 6 0 0 1 7 0" />`
        - `<path d="M12 19.5h.01" />`
  - Create `web/src/components/icons/OfflineIcon.vue`:
    - Same shell.
    - Path data: same three arcs/dot as `OnlineIcon`, then a
      slash overlay cutting diagonally across the icon:
        - `<path d="M5 12.55a11 11 0 0 1 14 0" />`
        - `<path d="M8.5 16.05a6 6 0 0 1 7 0" />`
        - `<path d="M12 19.5h.01" />`
        - `<line x1="3" y1="3" x2="21" y2="21" />`
  - Modify `web/src/views/HomeView.vue`:
    - Import the three icons:
      `import LogoutIcon from '../components/icons/LogoutIcon.vue';`
      `import OnlineIcon from '../components/icons/OnlineIcon.vue';`
      `import OfflineIcon from '../components/icons/OfflineIcon.vue';`
    - Reorder the two header rows in the `<template>`: move
      `<div class="home-meta">` above `<div class="sync-status">`.
      The internal markup of `home-meta` and `sync-status` stays
      the same except for the two swaps below.
    - Logout button: replace the text node `Logout` with
      `<LogoutIcon />`. Add `aria-label="Logout"` and
      `title="Logout"` to the `<button>`. Keep
      `data-test="home-logout"`.
    - `home-status-state` `<span>`: when the state is online or
      offline, render the matching icon inside the span instead
      of the text. The state-class-based colour continues to
      drive the icon via `currentColor`. Add
      `aria-label="Online"` / `aria-label="Offline"` (computed
      via the existing `stateLabel` value, or a small inline
      conditional) so screen readers announce the state.
      The text branch (`Syncing…` / `Error: …`) is unchanged.
      Concrete template change: replace
      `{{ stateLabel }}` with a `v-if`/`v-else-if`/`v-else`
      block:
        - `v-if="syncState.online"`: `<OnlineIcon />` (no
          text node).
        - `v-else-if="!syncState.syncing && !syncState.lastError"`:
          `<OfflineIcon />` (no text node).
        - `v-else`: `{{ stateLabel }}` (renders `Syncing…` or
          `Error: …` unchanged).
    - Confirm `home-meta` already uses
      `justify-content: flex-end` (or `space-between`); if it
      uses `space-between`, change to `flex-end` so username
      and logout sit together at the right edge with the
      expected gap.
    - Confirm the existing `.sync-status` styles still look
      right with the icon-only branch: the icon sits at
      `font-size` baseline; the existing `gap: 0.5rem` keeps
      pending/retry separated from the icon.
  - Modify `doc/architecture/web.md`:
    - In the top-level layout listing, extend the
      `components/` bullet. Current text is:
        ```
        ├── components/
        │   └── AppFooter.vue      # copyright + alexi.ch link + /about link; rendered below RouterView on every route
        ```
      Add a sub-bullet listing the new `icons/` folder and its
      three files, in the same compact one-line format:
        ```
        │   └── icons/
        │       ├── LogoutIcon.vue   # inline-SVG logout glyph used by the icon-only logout button on HomeView
        │       ├── OnlineIcon.vue   # inline-SVG wifi glyph for the online sync state
        │       └── OfflineIcon.vue  # inline-SVG wifi-glyph-with-slash for the offline sync state
        ```
      Adjust indentation to match the surrounding bullet
      style.
- **Acceptance criteria:**
  - `make dev` (or `cd web && npm run dev`) and log in.
  - Header on `/`:
    - Top row, right-aligned: `<username>` followed by an
      icon-only button containing the logout glyph. No text
      inside the button.
    - Below: the wifi icon (or the wifi-slash icon when
      offline). When online the icon is green (matches
      `.sync-state-online` color). When offline the icon is
      grey (matches `.sync-state-offline` color). No "Online"/
      "Offline" text label visible.
    - The pending pill and "Retry now" button (if applicable)
      still appear on the second row, unchanged.
  - Hovering the logout button shows the `Logout` tooltip
    (browser-native title); screen-reader announces
    "Logout, button".
  - Toggle DevTools → Network → Offline:
    - Header wifi icon flips to the slashed version, greyed.
    - On subsequent "Online" restore, icon returns to the
      green wifi arcs (no slash).
  - Trigger a sync (e.g. create an entry, edit, save) so the
    engine runs: header shows the existing `Syncing…` text
    (or, if you make a lastSync fail, `Error: …` text).
  - `data-test` hooks still resolve via DOM queries:
      `home-username`, `home-logout`, `home-status-chip`,
      `home-status-pending`, `home-status-state`,
      `home-status-retry`. The new icons themselves carry no
      hooks.
  - Typecheck: `cd web && ./node_modules/.bin/vue-tsc --noEmit`
    passes.
  - `doc/architecture/web.md` lists the new `icons/`
    subfolder in the top-level layout.
- **Notes:**
  - Implemented as planned in the first pass; one deviation
    raised by the developer during review (see below) and
    applied in place.
  - **Deviation 1 — LogoutIcon glyph swapped for the Font
    Awesome Free 7.3.1 solid `right-from-bracket` glyph.**
    Developer said the original stroke-style door+arrow was
    "ugly" and pointed at
    `https://fontawesome.com/icons/classic/solid/right-from-bracket`
    as the target look. New file uses Font Awesome's exact
    path data (single `<path>`, two subpaths, `fill="currentColor"`,
    `viewBox="0 0 512 512"`, no stroke attributes) at the same
    `width/height="1.25rem"`. The shape stays consistent at
    the 1.25rem display size; CSS still drives the colour via
    `currentColor`.
  - **Deviation 2 — License attribution comment added in
    `LogoutIcon.vue`.** CC BY 4.0 requires attribution to
    Fonticons, Inc. for the icon. Added a single-line
    HTML comment at the top of the `<template>`:
    `<!-- Glyph: Font Awesome Free 7.3.1 right-from-bracket
    (solid). CC BY 4.0. https://fontawesome.com/icons/right-from-bracket -->`
    Override of the project's "no comments" convention is
    deliberate — the project's `AGENTS.md` rule is for
    code comments; license notices in template markup are
    distinct.
  - **Visual inconsistency note.** The new `LogoutIcon` is a
    fill-style glyph; the existing `OnlineIcon` and
    `OfflineIcon` are stroke-style. At the 1.25rem display
    size the mix reads fine; developer accepted this when
    asking for the Font Awesome look.
  - **Typecheck:** `cd web && ./node_modules/.bin/vue-tsc
    --noEmit` passes.
  - No `data-test` hooks added (parent `home-logout` is
    sufficient).
  - Other files unchanged in this revision: `OnlineIcon.vue`,
    `OfflineIcon.vue`, `HomeView.vue`, `doc/architecture/web.md`.
  - **Deviation 3 — Status-group moved into the title row;
    one header row removed.** Developer asked to put the
    on-/offline icon "to the right (right-end side) of the
    app title (linkomat)" to save a line. After confirming
    where the pending pill and Retry now button should go
    (developer chose: same title row, right-aligned
    alongside the icon), the `.sync-status` wrapper row
    was removed entirely.
  - **New template structure inside `.home-header`:**
    1. `<div class="home-title-row">` containing
       `<h1 class="home-title">Linkomat</h1>` (left) and
       `<div class="home-status-group"
       data-test="home-status-chip">` (right). The status
       group keeps the existing pending pill, state span
       (with icon-or-text), and Retry button in their
       original visual order: pending → state → retry.
    2. `<input class="search">` — unchanged.
    3. `<div class="home-meta">` (username + logout) —
       unchanged, still right-aligned.
  - **New CSS:**
    - `.home-title-row` — `display: flex; align-items:
      center; justify-content: space-between; gap: 0.75rem;`
    - `.home-status-group` — `display: flex; align-items:
      center; gap: 0.5rem; font-size: 0.875rem; flex-wrap:
      wrap; justify-content: flex-end;` (same internal
      spacing as the deleted `.sync-status`).
    - Old `.sync-status` rule removed (dead code).
  - **`data-test` hooks preserved:** `home-status-chip`
    moved from the old `.sync-status` to
    `.home-status-group`; `home-status-pending`,
    `home-status-state`, `home-status-retry` unchanged.
  - **Typecheck** still passes after this revision.

## Open questions

- None. All decisions resolved during the grilling phase.