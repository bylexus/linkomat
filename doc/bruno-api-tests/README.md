# linkomat-api — Bruno Collection

Manual smoke tests for the Linkomat server API. The collection targets
the local server and assumes the default `:8080` listen address.

## Prerequisites

- A running Linkomat server reachable at the URL in the
  `Local` environment's `baseUrl` (default `http://localhost:8080`).
- A provisioned user with the username / password from the
  `Local` environment (default `alice` / `hunter2`).
- For `login-deactivated.bru`: a second user matching
  `deactivatableUsername` that has been deactivated with
  `linkomat-server user deactivate <name>`.

The server must be started with `LINKOMAT_COOKIE_SECURE=false` so
cookies work over plain HTTP (curl from localhost can't follow the
`Secure` flag). In a real deployment the server runs behind a
TLS-terminating proxy and the default `LINKOMAT_COOKIE_SECURE=true`
is the correct setting.

```
make build-server
LINKOMAT_COOKIE_SECURE=false LINKOMAT_LISTEN=:8080 ./bin/linkomat-server serve
```

```
printf 'hunter2\nhunter2\n' | LINKOMAT_COOKIE_SECURE=false ./bin/linkomat-server user create alice
```

## Import into Bruno

1. Open the Bruno desktop app.
2. **Collection → Open Collection…** (or `⌘O`).
3. Select the `doc/bruno-api-tests/` directory.
4. The collection appears as **linkomat-api** with two folders
   (**auth**, **entries**) and the **Local** environment under the
   environments dropdown.

## Folders

### `auth/` — covers §10 step 3

Login / refresh / logout (cookies + JSON body). See **Run order** below.

### `entries/` — covers §10 step 4

All `/entries` endpoints. Bruno CLI's cookie jar is per-folder, so the
entries tests use `Authorization: Bearer {{lastAccessToken}}` (the
CLI flow). The cookie auth path is exercised by `auth/refresh-cookie`
— the middleware handles Bearer and cookie the same way, so the
entries endpoints work via either method.

The `cached` Bruno CLI features we depend on:

- `bru.setEnvVar("lastAccessToken", ...)` in `auth/login.bru`
  populates the env var that the entries tests reference.
- `Authorization: Bearer {{lastAccessToken}}` in headers is
  interpolated by the CLI at send time.

Numbered with `seq` inside the folder. `entries/entries-create`
captures `entryId` for the downstream tests.

| # | Request                       | Auth                          | Notes                            |
|---|-------------------------------|-------------------------------|----------------------------------|
| 1 | entries-no-auth               | none                          | 401 unauthorized                 |
| 2 | entries-bad-token             | Bearer (bogus)                | 401 invalid-token                |
| 3 | entries-list                  | Bearer (lastAccessToken)      | 200, list                        |
| 4 | entries-create                | Bearer                        | PUT, captures entryId            |
| 5 | entries-get                   | Bearer                        | GET single                       |
| 6 | entries-update                | Bearer                        | PUT again, updated_at advances   |
| 7 | entries-changed-since         | Bearer                        | cursor query                     |
| 8 | entries-delete                | Bearer                        | 204 + soft-deletes               |
| 9 | entries-deleted-get           | Bearer                        | 200 with deleted_at set          |
| 10 | entries-not-found             | Bearer                        | 404 not-found                    |

The cookie auth path itself is tested by `auth/refresh-cookie`. The
PWA only sends cookies per §7.4, so the smoke check there happens in
step 6+ where the web app exercises the endpoints.

## Run order

The auth requests are numbered with `seq` and ordered to share state.

| # | Request                    | Depends on              | Notes                            |
|---|----------------------------|-------------------------|----------------------------------|
| 1 | Login (success)            | provisioned user        | Captures lastAccessToken / lastRefreshToken |
| 2 | Login (bad password)       | provisioned user        | Counts against rate-limit bucket |
| 3 | Login (deactivated user)   | deactivatable user off  | Requires `user deactivate` first |
| 4 | Login (rate limited)       | 5+ failed attempts/min  | Stateful, see request docs       |
| 5 | Refresh (cookie)           | 1                       | Browser flow via cookie jar      |
| 6 | Refresh (JSON body)        | 1                       | CLI flow; uses lastRefreshToken  |
| 7 | Refresh (invalid token)    | —                       | Independent                      |
| 8 | Logout                     | 1 or 5                  | Clears cookie jar                |
| 9 | Refresh after logout       | 6                       | Uses lastRefreshToken (rotated/revoked) |

Requests 3 and 4 require prior server state — deactivate the user
and exhaust the bucket before running them. Request 9 needs request
6 to have populated `lastRefreshToken` and request 8 to have revoked
it.

## Vars

The `Local` environment exposes:

| var                       | default                                          |
|---------------------------|--------------------------------------------------|
| `baseUrl`                 | `http://localhost:8080`                          |
| `username`                | `alice`                                          |
| `password`                | `hunter2`                                        |
| `badPassword`             | `definitely-wrong`                               |
| `deactivatableUsername`   | `charlie`                                        |
| `entryId`                 | `01923456-7890-7abc-9def-0123456789ab` (overwritten by `entries-create`) |

`lastAccessToken` and `lastRefreshToken` are populated at runtime by
the `script:post-response` blocks in `login.bru` and `refresh-body.bru`,
and override `entryId` is captured by `entries-create.bru`.

## CLI equivalent

```
make api-test                              # auth + entries (chained)
make api-test-auth                         # auth only
make api-test-req REQ=auth/login           # single request
make api-test-req REQ=entries/entries-create
make api-test-reset                        # wipe captured token values
```

The bare `bru` equivalent is:

```
bru run doc/bruno-api-tests/auth --env Local
bru run doc/bruno-api-tests/entries --env Local
```

Note: `bru run` resolves `vars:secret` (and runtime env vars from
`script:post-response`) only within the same invocation. To have
`auth/login` populate `lastAccessToken` and have `entries/*` use it,
run both folders in one `bru run` (which is what `make api-test`
does). Standalone folder runs will see empty tokens.

(use the [Bruno CLI](https://www.usebruno.com/bru-cli) if installed).

## Secrets and the auto-format caveat

`lastAccessToken` and `lastRefreshToken` are declared in `vars:secret`
in `environments/Local.bru` — Bruno treats them as opaque secrets
and masks them in the GUI. `bogusRefreshToken` is in the regular
`vars` block (not `vars:secret`) because Bruno CLI 4.x wipes values
declared in `vars:secret` on every run, which would break
`refresh-invalid`.

**Heads-up**: Bruno CLI 4.x writes runtime var values back into
`Local.bru` after each `bru run`. `lastAccessToken` and
`lastRefreshToken` will end up containing real tokens even though
they're declared as secret. Run `make api-test-reset` (or re-write
`Local.bru` from this file's source-of-truth version) before
committing so tokens don't leak into git.

`bogusRefreshToken` keeps its placeholder value through runs because
it's not populated by a script.

## Scope

Covers §10 step 3 (auth) and §10 step 4 (entries / per-user DB).
`/healthz`, `/version`, embedded web assets, CORS and the full
security-header set land in step 5.