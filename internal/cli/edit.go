package cli

import (
	"fmt"
	"strings"

	"github.com/RizkyChandra/duit/internal/ledger"
	"github.com/spf13/cobra"
)

func editCmd() *cobra.Command {
	var amount, category, note, date string
	var tags []string
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit an existing transaction (only the flags you pass change)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			account, _, tx, err := store.FindTransaction(args[0])
			if err != nil {
				return err
			}
			acct, _, _ := store.Account(account)
			if cmd.Flags().Changed("amount") {
				if len(tx.Splits) > 0 {
					return fmt.Errorf("cannot change the amount of a split transaction directly; re-create it")
				}
				amt, err := ledger.ParseMoney(amount, acct.Decimals)
				if err != nil {
					return fmt.Errorf("bad amount %q: %w", amount, err)
				}
				// Without an explicit +/- sign, keep the transaction's existing
				// direction (editing an expense's magnitude stays an expense).
				trimmed := strings.TrimSpace(amount)
				if !strings.HasPrefix(trimmed, "+") && !strings.HasPrefix(trimmed, "-") && tx.Amount < 0 {
					amt = -amt
				}
				tx.Amount = amt
			}
			if cmd.Flags().Changed("category") {
				tx.Category = category
			}
			if cmd.Flags().Changed("note") {
				tx.Note = note
			}
			if cmd.Flags().Changed("date") {
				tx.Date = date
			}
			if cmd.Flags().Changed("tag") {
				tx.Tags = tags
			}
			if err := store.EditTransaction(tx); err != nil {
				return err
			}
			commit(c, "edit transaction "+tx.ID)
			acct, _, _ = store.Account(account)
			fmt.Printf("Updated %s. Balance: %s %s\n", tx.ID, acct.Balance.Format(acct.Decimals), acct.Currency)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&amount, "amount", "", "new signed amount")
	f.StringVar(&category, "category", "", "new category")
	f.StringVar(&note, "note", "", "new note")
	f.StringVar(&date, "date", "", "new date YYYY-MM-DD")
	f.StringArrayVar(&tags, "tag", nil, "replace tags (repeatable)")
	return cmd
}

func rmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>",
		Short: "Delete a transaction by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			account, month, _, err := store.FindTransaction(args[0])
			if err != nil {
				return err
			}
			if err := store.RemoveTransaction(account, month, args[0]); err != nil {
				return err
			}
			commit(c, "remove transaction "+args[0])
			fmt.Printf("Removed transaction %s from %s\n", args[0], account)
			return nil
		},
	}
}
