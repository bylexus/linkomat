# Linkomat — v1 Development Plan

This document captures the agreed design for v1 of Linkomat, derived from
`README.md` and a one-question-at-a-time design review. v1 is a
single-user-style MVP, but the architecture is user-scoped from day one.

## 0. Resume Instructions

For a fresh agent session continuing work on this project:

1. **Read in this order**: `README.md` → `AGENTS.md` (root) → this file
   (`doc/PLANNING.md`) → if you need component-level context, the
   relevant doc in `doc/architecture/`.
2. **Source of truth**: this plan overrides the README where they
   disagree. If you find a discrepancy during implementation, update
   this file and call it out in the commit message.
3. **Scope guardrails**: do not introduce features listed in §11
   (Deferred) without an explicit ask. Do not add automated tests or
   CI in v1 (§1).
4. **Working agreement with the user**: address them as
   “Mr. Anderson”. Do not summarize at the end of a task — just stop.
   Do not add code comments unless asked.
5. **Pinned decisions** (do not renegotiate without prompting):
   - No self-signup; users created via server CLI only.
   - Tokens-as-sessions model; multiple sessions per user coexist as
     multiple `tokens` rows. (See `architecture/api.md` — Auth.)
   - Refresh-token rotation on every `/auth/refresh`.
   - Server-authoritative `updated_at`; client tracks
     `local_dirty_at` + `base_updated_at`.
   - Last-save-wins by client intent, no merge UI.
   - sql.js + OPFS for web local DB (no IDB fallback in v1).
   - sql.js WASM is the **browser** build (`sql-wasm-browser.wasm`),
     NOT `sql-wasm.wasm` (the Node build). Self-hosted at
     `/sqlite/`. The sql.js loader calls `locateFile` with that
     exact name.
   - The web SW does **not** precache anything under `/sqlite/*`
     (`globPatterns` excludes `wasm`; `navigateFallbackDenylist`
     blocks `/^\/sqlite\//`). sql.js's `locateFile` URL carries a
     `?v=${SQLJS_ASSET_VERSION}` cache-buster so even a stale SW
     cannot match a precache entry against it.
   - Web CSP `script-src` must include `'wasm-unsafe-eval'`
     (required by `WebAssembly.instantiate` for sql.js).
   - No Background Sync API usage.
   - Light mode only.
   - Embedded web assets in the server binary.
   - Node.js ≥ 24 is required for the web build (Vite 8 / vite-plugin-pwa 1
     require it; `npm install` and `npm run build` both emit engine warnings on
     older Node). The `web/` folder ships a `.nvmrc` pinned to `24`. Use a
     system-wide Node ≥ 24 if available; otherwise `cd web && nvm use` before
     any direct `npm` call. The Makefile already sources `$(HOME)/.nvm/nvm.sh`
     and runs `nvm use` for `make web-build` / `make dev`, so those targets work
     regardless of the system Node version.
6. **Placeholders to confirm before module init**:
   - Go module path: `github.com/bylexus/linkomat/{api,cli,shared}`
     (confirmed in step 1; existing `go.mod` files set this).
   - Default server URL in CLI: `https://app.linkomat.example.com`
     — update to the operator’s domain before any release.
   - Default PWA theme/background colors: confirmed in step 1 —
     `theme_color=#2563eb`, `background_color=#ffffff` (light only).
7. **Build order**: follow §10 strictly. Each step should end with a
   manual smoke check; no automated tests exist yet.

**Architecture reference**: `doc/architecture/` documents the system
that has already been built — the global map (`overview.md`), the Go
server (`api.md`), the Vue 3 PWA (`web.md`), the CLI (`cli.md`,
placeholder), and build / run / test (`build.md`). For component-level
questions (request lifecycle, local DB schema, sync engine state
machine, env vars, Makefile targets) read those docs rather than the
old sections here — they are kept in sync with the code.

## 1. Scope

- Single deployable server + embedded web PWA + CLI client.
- Multiple provisioned users supported, but no public sign-up.
- No multi-device awareness, no merge UI, no admin web UI.
- No CI, no automated tests; manual testing only in v1.
- v2 candidates (deferred): offline CLI, rich text, dark mode, multi-device
  locking, bulk sync endpoint.

## 2. Components

See `architecture/overview.md` for the global map and per-component
docs (`api.md`, `web.md`, `cli.md`, `build.md`) for what each part
does and how they connect.


## 10. Build Checklist

