package cli

import (
	"fmt"

	"github.com/RizkyChandra/duit/internal/ledger"
	"github.com/spf13/cobra"
)

func transferCmd() *cobra.Command {
	var date, note, destAmount string
	cmd := &cobra.Command{
		Use:   "transfer <from> <to> <amount>",
		Short: "Move money between accounts (linked pair, not income/expense)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			from, ok, err := store.Account(args[0])
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("unknown account %q", args[0])
			}
			to, ok, err := store.Account(args[1])
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("unknown account %q", args[1])
			}
			amt, err := ledger.ParseMoney(args[2], from.Decimals)
			if err != nil {
				return fmt.Errorf("bad amount %q: %w", args[2], err)
			}
			var destPtr *ledger.Money
			if destAmount != "" {
				d, err := ledger.ParseMoney(destAmount, to.Decimals)
				if err != nil {
					return fmt.Errorf("bad --dest-amount %q: %w", destAmount, err)
				}
				destPtr = &d
			}
			if date == "" {
				date = today()
			}
			src, dst, err := store.Transfer(from.Name, to.Name, amt, destPtr, date, note)
			if err != nil {
				return err
			}
			commit(c, fmt.Sprintf("transfer %s->%s", from.Name, to.Name))
			fmt.Printf("Transferred %s %s from %s → %s %s to %s\n",
				src.Format(from.Decimals), from.Currency, from.Name,
				dst.Format(to.Decimals), to.Currency, to.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "date YYYY-MM-DD (default today)")
	cmd.Flags().StringVar(&note, "note", "", "note")
	cmd.Flags().StringVar(&destAmount, "dest-amount", "", "exact amount received by <to> (overrides fx conversion)")
	return cmd
}
