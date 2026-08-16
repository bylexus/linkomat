# Linkomat — Architecture

System-level overview. A reading agent should be able to orient itself in
under five minutes and then dive into the component doc that matches
the part it needs to touch.

## What it is

Linkomat is a self-hosted, single-operator bookmark/note store with a
web PWA and a CLI. One Go server owns persistence; everything else is a
client. v1 is a single-user-style MVP, but the data model is
user-scoped from day one.

## Components

| Part    | Path        | Role                                                                  | Doc                               |
|---------|-------------|-----------------------------------------------------------------------|-----------------------------------|
| Server  | `api/`      | Go HTTP server. Meta DB, per-user SQLite, auth, sync API, embedded PWA. | [api.md](api.md)                  |
| Web     | `web/`      | Vue 3 + TS PWA. Local-first, offline-capable, syncs with the server.   | [web.md](web.md)                  |
| CLI     | `cli/`      | Go CLI. Online-only client over the same API.                          | [cli.md](cli.md)                  |
| Shared  | `shared/`   | Go DTOs and constants shared between server and CLI.                   | (see `shared/*.go`)               |
| Docs    | `doc/`      | `PLANNING.md` (design source of truth), `architecture/` (this tree), `bruno-api-tests/` (manual API test collection). | — |

A root `go.work` wires `api`, `cli`, and `shared`. A `Makefile` at the
repo root drives builds. Details in [build.md](build.md).

## How the parts fit

```
                       ┌────────────────────────────┐
                       │        web (PWA)           │
                       │ Vue 3 + sql.js (OPFS)      │
                       │  - local DB first          │
                       │  - sync engine main thread │
                       └─────────────┬──────────────┘
                                     │ HTTPS (cookies)
                                     ▼
┌─────────────────┐         ┌────────────────────────────┐
│      cli        │  HTTPS  │           api              │
│  Go + cobra     │ Bearer  │  Go net/http + GORM        │
│  (online only)  │────────▶│  - meta DB (users/tokens)  │
└─────────────────┘         │  - per-user SQLite cache   │
                            │  - embeds web/ as go:embed │
                            └────────────────────────────┘
                                     │
                            ┌────────┴────────┐
                            │   data/         │
                            │  ├── meta.db    │
                            │  └── users/     │
                            │       └── *.db  │
                            └─────────────────┘
```

Two HTTP clients, one API:

- **Web PWA** — uses the `linkomat_auth` (access) and
  `linkomat_refresh` (refresh) cookies. JS never reads them. On 401
  the API client (`web/src/api/client.ts`) attempts one
  `POST /auth/refresh`, retries the original request, and on failure
  routes the user back to `/login`.
- **CLI** — uses `Authorization: Bearer <access_token>` for non-auth
  routes and sends the refresh token in the JSON body of
  `/auth/refresh`. Tokens persist in
  `~/.config/linkomat/credentials.json` (mode 0600).

The server is the **only** component that touches the database. It
runs plain HTTP and is expected to live behind a TLS-terminating
reverse proxy (Caddy, nginx, Traefik, …). See
`PLANNING.md` §5.2 for `LINKOMAT_BEHIND_PROXY` and the cookie `Secure`
flag.

## Data flow at a glance

- **Writes (web)** happen against the local sql.js DB first, mark
  `local_dirty_at`, and get pushed FIFO by the sync engine.
- **Writes (CLI)** go straight to the server (`PUT /entries/:id`).
- **Reads (web)** are always against the local DB. The sync engine
  pulls server deltas via `GET /entries?changed_since=<cursor>` and
  applies them locally.
- **Auth** is opaque 32-byte hex tokens; sessions are rows in the
  meta DB's `tokens` table (no separate `sessions` table). Refresh
  rotates on every `/auth/refresh` call.

## Where to look next

- Editing server code? Read [api.md](api.md).
- Editing the web app? Read [web.md](web.md).
- Editing the CLI? Read [cli.md](cli.md) — placeholder for now, CLI
  is the last item in `PLANNING.md` §10.
- Need build / test commands? Read [build.md](build.md).
- Need design decisions? Read `../PLANNING.md`.