Steps 1–10 done. **Next: step 11 — CLI** (`login`, `logout`, `add`,
`ls`, `search`, `get`, `rm`, `edit` with `fsnotify`). Final e2e
pass is step 12. Build/test commands, env vars, and deploy notes
live in `architecture/build.md` — read that for the actual
commands. The CLI is intentionally one of the last steps so the
API surface stays stable while the web app — the primary v1
client — drives the design first.

1. Repo scaffold: `go.work`, `Makefile`, module skeletons for `api`,
   `cli`, `shared`, Vite scaffold for `web`.
2. `shared/` DTOs: `Entry`, `AuthResponse`, `ErrorResponse`, constants
   for cookie names, schema version constant, `currentSchemaVersion`.
3. Server: meta DB, user CRUD CLI, password hashing, `/auth/login`,
   `/auth/refresh`, `/auth/logout`, rate limit.
4. Server: per-user DB cache, `GET/PUT/DELETE /entries`, soft-delete,
   `AutoMigrate` + `schema_version`.
5. Server: embedded web assets, `/healthz`, `/version`, security
   headers, request logging.
6. Web: PWA scaffold, sql.js wrapper, OPFS persistence, integrity
   check. **Done**
7. Web: home view + virtualized list + search. **Done**
8. Web: note editing view + save-on-blur + delete. **Done**
9. Web: sync engine (dirty queue, delta pull, full pull on first
   launch, retry/backoff, status UI). Split into 9.1 auth + login view
   (**done**), 9.2 sync engine + cursor + reachability (**done**),
   9.3 status UI + backoff + retry (**done**).
10. End-to-end manual pass on mobile (PWA install). **Done**
11. CLI: `login`, `logout`, `add`, `ls`, `search`, `get`, `rm`,
    `edit` (with `fsnotify`).
12. Final end-to-end pass covering both PWA and CLI.

## 11. Deferred (v2+)

- Offline CLI.
- Bulk sync endpoint.
- Multi-device awareness and per-device token lock.
- Self-sign-up flow.
- Rich text editing / preview.
- Dark mode.
- GitHub Actions CI + automated tests.

## 12. Progress Log

Session-by-session state. Read this section first when resuming.
Step implementations live in the code; the docs in
`doc/architecture/` describe the current shape of the system. This
section records session intent + commit hash + landed files, and
keeps the items the next session needs cold.

### Session index

| #  | Step          | Commit                                  | Title                                                              |
|----|---------------|-----------------------------------------|--------------------------------------------------------------------|
| 1  | §10 step 1    | `1cab067`                               | repo scaffold (`go.work`, Makefile, Vite 8 + Vue 3 + PWA)         |
| 2  | §10 step 2    | `c51fa80`                               | `shared/` DTOs (Entry, AuthResponse, ErrorResponse, cookies)       |
| 3  | §10 step 3    | `2acc1c7`                               | server auth + meta DB + user CLI + Bruno collection                 |
| 4  | (small change)| `01d74d3`                               | drop server-side TLS termination; rely on TLS proxy                |
| 5  | §10 step 4    | `0c5bd1f`                               | per-user DB cache + entries endpoints                              |
| 6  | plan          | `23af127`                               | reorder build steps; defer CLI to step 11                          |
| 7  | §10 step 5    | `bda620f`                               | embedded web assets + /healthz + /version + security headers       |
| 8  | (cleanup)     | `c61c83b`                               | fold Cache-Control into apiHeaders middleware                      |
| 9  | §10 step 6    | `d67d6e2`                               | web PWA scaffold + sql.js + OPFS + integrity check                 |
| 10 | §10 step 7    | `6320abf`                               | web home view + virtualized list + FTS3 search + dev global        |
| 11 | §10 step 8    | `bb64b7a`                               | web EditView + save-on-blur + delete + URL link                   |
| 12 | §10 step 9.1  | `29059b8`                               | API client + auth flow + Login view + per-user OPFS                 |
| 13 | §10 step 9.2  | `b4faa16`                               | sync engine: cursor + dirty queue + reachability + push-on-save    |
| 14 | §10 step 9.3  | `29a032c`                               | sync status UI + backoff + retry button                            |
| 15 | (docs)        | `56c6f84`                               | split docs: `doc/architecture/` for system, `PLANNING.md` kept for handoff |
| 16 | (ops)         | `040f86e`                               | docker-compose env for SSL termination (self-signed, local dev)    |
| 17 | §10 step 10   | `bb2ed8b`                               | mobile PWA install on iOS Safari + Android Chrome (kube002 deploy) |

