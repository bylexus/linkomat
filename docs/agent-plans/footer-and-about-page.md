# Footer and about page

**Ticket/Source:** none
**Branch:** none (stay on `main`)
**Status:** done

## Goal

A small, unobtrusive footer that renders at the end of every page (login,
home, edit) with the line `© 2026 alexi.ch` (alexi.ch links out to
https://alexi.ch/) and an internal link to a new About page. The About
page describes the app in a few short sentences. The About page must be
reachable while logged out — it's a static info page and has nothing to
do with the user's data.

## Scope

**In scope:**
- Web PWA only.
- New `web/src/components/AppFooter.vue` component. Renders inline at
  the end of the page (not fixed, not overlaid) on every route.
- Integrate `AppFooter` in `web/src/App.vue` next to `<RouterView/>`.
- New `web/src/views/AboutView.vue`. Route `/about` registered in
  `web/src/router.ts`. Reachable while logged out.
- `web/src/App.vue` auth watcher: the logged-out → `/login` redirect
  must not fire for `/about`.
- `doc/architecture/web.md`: extend the views list and App.vue
  description to mention the footer and AboutView.
- Agent-drafted about copy, presented at the Step 1 review gate.

**Out of scope:**
- Server / API changes. No new endpoints, no new Go files.
- CLI changes.
- Local DB schema changes. No `CURRENT_SCHEMA_VERSION` bump.
- Automated tests / CI.
- Dark mode / theme toggles. App is light-only in v1.
- "Last updated" timestamp on the About page.
- Localization. The page is English to match the rest of the UI.
- Footer on the auth-loading splash. The splash is an `v-if` branch
  in App.vue and the footer renders after it; while the splash is up
  only the splash is visible. Once the splash clears, the footer
  appears below the route content.
- Multiple external links or social links in the footer. Just
  copyright + alexi.ch + About.
- A breadcrumb or back link on the About page. Plain page with a
  heading and paragraphs.
- Marking the alexi.ch link as a "follow" / "nofollow" link. Default.
- Adding `Privacy` / `Terms` / `Imprint` links. Only the requested
  About link.

## Rough Architecture / Concept

A `AppFooter.vue` component holds the copyright line and the About
link. App.vue renders it after `<RouterView/>` so it always appears
below the route content on all three routes. `App.vue` already shows
the loading splash via `v-if` and only renders `<RouterView/>` (and
therefore the footer) after auth settles — that's the natural way for
it to coexist with the existing splash logic. No wrapper component
needed, no new layout pattern.

`AboutView.vue` is a small static page (heading + body) styled like
the existing views: scoped styles, `data-test` hooks for any future
test wiring, no comments. Registered in `router.ts` as the third
non-login route.

The auth watcher in `App.vue` currently redirects a logged-out user
to `/login` for any path other than `/login`. Extend the predicate
to also skip `/about`. Logged-in users can also visit `/about` with
no extra logic — the only logged-in branch today sends someone off
`/login` and back to `/`, which doesn't apply on `/about`.

## Decision log

- **Footer placement: inline at end of page, not fixed.**
  *Rationale:* developer choice. Avoids overlap with fixed `.fab`
  on the home view and matches how mobile-friendly pages typically
  treat footers. Short pages (login, edit) show the footer right
  under the content; long pages (home list) require scroll.
- **Footer visibility: all 3 routes including `/login`.**
  *Rationale:* developer choice. Footer lives in App.vue next to
  `<RouterView/>`; simplest placement.
- **About page: public, no auth required.**
  *Rationale:* developer choice. Footer link is useful even before
  sign-in. Watcher change: redirect predicate becomes
  `p !== '/login' && p !== '/about'`.
- **Route path: `/about`.**
  *Rationale:* developer choice. Matches the page name and
  conventional short paths.
- **About heading: "About Linkomat", plain centered block.**
  *Rationale:* developer choice. Matches LoginView's centered
  max-width pattern.
- **Alexi.ch link: `target="_blank" rel="noopener noreferrer"`.**
  *Rationale:* standard security / accessibility pattern for
  outbound links. Mirrors the row URL links in `HomeView.vue:281`.
- **Footer markup: separate `AppFooter.vue` component.**
  *Rationale:* separation of concerns; the footer is independent of
  routing and could be reused. Smaller files; easier to review.
- **About copy: agent-drafted at implementation time, presented at
  the review gate.**
  *Rationale:* developer choice. Length target: 2–4 short
  sentences. Tone: matches the README abstract (first-person,
  no marketing language).
- **Copyright year: 2026.**
  *Rationale:* developer specified. Hardcoded in the component;
  no dynamic year. Out of scope: a "last updated" feel.
- **Architecture doc: update `web.md` only.**
  *Rationale:* AGENTS.md says architecture docs are the source of
  truth on conflict and they should stay high-level. The new view,
  new route, and AppFooter all belong in the existing views /
  router bullet — no new sections, no new files.
- **Branch: none — land on `main`.**
  *Rationale:* developer preference; previous plan
  (`home-load-more.md`) did the same.
- **Plan file path: `docs/agent-plans/footer-and-about-page.md`.**
  *Rationale:* standard feature-workflow location.
- **Out-of-scope items listed above.**
  *Rationale:* scope creep guard. Each item is a small thing that
  could be added later without touching the present change.
- **About body sentence about the server shortened to "…sync in the
  background to the server."**
  *Rationale:* developer choice. Removed the "a small Go server you
  run yourself" agent had drafted.
- **About page duplicates the footer copyright line.**
  *Rationale:* developer choice. `web/src/views/AboutView.vue` ends
  with a `<p class="about-copy">` containing the same
  `© 2026 [alexi.ch](https://alexi.ch/)` line as the global
  footer. Duplication is intentional.
- **Footer anchor attributes restored to `target="_blank" rel="noopener noreferrer"`.**
  *Rationale:* developer choice. The developer's HTML snippet had
  `target="blank"` (typo — would open in a window named `"blank"`
  instead of a new tab) and omitted `rel="noopener noreferrer"`
  (drops the `window.opener` tabnabbing guard). Developer
  confirmed both were slips and asked to restore. Matches the
  existing `HomeView.vue:281-283` row-link pattern.

## Testing strategy

No automated tests. Manual smoke checks only (per `AGENTS.md`).
Developer runs `make dev` (Vite + Go server), walks the scenarios in
Step 1's acceptance criteria.

## Steps

### Step 1 — Footer, About page, route, auth-watcher adjustment, docs

- **Status:** done (reviewed 2026-08-23)
- **What:**
  - Create `web/src/components/AppFooter.vue`:
    - Renders a `<footer>` element with class `app-footer`.
    - Two pieces of content:
      - Text `© 2026 ` followed by an `<a href="https://alexi.ch/" target="_blank" rel="noopener noreferrer">alexi.ch</a>`.
      - A separate line or spacer-separated link `About` pointing to `/about`. Use `<router-link>` so it does client-side navigation.
    - `data-test` hooks:
      - `app-footer` on the root.
      - `app-footer-copyright` on the copyright line.
      - `app-footer-alexi` on the alexi.ch anchor.
      - `app-footer-about-link` on the About router-link.
    - Scoped styles: small grey text (`color: #6b7280`),
      `font-size: 0.875rem`, top border
      `border-top: 1px solid #e5e7eb; margin-top: 2rem;`,
      centered max-width (`max-width: 720px; margin-left: auto; margin-right: auto;`),
      `padding: 1rem;` so the border feels like a divider not a wall.
      Content on a single line, gap-separated:
      `display: flex; justify-content: center; align-items: center; gap: 0.75rem; flex-wrap: wrap;`.
      The link `color: #2563eb; text-decoration: underline;` (matches the row URL colour in `HomeView`).
  - Modify `web/src/App.vue`:
    - Import and render `<AppFooter />` after `<RouterView v-else />`
      so all three routes get it once the auth state settles.
    - Adjust the existing auth `watch` so the
      `state === 'logged-out'` branch only redirects when
      `p !== '/login' && p !== '/about'`. The `state === 'logged-in'
      && p === '/login'` branch stays as is (it only fires on
      `/login` so `/about` is unaffected).
  - Create `web/src/views/AboutView.vue`:
    - A `<main class="about">` block, centered max-width block
      matching `LoginView` (`max-width: 32rem; margin: 2rem auto; padding: 1rem;`).
    - `<h1 class="about-title">About Linkomat</h1>`.
    - A short body (2–4 sentences) describing the app. **Draft at
      implementation time and present in the review message.** Topic
      coverage: local-first / offline-first bookmark+note store,
      PWA for mobile, single operator, syncs to a server you run.
      Do not include marketing language or sales pitch.
    - Scoped styles: minimal — title margin-bottom 1rem, paragraph
      spacing, link colours reused.
    - `data-test` hooks: `about-title`, `about-body`.
  - Modify `web/src/router.ts`:
    - Import `AboutView`.
    - Add `{ path: '/about', component: AboutView }` to the routes
      list.
  - Modify `doc/architecture/web.md`:
    - In the top-level layout listing, extend the `views/` bullet
      list to include `AboutView.vue`.
    - In the route mention (`router.ts — 3 routes: /login, /, /entries/:id`),
      bump to 4 routes and list `/about`.
    - In the `App.vue` paragraph, add one sentence noting that
      `AppFooter` is rendered below `<RouterView/>` on every route.
    - No new sections. High-level only.
- **Acceptance criteria:**
  - Visit `/login` in the browser after `make dev`: footer with
    `© 2026 alexi.ch` visible below the form. `alexi.ch` is a link.
    Clicking it opens `https://alexi.ch/` in a new tab.
  - The About link is visible in the footer on `/login`. Clicking it
    does client-side navigation to `/about`. The login form is
    replaced by the About page (no full reload).
  - On `/about` while logged out: page shows the heading
    "About Linkomat" and the 2–4 sentence body. The auth watcher
    does **not** redirect to `/login`.
  - Log in: footer still visible on `/`. Footer still visible on
    `/entries/:id` after clicking a row. About link still works
    from those pages.
  - While the loading splash is briefly visible, the footer is not
    yet rendered (splash replaces the whole view via the `v-if`
    branch).
  - `doc/architecture/web.md` mentions the new view and the new
    footer in the affected bullets. No new section.
  - Typecheck: `cd web && npx vue-tsc --noEmit` passes (or the
    project's documented typecheck command — discover before
    running). If no explicit typecheck command is wired up, just
    read the modified `.vue` files for obvious type errors.
- **Notes:**
  - Implemented as planned. All four files created/modified:
    - `web/src/components/AppFooter.vue` — new
    - `web/src/views/AboutView.vue` — new
    - `web/src/App.vue` — `AppFooter` rendered after `<RouterView/>` in the
      `v-else` branch; auth watcher redirect predicate changed to
      `p !== '/login' && p !== '/about'`.
    - `web/src/router.ts` — added `/about` route.
    - `doc/architecture/web.md` — folder layout bullet extended; route
      count bumped from 3 to 4.
  - Typecheck: `cd web && ./node_modules/.bin/vue-tsc --noEmit` → passes.
  - Three deviations, all raised and decided by the developer during
    review:
    - **About body shortened.** "...sync in the background to **the
      server**." (developer) rather than "...to **a small Go server you
      run yourself**." (agent draft). Three sentences unchanged
      otherwise.
    - **About page duplicates the footer copyright line.**
      `web/src/views/AboutView.vue` has a second
      `<p class="about-copy">` below the body with the same
      `© 2026 [alexi.ch](https://alexi.ch/)` content as the
      global footer. Intentional — developer confirmed in review
      that the duplication is wanted. `data-test="about-copy"` /
      `data-test="about-link"` hooks added.
    - **Footer anchor kept as-is.** Developer's HTML snippet had
      `target="blank"` (likely typo) and dropped
      `rel="noopener noreferrer"`; developer confirmed both were
      slips and chose to restore to `target="_blank"
      rel="noopener noreferrer"`.

## Open questions

- None. About copy was drafted by the agent and revised by the
  developer during review. All other decisions resolved.
