# AGENTS.md

Quick map for a fresh reading LLM agent. **Read this file first.**

## Read first

1. `README.md` — what Linkomat is, in a paragraph.
2. `doc/architecture/overview.md` — the system map; everything below
   hangs off it. Pick the per-component doc for the part you're
   about to touch:
   - `doc/architecture/api.md` — Go server
   - `doc/architecture/web.md` — Vue 3 PWA
   - `doc/architecture/cli.md` — Go CLI (placeholder)
   - `doc/architecture/build.md` — build, run, test
3. If an in-flight feature is being worked on, its plan lives at
   `docs/agent-plans/<feature>.md`. Architecture is the source of
   truth for the system; the plan file is the source of truth for
   the current work.

## Conventions for reading the code

- **Source of truth on conflict**: this file + `doc/architecture/*.md`
  outrank the README. If the code disagrees with the docs, the docs
  were last updated by the most recent session — check `git log` and
  the in-flight `docs/agent-plans/<feature>.md` to see whether the
  deviation is recorded.
- **No code comments unless asked.** Don't add them when editing.
- **No automated tests / CI in v1.** Smoke checks are manual; see
  the "Acceptance criteria" / "Manual test instructions" sections
  of the relevant feature plan under `docs/agent-plans/` for what
  to re-run.
- **No summary at the end of a task.** Stop when the task is done.
- **Web PWA specific**:
  - `CURRENT_SCHEMA_VERSION` is the migration boundary — bump it
    in `web/src/db/schema.ts` when adding columns or tables.
    `LocalDb.open()` will sidecar the old file and recreate.
  - `DB_CHANGE_EVENT` (`'linkomat-db-change'`) is the cross-view
    notification; `HomeView` and `EditView` listen to it.
  - FTS3 (not FTS5) is what's available in `sql.js@1.14.2`'s
    WASM. Triggers strip JSON syntax from `tags` before indexing.
- **Server specific**:
  - Per-user DB cache (`api/userdb.go`) has no eviction in v1;
    don't add one without a reason.
  - `tags` is JSON-in-TEXT (not a normalised table).
  - `authenticate` writes its 401/403/500 directly and returns
    `nil` — the mux never runs. New routes that need auth must
    match the `/entries*` prefix in `needsAuth`, or
    `authenticate` will let them through.
  - All `/api/*` routes go through `apiHeaders` middleware
    (sets `Cache-Control: no-store, private`). Web assets do
    **not** — workbox precache needs HTTP caching.

## Where to make changes

| Want to change…             | File(s)                                                                       |
|-----------------------------|-------------------------------------------------------------------------------|
| API endpoint / response     | `api/handlers_*.go`, add a DTO to `shared/`, route in `api/server.go`         |
| Auth behaviour              | `api/auth.go`, `api/handlers_auth.go`, `api/middleware_auth.go`               |
| Local DB schema             | `web/src/db/schema.ts` (and bump `CURRENT_SCHEMA_VERSION`)                    |
| Sync engine                 | `web/src/sync/engine.ts`, `web/src/state/sync.ts`                             |
| Sync reachability           | `web/src/sync/reachability.ts`                                                |
| API client / 401 refresh    | `web/src/api/client.ts`                                                       |
| PWA / service worker config | `web/vite.config.ts`                                                          |
| Cookie / DTO constant       | `shared/cookies.go`, `shared/auth.go`, `shared/entry.go`, `shared/entries.go` |
| Build targets               | `Makefile`                                                                    |
| Plan / design               | `docs/agent-plans/<feature>.md` for the in-flight feature                    |
| Architecture docs           | `doc/architecture/*.md`                                                       |
