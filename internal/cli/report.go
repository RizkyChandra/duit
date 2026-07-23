package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/RizkyChandra/duit/internal/ledger"
	"github.com/spf13/cobra"
)

// bar renders a horizontal block bar sized to value/max over width cells. A
// non-zero value always gets at least one cell so tiny amounts stay visible.
func bar(value, max float64, width int) string {
	if max <= 0 || value <= 0 {
		return ""
	}
	n := int(value / max * float64(width))
	if n < 1 {
		n = 1
	}
	if n > width {
		n = width
	}
	return strings.Repeat("█", n)
}

func reportCmd() *cobra.Command {
	var account, month, in string
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Expense breakdown by category for a month",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, store, err := mustCtx()
			if err != nil {
				return err
			}
			if month == "" {
				month = thisMonth()
			}
			accts, err := store.LoadAccounts()
			if err != nil {
				return err
			}

			decimals, rates, err := reportCurrency(store, accts, account, in)
			if err != nil {
				return err
			}
			in = strings.ToUpper(in)

			cats := map[string]ledger.Money{}
			currencies := map[string]bool{}
			for _, a := range accts {
				if account != "" && a.Name != account {
					continue
				}
				currencies[a.Currency] = true
				txns, err := store.Transactions(a.Name, month)
				if err != nil {
					return err
				}
				for _, t := range txns {
					if t.Amount >= 0 {
						continue // expenses only
					}
					amt := -t.Amount // magnitude
					if in != "" && a.Currency != in {
						conv, err := rates.Convert(t.Amount, a.Currency, in)
						if err != nil {
							return fmt.Errorf("cannot convert %s to %s: %w (set a rate with `duit fx set`)", a.Currency, in, err)
						}
						amt = -conv
					}
					key := t.Category
					if key == "" {
						key = "(uncategorized)"
					}
					cats[key] += amt
				}
			}

			title := "Expenses " + month
			if in != "" {
				title += " (in " + in + ")"
			}
			fmt.Println(title)
			if len(cats) == 0 {
				fmt.Println("  no expenses")
				return nil
			}
			renderBarsOrdered(mapPairs(cats), decimals)
			if in == "" && len(currencies) > 1 {
				fmt.Println("note: accounts use different currencies; pass --in CODE to convert and combine")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "limit to one account")
	cmd.Flags().StringVar(&month, "month", "", "month YYYY-MM (default current)")
	cmd.Flags().StringVar(&in, "in", "", "convert all amounts to this currency (needs fx rates)")
	cmd.AddCommand(reportTrendCmd())
	return cmd
}

func reportTrendCmd() *cobra.Command {
	var account, in string
	var months int
	cmd := &cobra.Command{
		Use:   "trend",
		Short: "Total expense per month for the last N months",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, store, err := mustCtx()
			if err != nil {
				return err
			}
			if months < 1 {
				months = 6
			}
			accts, err := store.LoadAccounts()
			if err != nil {
				return err
			}
			decimals, rates, err := reportCurrency(store, accts, account, in)
			if err != nil {
				return err
			}
			in = strings.ToUpper(in)

			monthList := lastMonths(months)
			totals := map[string]ledger.Money{}
			currencies := map[string]bool{}
			for _, a := range accts {
				if account != "" && a.Name != account {
					continue
				}
				currencies[a.Currency] = true
				for _, m := range monthList {
					txns, err := store.Transactions(a.Name, m)
					if err != nil {
						return err
					}
					for _, t := range txns {
						if t.Amount >= 0 {
							continue
						}
						amt := -t.Amount
						if in != "" && a.Currency != in {
							conv, err := rates.Convert(t.Amount, a.Currency, in)
							if err != nil {
								return fmt.Errorf("cannot convert %s to %s: %w (set a rate with `duit fx set`)", a.Currency, in, err)
							}
							amt = -conv
						}
						totals[m] += amt
					}
				}
			}

			title := "Expense trend"
			if in != "" {
				title += " (in " + in + ")"
			}
			fmt.Println(title)
			pairs := make([]pair, len(monthList))
			for i, m := range monthList {
				pairs[i] = pair{m, totals[m]}
			}
			renderBarsOrdered(pairs, decimals)
			if in == "" && len(currencies) > 1 {
				fmt.Println("note: accounts use different currencies; pass --in CODE to convert and combine")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "limit to one account")
	cmd.Flags().StringVar(&in, "in", "", "convert all amounts to this currency (needs fx rates)")
	cmd.Flags().IntVar(&months, "months", 6, "number of months to show")
	return cmd
}

// reportCurrency picks display decimals and loads rates when --in is set.
func reportCurrency(store *ledger.Store, accts []ledger.Account, account, in string) (int, ledger.Rates, error) {
	if in != "" {
		rates, err := store.LoadRates()
		if err != nil {
			return 0, ledger.Rates{}, err
		}
		return ledger.CurrencyDecimals(strings.ToUpper(in)), rates, nil
	}
	// No conversion: use the (first matching) account's decimals.
	for _, a := range accts {
		if account == "" || a.Name == account {
			return a.Decimals, ledger.Rates{}, nil
		}
	}
	return 2, ledger.Rates{}, nil
}

type pair struct {
	label string
	value ledger.Money
}

func mapPairs(m map[string]ledger.Money) []pair {
	pairs := make([]pair, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, pair{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].value > pairs[j].value })
	return pairs
}

const barWidth = 30

// renderBarsOrdered draws pairs in the given order with aligned labels/values.
func renderBarsOrdered(pairs []pair, decimals int) {
	var max ledger.Money
	labelW, valW := 0, 0
	vals := make([]string, len(pairs))
	for i, p := range pairs {
		if p.value > max {
			max = p.value
		}
		if len(p.label) > labelW {
			labelW = len(p.label)
		}
		vals[i] = p.value.Format(decimals)
		if len(vals[i]) > valW {
			valW = len(vals[i])
		}
	}
	for i, p := range pairs {
		fmt.Printf("  %-*s  %*s  %s\n", labelW, p.label, valW, vals[i],
			bar(float64(p.value), float64(max), barWidth))
	}
}

// lastMonths returns the last n months (YYYY-MM), oldest first, ending this
// month. Anchored to the 1st so month-end days don't skip a month.
func lastMonths(n int) []string {
	now := time.Now()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	out := make([]string, 0, n)
	for i := n - 1; i >= 0; i-- {
		out = append(out, first.AddDate(0, -i, 0).Format("2006-01"))
	}
	return out
}
