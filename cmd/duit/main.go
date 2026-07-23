// Command duit is a git-backed personal ledger with CLI, TUI, and MCP frontends.
package main

import "fmt"

// version is set at build time via -ldflags; defaults to "dev".
var version = "dev"

func main() {
	// ponytail: stub for R0 scaffold; cobra root wired in R4.
	fmt.Printf("duit %s — personal ledger (WIP)\n", version)
}
