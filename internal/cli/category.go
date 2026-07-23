package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/RizkyChandra/duit/internal/ledger"
	"github.com/spf13/cobra"
)

func categoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "category",
		Aliases: []string{"cat"},
		Short:   "Manage the curated category list",
	}
	cmd.AddCommand(categoryAddCmd(), categoryListCmd(), categoryRenameCmd(), categoryRmCmd())
	return cmd
}

func categoryAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name>",
		Short: "Register a category",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			if err := store.AddCategory(args[0]); err != nil {
				return err
			}
			commit(c, "add category "+args[0])
			fmt.Printf("Added category %q\n", args[0])
			return nil
		},
	}
}

func categoryListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List categories (with how many transactions use each)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, store, err := mustCtx()
			if err != nil {
				return err
			}
			cats, err := store.LoadCategories()
			if err != nil {
				return err
			}
			if len(cats) == 0 {
				fmt.Println("no categories registered — `duit category add <name>`")
				return nil
			}
			w := tw()
			fmt.Fprintln(w, "CATEGORY\tUSED BY")
			for _, name := range cats {
				n, _ := store.CategoryInUse(name)
				fmt.Fprintf(w, "%s\t%d\n", name, n)
			}
			return w.Flush()
		},
	}
}

func categoryRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename a category, migrating existing transactions and budgets",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			n, err := store.RenameCategory(args[0], args[1])
			if err != nil {
				return err
			}
			commit(c, fmt.Sprintf("rename category %s -> %s", args[0], args[1]))
			fmt.Printf("Renamed %q → %q (migrated %d transaction(s))\n", args[0], args[1], n)
			return nil
		},
	}
}

func categoryRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a category from the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			if n, _ := store.CategoryInUse(args[0]); n > 0 {
				fmt.Fprintf(os.Stderr, "note: %d transaction(s) still use %q (their category text is unchanged)\n", n, args[0])
			}
			if err := store.RemoveCategory(args[0]); err != nil {
				return err
			}
			commit(c, "remove category "+args[0])
			fmt.Printf("Removed category %q\n", args[0])
			return nil
		},
	}
}

// warnUnknownCategories warns (never blocks) when a transaction uses categories
// not in the curated list — but only once the user has registered any, so
// people who don't use category management see no noise.
func warnUnknownCategories(store *ledger.Store, cats []string) {
	registered, err := store.LoadCategories()
	if err != nil || len(registered) == 0 {
		return
	}
	known := map[string]bool{}
	for _, c := range registered {
		known[c] = true
	}
	var unknown []string
	for _, c := range cats {
		if c != "" && !known[c] {
			unknown = append(unknown, c)
		}
	}
	if len(unknown) > 0 {
		fmt.Fprintf(os.Stderr, "note: unregistered categor%s %s (add with `duit category add`)\n",
			plural(len(unknown), "y", "ies"), strings.Join(unknown, ", "))
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
