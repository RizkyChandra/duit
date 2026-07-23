package cli

import (
	"fmt"
	"strings"

	"github.com/RizkyChandra/duit/internal/ledger"
	"github.com/spf13/cobra"
)

func findCmd() *cobra.Command {
	var account, category, tag, typ, from, to, month, min, max string
	cmd := &cobra.Command{
		Use:   "find [text]",
		Short: "Search transactions across accounts and months",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			switch typ {
			case "", "income", "expense", "transfer":
			default:
				return fmt.Errorf("--type must be income, expense, or transfer")
			}
			f := ledger.FindFilter{Account: account, Category: category, Tag: tag, Type: typ, From: from, To: to}
			if len(args) == 1 {
				f.Text = args[0]
			}
			if month != "" {
				f.From, f.To = month+"-01", month+"-31"
			}
			dec := ledger.CurrencyDecimals(c.DefaultCurrency)
			if min != "" {
				m, err := ledger.ParseMoney(min, dec)
				if err != nil {
					return fmt.Errorf("bad --min %q: %w", min, err)
				}
				f.Min = &m
			}
			if max != "" {
				m, err := ledger.ParseMoney(max, dec)
				if err != nil {
					return fmt.Errorf("bad --max %q: %w", max, err)
				}
				f.Max = &m
			}
			found, err := store.Find(f)
			if err != nil {
				return err
			}
			if jsonOut {
				if found == nil {
					found = []ledger.Found{}
				}
				return printJSON(found)
			}
			if len(found) == 0 {
				fmt.Println("no matching transactions")
				return nil
			}
			// Per-account decimals for formatting.
			accts, _ := store.LoadAccounts()
			decOf := map[string]int{}
			for _, a := range accts {
				decOf[a.Name] = a.Decimals
			}
			w := tw()
			fmt.Fprintln(w, "DATE\tACCOUNT\tAMOUNT\tCATEGORY\tNOTE\tID")
			for _, x := range found {
				cat := x.Category
				if len(x.Splits) > 0 {
					parts := make([]string, len(x.Splits))
					for i, s := range x.Splits {
						parts[i] = s.Category
					}
					cat = "split:" + strings.Join(parts, ",")
				}
				note := x.Note
				if x.Attachment != "" {
					note = "📎 " + note
				}
				if len(x.Tags) > 0 {
					note = strings.TrimSpace(note + " #" + strings.Join(x.Tags, " #"))
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					x.Date, x.Account, x.Amount.Format(decOf[x.Account]), cat, note, x.ID)
			}
			return w.Flush()
		},
	}
	f := cmd.Flags()
	f.StringVar(&account, "account", "", "limit to one account")
	f.StringVar(&category, "category", "", "exact category (incl. split categories)")
	f.StringVar(&tag, "tag", "", "has this tag")
	f.StringVar(&typ, "type", "", "income | expense | transfer")
	f.StringVar(&min, "min", "", "minimum amount magnitude")
	f.StringVar(&max, "max", "", "maximum amount magnitude")
	f.StringVar(&from, "from", "", "from date YYYY-MM-DD (inclusive)")
	f.StringVar(&to, "to", "", "to date YYYY-MM-DD (inclusive)")
	f.StringVar(&month, "month", "", "limit to a month YYYY-MM")
	return cmd
}
