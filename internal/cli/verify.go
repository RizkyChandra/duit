package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func verifyCmd() *cobra.Command {
	var fix bool
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Check data integrity (cached balances, running totals, splits)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			issues, err := store.Verify()
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(issues)
			}
			if len(issues) == 0 {
				fmt.Println("ok — no integrity issues")
				return nil
			}
			w := tw()
			fmt.Fprintln(w, "ACCOUNT\tKIND\tDETAIL")
			for _, is := range issues {
				fmt.Fprintf(w, "%s\t%s\t%s\n", is.Account, is.Kind, is.Detail)
			}
			w.Flush()
			if fix {
				accts, err := store.LoadAccounts()
				if err != nil {
					return err
				}
				for _, a := range accts {
					if _, err := store.Recompute(a.Name); err != nil {
						return err
					}
				}
				commit(c, "verify --fix (recompute)")
				fmt.Println("recomputed all accounts; re-run `duit verify` to confirm")
				return nil
			}
			return fmt.Errorf("%d integrity issue(s); run `duit verify --fix` to repair balance drift", len(issues))
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "repair balance/running-total drift by recomputing")
	return cmd
}
