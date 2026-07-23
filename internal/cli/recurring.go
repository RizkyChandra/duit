package cli

import (
	"fmt"

	"github.com/RizkyChandra/duit/internal/ledger"
	"github.com/spf13/cobra"
)

func recurringCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "recurring",
		Aliases: []string{"rec"},
		Short:   "Manage recurring transaction rules",
	}
	cmd.AddCommand(recurringAddCmd(), recurringListCmd(), recurringRmCmd(), recurringApplyCmd())
	return cmd
}

func recurringAddCmd() *cobra.Command {
	var account, to, amount, category, note, cadence, start string
	var interval, day int
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a recurring rule",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			if account == "" || amount == "" {
				return fmt.Errorf("--account and --amount are required")
			}
			acct, ok, err := store.Account(account)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("unknown account %q", account)
			}
			amt, err := ledger.ParseMoney(amount, acct.Decimals)
			if err != nil {
				return fmt.Errorf("bad amount %q: %w", amount, err)
			}
			if start == "" {
				start = today()
			}
			var destName string
			if to != "" {
				dst, ok, err := store.Account(to)
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("unknown destination account %q", to)
				}
				destName = dst.Name
				if amt < 0 {
					amt = -amt // transfers use a positive magnitude
				}
			}
			r, err := store.AddRecurring(ledger.Recurring{
				Account: acct.Name, Amount: amt, Category: category, Note: note, To: destName,
				Cadence: cadence, Interval: interval, Day: day, Start: start,
			})
			if err != nil {
				return err
			}
			commit(c, "add recurring "+r.ID)
			if destName != "" {
				fmt.Printf("Added recurring transfer %s: %s %s %s from %s → %s (%s)\n",
					r.ID, r.Cadence, amt.Format(acct.Decimals), acct.Currency, acct.Name, destName, start)
			} else {
				fmt.Printf("Added recurring %s: %s %s %s from %s (%s)\n",
					r.ID, r.Cadence, amt.Format(acct.Decimals), acct.Currency, start, acct.Name)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&account, "account", "", "account (required)")
	f.StringVar(&to, "to", "", "destination account — makes this a recurring transfer")
	f.StringVar(&amount, "amount", "", "signed amount (or positive transfer magnitude with --to) (required)")
	f.StringVar(&category, "category", "", "category")
	f.StringVar(&note, "note", "", "note")
	f.StringVar(&cadence, "cadence", "monthly", "daily | weekly | monthly")
	f.IntVar(&interval, "interval", 1, "repeat every N periods")
	f.IntVar(&day, "day", 0, "day-of-month for monthly (0 = use start's day)")
	f.StringVar(&start, "start", "", "start date YYYY-MM-DD (default today)")
	return cmd
}

func recurringListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List recurring rules",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, store, err := mustCtx()
			if err != nil {
				return err
			}
			rules, err := store.LoadRecurring()
			if err != nil {
				return err
			}
			if len(rules) == 0 {
				fmt.Println("no recurring rules — `duit recurring add ...`")
				return nil
			}
			w := tw()
			fmt.Fprintln(w, "ID\tACCOUNT\tAMOUNT\tCADENCE\tEVERY\tDAY\tSTART\tLAST APPLIED")
			for _, r := range rules {
				dec := 2
				if a, ok, _ := store.Account(r.Account); ok {
					dec = a.Decimals
				}
				last := r.LastApplied
				if last == "" {
					last = "never"
				}
				account := r.Account
				if r.To != "" {
					account += " → " + r.To // recurring transfer
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%d\t%s\t%s\n",
					r.ID, account, r.Amount.Format(dec), r.Cadence, r.Interval, r.Day, r.Start, last)
			}
			return w.Flush()
		},
	}
}

func recurringRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a recurring rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			if err := store.RemoveRecurring(args[0]); err != nil {
				return err
			}
			commit(c, "remove recurring "+args[0])
			fmt.Printf("Removed recurring %q\n", args[0])
			return nil
		},
	}
}

func recurringApplyCmd() *cobra.Command {
	var until string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Materialize all recurring rules due up to a date (default today)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			if until == "" {
				until = today()
			}
			n, err := store.ApplyRecurring(until)
			if err != nil {
				return err
			}
			if n > 0 {
				commit(c, fmt.Sprintf("apply recurring (%d txns)", n))
			}
			fmt.Printf("Applied %d recurring transaction(s) up to %s\n", n, until)
			return nil
		},
	}
	cmd.Flags().StringVar(&until, "until", "", "apply through this date YYYY-MM-DD (default today)")
	return cmd
}
