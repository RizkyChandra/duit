package cli

import (
	"fmt"
	"os"

	"github.com/RizkyChandra/duit/internal/config"
	"github.com/RizkyChandra/duit/internal/ledger"
	"github.com/spf13/cobra"
)

func budgetCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "budget", Short: "Manage per-category monthly budgets"}
	cmd.AddCommand(budgetSetCmd(), budgetListCmd(), budgetRmCmd(), budgetStatusCmd())
	return cmd
}

func budgetSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <category> <amount>",
		Short: "Set a monthly limit for a category (in the default currency)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			limit, err := ledger.ParseMoney(args[1], budgetDecimals(c))
			if err != nil {
				return fmt.Errorf("bad amount %q: %w", args[1], err)
			}
			if err := store.SetBudget(args[0], limit); err != nil {
				return err
			}
			commit(c, "set budget "+args[0])
			fmt.Printf("Budget for %q set to %s %s\n", args[0], limit.Format(budgetDecimals(c)), c.DefaultCurrency)
			return nil
		},
	}
}

func budgetListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List budgets",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			budgets, err := store.LoadBudgets()
			if err != nil {
				return err
			}
			if len(budgets) == 0 {
				fmt.Println("no budgets set — `duit budget set <category> <amount>`")
				return nil
			}
			d := budgetDecimals(c)
			w := tw()
			fmt.Fprintln(w, "CATEGORY\tLIMIT")
			for _, b := range budgets {
				fmt.Fprintf(w, "%s\t%s %s\n", b.Category, b.Limit.Format(d), c.DefaultCurrency)
			}
			return w.Flush()
		},
	}
}

func budgetRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <category>",
		Short: "Remove a budget",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			if err := store.RemoveBudget(args[0]); err != nil {
				return err
			}
			commit(c, "remove budget "+args[0])
			fmt.Printf("Removed budget %q\n", args[0])
			return nil
		},
	}
}

func budgetStatusCmd() *cobra.Command {
	var month string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show spent vs limit per category for a month",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			if month == "" {
				month = thisMonth()
			}
			lines, err := store.BudgetStatus(month)
			if err != nil {
				return err
			}
			if len(lines) == 0 {
				fmt.Println("no budgets set")
				return nil
			}
			d := budgetDecimals(c)
			w := tw()
			fmt.Fprintf(w, "Budgets %s (%s)\n", month, c.DefaultCurrency)
			fmt.Fprintln(w, "CATEGORY\tLIMIT\tSPENT\tREMAINING\t")
			for _, l := range lines {
				flag := ""
				if l.Over {
					flag = "OVER"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					l.Category, l.Limit.Format(d), l.Spent.Format(d), l.Remaining.Format(d), flag)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&month, "month", "", "month YYYY-MM (default current)")
	return cmd
}

// budgetDecimals is the decimal precision for budget amounts (the ledger's
// default currency). Budgets are single-currency until v0.3 FX.
func budgetDecimals(c *config.Config) int { return ledger.CurrencyDecimals(c.DefaultCurrency) }

// warnOverBudget prints a stderr warning if category is over its monthly budget.
func warnOverBudget(store *ledger.Store, c *config.Config, category, month string) {
	lines, err := store.BudgetStatus(month)
	if err != nil {
		return
	}
	d := budgetDecimals(c)
	for _, l := range lines {
		if l.Category == category && l.Over {
			fmt.Fprintf(os.Stderr, "⚠ over budget for %q: spent %s of %s %s\n",
				category, l.Spent.Format(d), l.Limit.Format(d), c.DefaultCurrency)
			return
		}
	}
}
