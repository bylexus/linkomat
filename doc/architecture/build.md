# Build, test, and run

## Quick reference

| Target                              | What it does                                                              |
|-------------------------------------|---------------------------------------------------------------------------|
| `make build-server`                 | Build `bin/linkomat-server`. Depends on `web-embed` (Vite build + copy).  |
| `make build-server-expose-dev`      | Same as above but Vite `--mode expose-dev` — `window.__linkomatDb` + `window.__linkomatSync` survive in the embedded bundle for manual DevTools seeding. |
| `make build-cli`                    | Build `bin/linkomat`.                                                    |
| `make web-build`                    | `npm install` + `npm run build` in `web/`.                               |
| `make web-embed`                    | `web-build` + copy `web/dist/*` into `api/web_dist/` for `go:embed`.     |
| `make dev`                          | Run the Vite dev server (`http://localhost:5173`) and `go run ./api` (`:8080`) in parallel. |
| `make tidy`                         | `go mod tidy` across `api`, `cli`, `shared`.                             |
| `make all`                          | `build-server` + `build-cli`.                                            |
| `make api-test`                     | Run the Bruno collection (`doc/bruno-api-tests/`) against a running server. |
| `make api-test-auth`                | Run only the `auth/` folder.                                             |
| `make api-test-req REQ=auth/login`  | Run a single request by name (path is `<folder>/<name>`).                 |
| `make api-test-reset`               | Reset captured token values in `Local.bru` (Bruno CLI 4.x writes runtime vars back to the env file). Run before committing. |
| `make clean`                        | Remove `bin/`, `web/dist/`, `web/node_modules/`, `api/web_dist/`.        |

## Node version

The web build is pinned to Node 24 (`web/.nvmrc`). `make web-build`,
`make web-embed`, and `make dev` source `$(HOME)/.nvm/nvm.sh` and
run `nvm use` so they work regardless of the system Node version.
Direct `npm` calls in `web/` need `nvm use` first if the system Node
is older.

## Server configuration

All via env vars, loaded by `api/config.go`:

| Var                              | Default                                    | Meaning                                            |
|----------------------------------|--------------------------------------------|----------------------------------------------------|
| `LINKOMAT_LISTEN`                | `:8080`                                    | `http.Server.Addr`.                                |
| `LINKOMAT_DATA_DIR`              | `./data`                                   | Created with mode 0700. Meta DB + `users/` live here. |
| `LINKOMAT_COOKIE_SECURE`         | `true`                                     | Sets the `Secure` cookie flag.                     |
| `LINKOMAT_BEHIND_PROXY`          | `false`                                    | Trust `X-Forwarded-For` for client IP.             |
| `LINKOMAT_CORS_ORIGINS`          | (empty)                                    | CSV. Empty = same-origin only.                     |
| `LINKOMAT_ACCESS_TTL_SECONDS`    | `900` (15 min)                             | Access token lifetime.                             |
| `LINKOMAT_REFRESH_TTL_SECONDS`   | `2592000` (30 d)                           | Refresh token lifetime.                            |

The server is plain HTTP. Put it behind a TLS-terminating reverse
proxy.

## Server user management

```sh
# start
linkomat-server serve

# create / manage users
linkomat-server user create <username>          # prompts password
linkomat-server user set-password <username>    # prompts new password
linkomat-server user list
linkomat-server user activate <username>
linkomat-server user deactivate <username>      # revokes all tokens
linkomat-server user delete <username>          # drops the per-user DB file
linkomat-server user purge                      # v1: manual cleanup of expired tokens

linkomat-server version
```

## Manual API testing (Bruno)

`doc/bruno-api-tests/` is a Bruno collection covering auth, entries,
and meta endpoints. Run it against a live server:

```sh
# in one terminal
make build-server && ./bin/linkomat-server serve

# in another
make api-test        # full collection
make api-test-auth   # just the auth folder
make api-test-req REQ=entries/entries-create
```

Override target / creds with `BRUNO_BASEURL`, `BRUNO_USER`,
`BRUNO_PASS`, `BRUNO_ENV` env vars.

## Dev mode

```sh
make dev
```

Runs Vite (`:5173`) and `go run ./api` (`:8080`) in parallel. The
Vite dev server proxies nothing — the web app points directly at
`:8080` for `/api/*`, `/auth/*`, `/healthz`, `/sqlite/*`, and the
service-worker-pre-cached assets are served by `:8080` too. Vite's
HMR is for the UI; OPFS state lives in the browser, so reloads in
Vite keep the local DB.

## Production / release

```sh
make all
```

`bin/linkomat-server` is a self-contained binary — web PWA is
embedded via `go:embed`. Run it behind a reverse proxy that
terminates TLS and forwards to `LINKOMAT_LISTEN`. Set
`LINKOMAT_BEHIND_PROXY=true` so the server trusts
`X-Forwarded-For`. Keep `LINKOMAT_COOKIE_SECURE=true` (default) so
the cookies' `Secure` flag is set even though the server-to-proxy
hop is plain HTTP.