Key landed files by step (also reachable from `git show <commit>`):

- 9 — `web/src/db/{schema.ts,opfs.ts,sqlite.ts}`, `web/src/App.vue`,
  `web/scripts/copy-sqljs-wasm.mjs`, `web/public/sqlite/`
  (gitignored).
- 10 — `web/src/views/HomeView.vue`, `web/src/router.ts`,
  `web/src/db/index.ts` (FTS3 + `entries_fts` triggers in `schema.ts`).
- 11 — `web/src/views/EditView.vue`, `web/src/utils/{uuid.ts,url.ts}`.
- 12 — `web/src/api/{client.ts,auth.ts}`, `web/src/state/auth.ts`,
  `web/src/views/LoginView.vue`.
- 13 — `web/src/api/entries.ts`, `web/src/state/sync.ts`,
  `web/src/sync/{reachability.ts,engine.ts}`, `sync_state` table
  (schema v3). Server: `shared/entry.go`, `api/handlers_entries.go`.
- 14 — `web/src/views/HomeView.vue` (status chip + row dots),
  `web/src/views/EditView.vue` (per-entry dot + echo), backoff in
  `web/src/sync/engine.ts`.
- 15 — `doc/architecture/{overview,api,web,cli,build}.md` lifted out
  of `PLANNING.md`; `PLANNING.md` kept as the single design / progress
  handoff document.
- 16 — `docker-compose.yml` + `.env.example` for local dev with a
  self-signed TLS-terminating Caddy on `:443`; `LINKOMAT_BEHIND_PROXY=1`.
- 17 — `doc/kube002.alexi.ch/linkomat/{00..90-*.yaml, README.md}`
  (namespace, PV, PVC, env Secret, Deployment, Service, HTTPRoutes,
  Certificate, Gateway listener patch via Server-Side Apply) +
  `Makefile` `image-build` / `image-push` (multi-platform
  linux/amd64+arm64 via `docker buildx`, push to
  `registry.alexi.ch/linkomat`).

### Key traps worth remembering

- **sql.js rejects `undefined` bind parameters** (only `null` maps to
  SQL NULL). A Go DTO with `json:"…,omitempty"` on a nil pointer omits
  the key, which arrives as `undefined` and throws
  `Wrong API use : tried to bind a value of an unknown type`. The
  `deleted_at` wire contract is fixed on both sides (server drops
  `omitempty`; client binds `entry.deleted_at ?? null`).
- **Do not expose a Vue state snapshot for debugging.** The dev global
  must publish the reactive `Ref` itself, not a one-shot value —
  otherwise console reads are frozen at login time and misleading.
- **`<sqlite>` rebuild needs `INSERT OR UPDATE` sql.js helper, not
  `INSERT OR REPLACE`.** `REPLACE` does not fire the `AFTER DELETE`
  trigger (recursive triggers off) and reallocates the rowid,
  orphaning `entries_fts` docids on every sync.
- **sql.js WASM file is `sql-wasm-browser.wasm`, not
  `sql-wasm.wasm`.** The Node build ships in the npm artefact and
  causes a 404 → SPA fallback → HTML parsed as WASM → magic byte
  mismatch failure that looks like a SW problem.
- **Backoff + 2 s push debounce + 5 min interval must share the same
  `syncInFlight` guard.** Backoff is a single `setTimeout` keyed by
  `backoffTimer`; the debounce skips scheduling while a cycle is
  in-flight; the interval re-arms only on a successful cycle that
  still has dirty rows. Adding a second retry timer breaks the loop
  suppression.
- **Gateway API rejects `strategic-merge-patch`.** `kubectl patch
  --type=strategic` on a `Gateway`/`HTTPRoute` errors with
  `accepted media types: json-patch, merge-patch, apply-patch-yaml`.
  Adding a listener to an existing Gateway uses Server-Side Apply
  (`kubectl apply --server-side --field-manager=<name> -f partial.yaml`)
  and only declares the new listener — existing listeners stay under
  their original field owner.
- **SSA migration warning on partial Gateway spec.** When the Gateway
  was first applied via client-side apply, the first SSA patch with a
  partial spec logs `failed to re-apply configuration after performing
  Server-Side Apply migration … spec.gatewayClassName: Required value`.
  It's non-fatal (the listener is still added), but to silence it,
  declare the full Gateway spec in the SSA file.
