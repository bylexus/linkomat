# CLI (`cli/`)

**Status: placeholder.** It currently builds a no-op `main` that
prints `shared.Version`; the real subcommands (`login`, `logout`,
`add`, `ls`, `search`, `get`, `rm`, `edit`) are not implemented yet.

## What is decided

- Go 1.22+, `cobra`, `github.com/google/uuid` (UUIDv7), `fsnotify`.
  HTTP via `net/http`. No other third-party deps.
- Online-only in v1 (no local DB, no offline queue).
- Auth: `Authorization: Bearer <access_token>` for non-auth routes;
  refresh token in JSON body of `/auth/refresh`. Tokens persist in
  `~/.config/linkomat/credentials.json` (mode 0600).
- Server URL via `LINKOMAT_SERVER` env var (default
  `https://app.linkomat.example.com`).
- No password cache.

## Subcommands (planned, §8.3)

- `linkomat login` — prompt username + password, store tokens.
- `linkomat logout` — call `/auth/logout`, clear credentials.
- `linkomat add [args...]` — auto-detect positional args (one URL,
  or title + URL), prompt for tags (single line) and note
  (multi-line, ctrl-d to finish). `--tags` / `--note` flags override
  prompts.
- `linkomat search <query>` — tabular output, ordered by
  `updated_at` desc.
- `linkomat ls [--tag ...]` — list entries, optional tag filter.
- `linkomat get <id>` — print a single entry as JSON.
- `linkomat edit <id-or-text>` — if id, edit that entry; else
  search title/url and present a selection list. Snapshot to a temp
  file with a `.md` template (`# @tag1 @tag2` for tags, body for
  note), open in `$EDITOR`, watch with `fsnotify` (500 ms debounce),
  PUT to server. Empty file → confirm delete.
- `linkomat rm <id>` — `DELETE /entries/:id`.
- `linkomat sync` — no-op in v1.

Exit codes: `0` success, `1` server error, `2` auth required,
`3` invalid args, `4` not found.

## Entry point

`cli/main.go` — `cobra` root, subcommands added here as they're
implemented. Mirrors the server's `api/main.go` shape.
