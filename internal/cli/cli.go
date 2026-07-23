// Package cli wires the ledger core, git sync, MCP server, and TUI together as
// the `duit` command-line interface.
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/RizkyChandra/duit/internal/config"
	"github.com/RizkyChandra/duit/internal/gitsync"
	"github.com/RizkyChandra/duit/internal/ledger"
	"github.com/RizkyChandra/duit/internal/tui"
	"github.com/spf13/cobra"
)

// Execute runs the duit CLI, exiting non-zero on error.
func Execute(version string) {
	if err := newRoot(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newRoot(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "duit",
		Short:         "Git-backed personal ledger (CLI + TUI + MCP)",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		// No subcommand: launch the TUI.
		RunE: func(cmd *cobra.Command, args []string) error { return runTUI() },
	}
	root.AddCommand(
		initCmd(),
		accountCmd(),
		addCmd(),
		signedAddCmd("income", +1, "Record income (positive)"),
		signedAddCmd("expense", -1, "Record an expense (magnitude, stored negative)"),
		listCmd(),
		balanceCmd(),
		summaryCmd(),
		recomputeCmd(),
		budgetCmd(),
		recurringCmd(),
		authCmd(),
		syncCmd(),
		mcpCmd(),
		tuiCmd(),
	)
	return root
}

// mustCtx loads config and opens the store, erroring if duit was never init'd.
func mustCtx() (*config.Config, *ledger.Store, error) {
	p, err := config.DefaultPath()
	if err != nil {
		return nil, nil, err
	}
	c, err := config.Load(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, errors.New("no ledger configured — run `duit init` first")
	}
	if err != nil {
		return nil, nil, err
	}
	return c, &ledger.Store{Dir: c.DataDir}, nil
}

// commit makes a best-effort local git commit; a git failure warns but does not
// fail the command (the JSON is already written).
func commit(c *config.Config, msg string) {
	if _, err := gitsync.CommitAll(c.DataDir, msg); err != nil {
		fmt.Fprintln(os.Stderr, "warning: git commit failed:", err)
	}
}

func runTUI() error {
	c, store, err := mustCtx()
	if err != nil {
		return err
	}
	return tui.Run(store, func() error {
		if _, err := gitsync.CommitAll(c.DataDir, "sync via tui"); err != nil {
			return err
		}
		return gitsync.Sync(c.DataDir, c.Remote, resolveAuth(c))
	})
}

// expandHome expands a leading ~ to the user's home directory.
func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}
