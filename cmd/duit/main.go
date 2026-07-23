// Command duit is a git-backed personal ledger with CLI, TUI, and MCP frontends.
package main

import "github.com/RizkyChandra/duit/internal/cli"

// version is set at build time via -ldflags; defaults to "dev".
var version = "dev"

func main() {
	cli.Execute(version)
}