- **Distroless container UID must own the PV path.** The Go image
  runs as `nonroot` (UID 65532). A static local-path PV mounted at
  `/app/data` must be created with
  `sudo install -d -o 65532 -g 65532 -m 0750 /data/k3s-storage/<name>`
  on the node, else SQLite can't open the DB and the pod
  `CrashLoopBackOff`s with `unable to open database file`.
- **`kubectl exec` defaults to `--stdin=false`.** `printf 'pw\npw\n'
  | kubectl exec deploy/X -- cmd` without `-i` ignores the pipe; the
  container's `promptPassword` reads EOF and returns empty. Always
  add `-i` (or `-it` for TTY) when piping into the distroless binary.

### Step 10 — Mobile PWA install (done)

**Deployment:** `https://linkomat.alexi.ch` on `kube002.alexi.ch`
(`doc/kube002.alexi.ch/linkomat/`). Shared Envoy Gateway
`kube002-gateway`, cert-manager + Hetzner DNS-01
`lets-encrypt-alexi-ch`, local-path PV on `kube002`, private registry
`registry.alexi.ch/linkomat:latest` with `registry-alexich-cred` pull
secret. Image built via `make image-push` (multi-platform
linux/amd64+linux/arm64, buildx docker-container driver).

**Smoke checks performed:** all green.

- `https://linkomat.alexi.ch/healthz` → HTTP 200, 71 B.
- `https://linkomat.alexi.ch/version` → JSON `0.1.0`.
- `http://linkomat.alexi.ch/healthz` → HTTP 301 → `https://…`.
- `https://linkomat.alexi.ch/login` → HTTP 200, `text/html`, SPA fallback.
- PWA installs on iOS 17+ Safari and Android Chrome 120+.
- Offline create → push round-trip on both platforms.
- Sync chip behaves: `Offline` (gray) → `pending: N` → reconnect →
  `Syncing…` → `Online`, count drops to 0.

**Known gaps carried forward (v2 candidates, still v1-acceptable):**

- `/settings` route still unimplemented (HomeView's temporary logout
  button is fine).
- Direct browser navigation to `/entries/:id` returns 401 on a hard
  reload (mux precedence). SPA pushState navigation works; mobile
  reloads aren't part of the test pass. Proper fix would route on
  `Accept: text/html`.
- The `Secure` cookie flag is on by default —
  `LINKOMAT_COOKIE_SECURE=false` is only for local HTTP dev.

### Step 11 — CLI (next)

**Scope (§10 step 11):** implement `cli/linkomat` (Go, cobra-style —
the existing scaffold under `cli/` is empty placeholder). Commands:

- `login` — prompt for server URL + username + password, store
  access/refresh tokens locally (file mode 0600).
- `logout` — invalidate refresh token on server, delete local
  credentials.
- `add` — interactive entry create; reads title/url/note/tags,
  POSTs to `/entries`, prints the new id.
- `ls` — list entries (`?limit=&offset=`), paginated, mirrors the
  PWA home view shape.
- `search` — FTS-style query against title/url/note/tags.
- `get <id>` — print one entry as JSON.
- `rm <id>` — DELETE (soft-delete on the server).
- `edit <id>` — open the entry in `$EDITOR` (default `vi`); on save
  PUT to server. Use `fsnotify` on the file to push the change
  immediately after the editor exits cleanly. Mirror the PWA's
  `local_dirty_at` + `base_updated_at` model: only push on actual
  change; if the server's `updated_at` moved while editing, last-write
  wins (no merge UI).

**Prerequisites:**

- Reuse `shared/` DTOs and `shared/cookies.go` constants. Do **not**
  duplicate the entry shape.
- CLI talks the same HTTPS API the web app does; share auth/refresh
  semantics with `web/src/api/client.ts` (or duplicate and document
  the divergence).
- Token storage: a single config dir (`~/.config/linkomat/`), mode
  0700; `credentials.json` mode 0600. Server URL is per-install.
- No Background Sync API usage (deferred per §11).
- No offline mode (deferred per §11) — surface a clear error if
  `/healthz` is unreachable and abort.

**Acceptance for step 11:**

- `login` against the live `linkomat.alexi.ch` round-trips a refresh.
- `add` → entry shows up in the PWA home view after a pull.
- `ls` / `search` / `get` / `rm` behave as the equivalent HTTP calls.
- `edit` updates the server after the editor exits and the file
  actually changed; no change → no PUT.

**Step 12 (final e2e) follows step 11:** exercise both PWA and CLI
against the same account on the live cluster; verify the two clients
see each other's writes through the sync pull.