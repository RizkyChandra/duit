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
| `add <acct> <amount>` | Signed add (positive = income). For a negative literal use `expense` instead. `--split cat=amt` splits across categories; `--tag` labels (repeatable) |
| `transfer <from> <to> <amount>` | Move money between accounts (linked pair, excluded from income/expense; cross-currency auto-converts, `--dest-amount` overrides) |
| `list <acct> [--month]` | Transactions for a month |
| `find [text] [--account --category --tag --type --min --max --from --to --month]` | Search transactions across all accounts/months |
| `edit <id>` / `rm <id>` | Edit (only the flags you pass; `--amount` keeps direction unless you type `+`/`-`) or delete a transaction |
| `verify [--fix]` | Check data integrity (balances, running totals, splits); `--fix` repairs drift by recomputing |
| `config` | Show current settings (token redacted) |
| `balance [acct]` | Balance(s) |
| `summary [--account] [--month]` | Per-category income/expense/net |
| `recompute [acct]` | Rebuild cached balances from transaction files |
| `budget set\|list\|rm\|status` | Per-category monthly limits; `status` shows spent vs limit (warns on overspend, never blocks) |
| `category add\|list\|rename\|rm` | Curated category list; `rename` migrates existing transactions, splits, and budgets |
| `account add\|list\|rm\|archive\|unarchive` | Manage accounts; archived ones are hidden from `list`/`balance` unless `--all` (data kept, still in net worth) |
| `recurring add\|list\|rm\|apply` | Recurring rules; `--to <acct>` makes a recurring transfer; `apply` materializes everything due up to a date (idempotent) |
| `fx set\|list\|rm\|update` | Exchange rates for cross-currency views; `update` pulls from frankfurter.app (ECB) |
| `networth [--in CODE]` | Total balance across all accounts, converted to one currency |
| `report [--month] [--in CODE]` / `report trend [--months N]` / `report networth [--months N]` | In-terminal bar charts: category breakdown, monthly expense trend, net-worth-over-time |
| `export [--account] [--from --to] [--out]` | Write transactions to CSV |
| `import <account> <file>` | Import a CSV (auto-detects date/amount/debit/credit/category headers; `--dry-run`, override flags) |
| `attach <id> <file>` / `receipt <id>` | Attach a receipt to a transaction / print its stored path |
| `auth set-token\|migrate` | Manage the GitHub PAT (stored in the OS keychain, falls back to config file) |
| `sync` | Commit pending + pull + push |
| `mcp` | Run the MCP stdio server |
| `tui` | Interactive TUI (also the default) |

## MCP

`duit mcp` speaks MCP over stdio and exposes: `list_accounts`, `get_balance`,
`add_transaction`, `list_transactions`, `summary`, `budget_status`,
`list_recurring`, `apply_recurring`, `net_worth`, `find_transactions`, `transfer`.
Register it with an MCP client, e.g.:

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
- [x] v0.5.0 — transfers between accounts · net-worth-over-time (`report networth`) · TUI transfer (`t`) + editable budget/fx screens
- [x] v0.6.0 — search/filter (`find`) · split transactions (`--split`) · receipts (`attach`/`receipt`) · MCP `find_transactions` + `transfer`
- [x] v0.7.0 — category management (`category add/list/rename/rm`; rename migrates existing transactions, splits, and budgets)
- [x] v0.8.0 — recurring transfers (`recurring add --to`) · TUI dashboard (`D`) · account archiving (`account archive`)
- [x] v0.9.0 — tags (`--tag`, `find --tag`) · CLI `edit`/`rm` · `verify` integrity check · scriptability (`--json`, `config`, shell completions)

### Planned

- **v1.0.0** — stabilization: hardening, docs, and a stable release

## License

MIT — see [LICENSE](LICENSE).
