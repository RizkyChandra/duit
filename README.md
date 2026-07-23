# duit

A git-backed personal ledger — one Go binary, three frontends: **CLI**, **TUI**, and **MCP** server.

Track accounts, income, and expenses as plain JSON in a git repo you own, so your data
syncs to your own GitHub account.

> `duit` — "money" in Malay/Indonesian.

## Status

Early development. See the [roadmap board](https://github.com/users/RizkyChandra/projects/5/views/1).

- [ ] R0 — Scaffold
- [ ] R1 — Core ledger (money, models, storage)
- [ ] R2 — Config + init
- [ ] R3 — Git sync (go-git, SSH/PAT)
- [ ] R4 — CLI
- [ ] R5 — MCP server
- [ ] R6 — TUI
- [ ] R7 — Polish

## Design

- **Money** is stored as integer minor units (never float).
- **Storage** is per-account, split one JSON file per month:
  ```
  config.json                    # data dir, default currency, remote, auth
  accounts.json                  # accounts + cached balances
  txns/<account>/<YYYY-MM>.json  # {opening, closing, transactions[]}
  ```
- **Git** is embedded (go-git); the data dir is a repo pushed to your own remote.

## Build

```sh
go build -o duit ./cmd/duit
```

## License

MIT — see [LICENSE](LICENSE).
