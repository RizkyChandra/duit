# duit

A git-backed personal ledger — one Go binary, three frontends: **CLI**, **TUI**, and **MCP** server.

Track accounts, income, and expenses as plain JSON in a git repo you own, so your data
syncs to your own GitHub account.

> `duit` — "money" in Malay/Indonesian.

## Install

```sh
go install github.com/RizkyChandra/duit/cmd/duit@latest
# or from a clone, with a version stamp:
go build -ldflags "-X main.version=$(git describe --tags --always)" -o duit ./cmd/duit
```

## Quick start

```sh
duit init                       # choose data dir, default currency, remote + auth
duit account add cash --type cash
duit account add bank --type bank --currency USD

duit income  cash 5000000 --category salary
duit expense cash 15000   --category food --note lunch
duit expense bank 12.50   --category fee

duit list cash                  # this month's transactions
duit balance                    # all account balances
duit summary --month 2026-07    # income/expense/net per category
duit sync                       # commit + pull + push to your remote

duit                            # no args → interactive TUI
duit mcp                        # MCP server over stdio
```

### Commands

| Command | Purpose |
|---|---|
| `init` | Create data dir + git repo + config (data dir, currency, remote, auth) |
| `account add\|list\|rm` | Manage accounts (`rm` needs `--yes`) |
| `income \| expense <acct> <amount>` | Record money (positive magnitude; direction by verb) |
| `add <acct> <amount>` | Signed add (positive = income). For a negative literal use `expense` instead |
| `list <acct> [--month]` | Transactions for a month |
| `balance [acct]` | Balance(s) |
| `summary [--account] [--month]` | Per-category income/expense/net |
| `recompute [acct]` | Rebuild cached balances from transaction files |
| `budget set\|list\|rm\|status` | Per-category monthly limits; `status` shows spent vs limit (warns on overspend, never blocks) |
| `recurring add\|list\|rm\|apply` | Recurring rules; `apply` materializes everything due up to a date (idempotent) |
| `fx set\|list\|rm\|update` | Exchange rates for cross-currency views; `update` pulls from frankfurter.app (ECB) |
| `networth [--in CODE]` | Total balance across all accounts, converted to one currency |
| `report [--month] [--in CODE]` / `report trend [--months N]` | In-terminal bar charts: category breakdown + monthly trend |
| `export [--account] [--from --to] [--out]` | Write transactions to CSV |
| `import <account> <file>` | Import a CSV (auto-detects date/amount/debit/credit/category headers; `--dry-run`, override flags) |
| `auth set-token\|migrate` | Manage the GitHub PAT (stored in the OS keychain, falls back to config file) |
| `sync` | Commit pending + pull + push |
| `mcp` | Run the MCP stdio server |
| `tui` | Interactive TUI (also the default) |

## MCP

`duit mcp` speaks MCP over stdio and exposes: `list_accounts`, `get_balance`,
`add_transaction`, `list_transactions`, `summary`, `budget_status`,
`list_recurring`, `apply_recurring`, `net_worth`. Register it with an MCP client, e.g.:

```json
{ "mcpServers": { "duit": { "command": "duit", "args": ["mcp"] } } }
```

## Design

- **Money** is stored as integer minor units (never float); decimals are per-currency
  (2 for USD, 0 for IDR/JPY).
- **Storage** is per-account, one JSON file per month:
  ```
  accounts.json                  # accounts + cached balances
  txns/<account>/<YYYY-MM>.json  # {opening, closing, transactions[]}
  ```
  Cached balances are derived; `duit recompute` rebuilds them.
- **Config** lives at `~/.config/duit/config.json` (mode 0600, outside the data repo)
  so your token is never committed.
- **Git** is embedded (go-git); the data dir is a repo pushed to your own remote
  over SSH or a personal access token.

## Roadmap

Tracked at [Project #5](https://github.com/users/RizkyChandra/projects/5/views/1).

- [x] R0 — Scaffold
- [x] R1 — Core ledger (money, models, storage)
- [x] R2 — Config
- [x] R3 — Git sync (go-git, SSH/PAT)
- [x] R4 — CLI
- [x] R5 — MCP server
- [x] R6 — TUI
- [x] R7 — Polish

- [x] v0.2.0 — OS-keychain PAT storage · per-category monthly budgets (warn-only) · recurring transactions (explicit `apply`)
- [x] v0.3.0 — multi-currency conversion (manual rate table + `fx update`), net worth, currency-aware summary/budgets
- [x] v0.4.0 — CSV import/export · terminal reports (`report`, `report trend`) · TUI budget & fx screens (`b`/`f`)

## License

MIT — see [LICENSE](LICENSE).
