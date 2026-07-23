package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RizkyChandra/duit/internal/config"
	"github.com/RizkyChandra/duit/internal/ledger"
	"github.com/spf13/cobra"
)

func fxCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "fx", Short: "Manage exchange rates for cross-currency views"}
	cmd.AddCommand(fxSetCmd(), fxListCmd(), fxRmCmd(), fxUpdateCmd())
	return cmd
}

// ensureBase makes sure the rate table has a base currency (the ledger default)
// and that the base maps to 1.0.
func ensureBase(r *ledger.Rates, c *config.Config) {
	if r.Rates == nil {
		r.Rates = map[string]float64{}
	}
	if r.Base == "" {
		r.Base = c.DefaultCurrency
	}
	r.Rates[r.Base] = 1
}

func fxSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <code> <rate>",
		Short: "Set a rate: how many <code> equal one base-currency unit",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			rate, err := strconv.ParseFloat(args[1], 64)
			if err != nil || rate <= 0 {
				return fmt.Errorf("bad rate %q (want a positive number)", args[1])
			}
			code := strings.ToUpper(args[0])
			rates, err := store.LoadRates()
			if err != nil {
				return err
			}
			ensureBase(&rates, c)
			rates.Rates[code] = rate
			if err := store.SaveRates(rates); err != nil {
				return err
			}
			commit(c, "set fx rate "+code)
			fmt.Printf("1 %s = %g %s\n", rates.Base, rate, code)
			return nil
		},
	}
}

func fxListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List exchange rates",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, store, err := mustCtx()
			if err != nil {
				return err
			}
			rates, err := store.LoadRates()
			if err != nil {
				return err
			}
			if len(rates.Rates) == 0 {
				fmt.Println("no rates set — `duit fx set <code> <rate>` or `duit fx update`")
				return nil
			}
			codes := make([]string, 0, len(rates.Rates))
			for code := range rates.Rates {
				codes = append(codes, code)
			}
			sort.Strings(codes)
			w := tw()
			fmt.Fprintf(w, "base %s\n", rates.Base)
			for _, code := range codes {
				fmt.Fprintf(w, "1 %s\t=\t%g %s\n", rates.Base, rates.Rates[code], code)
			}
			return w.Flush()
		},
	}
}

func fxRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <code>",
		Short: "Remove a rate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			code := strings.ToUpper(args[0])
			rates, err := store.LoadRates()
			if err != nil {
				return err
			}
			if code == rates.Base {
				return fmt.Errorf("cannot remove the base currency %q", code)
			}
			if _, ok := rates.Rates[code]; !ok {
				return fmt.Errorf("no rate for %q", code)
			}
			delete(rates.Rates, code)
			if err := store.SaveRates(rates); err != nil {
				return err
			}
			commit(c, "remove fx rate "+code)
			fmt.Printf("Removed rate %s\n", code)
			return nil
		},
	}
}

func fxUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Refresh rates from frankfurter.app (ECB) for the currencies you use",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			rates, err := store.LoadRates()
			if err != nil {
				return err
			}
			ensureBase(&rates, c)
			// Currencies to refresh: those used by accounts plus any already in
			// the table, minus the base.
			accts, err := store.LoadAccounts()
			if err != nil {
				return err
			}
			want := map[string]bool{}
			for _, a := range accts {
				if code := strings.ToUpper(a.Currency); code != "" && code != rates.Base {
					want[code] = true
				}
			}
			for code := range rates.Rates {
				if code != rates.Base {
					want[code] = true
				}
			}
			if len(want) == 0 {
				fmt.Println("no non-base currencies to update")
				return nil
			}
			symbols := make([]string, 0, len(want))
			for code := range want {
				symbols = append(symbols, code)
			}
			sort.Strings(symbols)
			fetched, date, err := fetchFrankfurter(rates.Base, symbols)
			if err != nil {
				return err
			}
			for code, rate := range fetched {
				rates.Rates[strings.ToUpper(code)] = rate
			}
			if err := store.SaveRates(rates); err != nil {
				return err
			}
			commit(c, "update fx rates")
			fmt.Printf("Updated %d rate(s) from frankfurter.app (%s)\n", len(fetched), date)
			return nil
		},
	}
}

// fetchFrankfurter pulls rates for `symbols` relative to `base` from the free,
// no-key frankfurter.app (ECB reference rates).
func fetchFrankfurter(base string, symbols []string) (map[string]float64, string, error) {
	q := url.Values{"from": {base}, "to": {strings.Join(symbols, ",")}}
	u := "https://api.frankfurter.app/latest?" + q.Encode()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("frankfurter.app returned %s (is %s a supported base currency?)", resp.Status, base)
	}
	var out struct {
		Date  string             `json:"date"`
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, "", err
	}
	return out.Rates, out.Date, nil
}

func networthCmd() *cobra.Command {
	var in string
	cmd := &cobra.Command{
		Use:   "networth",
		Short: "Total balance across all accounts, converted to one currency",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			rates, err := store.LoadRates()
			if err != nil {
				return err
			}
			if in == "" {
				if rates.Base != "" {
					in = rates.Base
				} else {
					in = c.DefaultCurrency
				}
			}
			in = strings.ToUpper(in)
			accts, err := store.LoadAccounts()
			if err != nil {
				return err
			}
			dec := ledger.CurrencyDecimals(in)
			var total ledger.Money
			missing := map[string]bool{}
			w := tw()
			fmt.Fprintf(w, "ACCOUNT\tBALANCE\tIN %s\n", in)
			for _, a := range accts {
				conv, err := rates.Convert(a.Balance, a.Currency, in)
				if err != nil {
					missing[a.Currency] = true
					fmt.Fprintf(w, "%s\t%s %s\t(no rate)\n", a.Name, a.Balance.Format(a.Decimals), a.Currency)
					continue
				}
				total += conv
				fmt.Fprintf(w, "%s\t%s %s\t%s\n", a.Name, a.Balance.Format(a.Decimals), a.Currency, conv.Format(dec))
			}
			fmt.Fprintf(w, "TOTAL\t\t%s %s\n", total.Format(dec), in)
			if err := w.Flush(); err != nil {
				return err
			}
			if len(missing) > 0 {
				codes := make([]string, 0, len(missing))
				for c := range missing {
					codes = append(codes, c)
				}
				sort.Strings(codes)
				fmt.Fprintf(os.Stderr, "warning: no rate for %s; add one with `duit fx set` or `duit fx update`\n", strings.Join(codes, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&in, "in", "", "target currency (default: base / ledger default)")
	return cmd
}
