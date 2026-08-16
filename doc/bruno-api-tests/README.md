# linkomat-api — Bruno Collection

Manual smoke tests for the auth surface of the Linkomat server
(§10 step 3). The collection targets the local server and assumes the
default `:8080` listen address.

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
4. The collection appears as **linkomat-api** with the **Local**
   environment under the environments dropdown.

## Run order

The requests are numbered with `seq` and ordered to share state.

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
| `bogusRefreshToken`       | 32-byte hex string that was never issued         |

`lastAccessToken` and `lastRefreshToken` are populated at runtime by
the `script:post-response` blocks in `login.bru` and `refresh-body.bru`.

## CLI equivalent

```
make api-test                              # full folder
make api-test-req REQ=login                # single request
make api-test-reset                        # wipe captured token values
```

The bare `bru` equivalent is:

```
bru run doc/bruno-api-tests --env Local
```

(use the [Bruno CLI](https://www.usebruno.com/bru-cli) if installed).

## Secrets and the auto-format caveat

`bogusRefreshToken`, `lastAccessToken`, and `lastRefreshToken` are
declared in `vars:secret` in `environments/Local.bru` — Bruno treats
them as opaque secrets and masks them in the GUI.

**Heads-up**: Bruno CLI 4.x writes runtime var values back into
`Local.bru` after each `bru run`. `lastAccessToken` and
`lastRefreshToken` will end up containing real tokens even though
they're declared as secret. Run `make api-test-reset` (or re-write
`Local.bru` from this file's source-of-truth version) before
committing so tokens don't leak into git.

`bogusRefreshToken` keeps its placeholder value through runs because
it's not populated by a script.

## Scope

Covers only the auth endpoints from §10 step 3. `/entries`,
`/healthz`, `/version`, embedded web assets, CORS and the full
security-header set land in later steps.