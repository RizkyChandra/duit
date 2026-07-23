package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/RizkyChandra/duit/internal/gitsync"
	"github.com/RizkyChandra/duit/internal/ledger"
	"github.com/RizkyChandra/duit/internal/mcpserver"
	"github.com/spf13/cobra"
)

func today() string     { return time.Now().Format("2006-01-02") }
func thisMonth() string { return time.Now().Format("2006-01") }
func tw() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
}

// --- account ---

func accountCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "account", Short: "Manage accounts"}
	cmd.AddCommand(accountAddCmd(), accountListCmd(), accountRmCmd(),
		accountArchiveCmd("archive", true), accountArchiveCmd("unarchive", false))
	return cmd
}

func accountArchiveCmd(use string, archived bool) *cobra.Command {
	short := "Hide an account from default lists (data retained)"
	if !archived {
		short = "Restore an archived account to active"
	}
	return &cobra.Command{
		Use:   use + " <name>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			if err := store.SetArchived(args[0], archived); err != nil {
				return err
			}
			commit(c, use+" account "+args[0])
			fmt.Printf("%sd account %q\n", use, args[0])
			return nil
		},
	}
}

func accountAddCmd() *cobra.Command {
	var currency, typ string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add an account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			if currency == "" {
				currency = c.DefaultCurrency
			}
			acct := ledger.Account{
				Name:     args[0],
				Currency: currency,
				Decimals: ledger.CurrencyDecimals(currency),
				Type:     typ,
				Created:  today(),
			}
			if err := store.AddAccount(acct); err != nil {
				return err
			}
			commit(c, "add account "+acct.Name)
			fmt.Printf("Added account %q (%s)\n", acct.Name, acct.Currency)
			return nil
		},
	}
	cmd.Flags().StringVar(&currency, "currency", "", "currency code (defaults to ledger default)")
	cmd.Flags().StringVar(&typ, "type", "", "account type (cash, bank, credit, ...)")
	return cmd
}

func accountListCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List accounts and balances",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, store, err := mustCtx()
			if err != nil {
				return err
			}
			accts, err := store.LoadAccounts()
			if err != nil {
				return err
			}
			if len(accts) == 0 {
				fmt.Println("no accounts yet — add one with `duit account add <name>`")
				return nil
			}
			w := tw()
			fmt.Fprintln(w, "NAME\tTYPE\tCURRENCY\tBALANCE")
			for _, a := range accts {
				if a.Archived && !all {
					continue
				}
				name := a.Name
				if a.Archived {
					name += " (archived)"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, a.Type, a.Currency, a.Balance.Format(a.Decimals))
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "include archived accounts")
	return cmd
}

func accountRmCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove an account and all its transactions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			if !yes {
				return fmt.Errorf("this deletes account %q and all its transactions; pass --yes to confirm", args[0])
			}
			if err := store.RemoveAccount(args[0]); err != nil {
				return err
			}
			commit(c, "remove account "+args[0])
			fmt.Printf("Removed account %q\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return cmd
}

// --- transactions ---

func addCmd() *cobra.Command {
	return signedAddCmd("add", 0, "Add a transaction (signed: negative = expense)")
}

// signedAddCmd builds add/income/expense. sign 0 keeps the amount as typed;
// +1 forces income (positive magnitude); -1 forces expense (negative magnitude).
func signedAddCmd(use string, sign int, short string) *cobra.Command {
	var category, note, date string
	var splitSpecs, tags []string
	cmd := &cobra.Command{
		Use:   use + " <account> <amount>",
		Short: short,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			acct, ok, err := store.Account(args[0])
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("unknown account %q", args[0])
			}
			amt, err := ledger.ParseMoney(args[1], acct.Decimals)
			if err != nil {
				return fmt.Errorf("bad amount %q: %w", args[1], err)
			}
			switch {
			case sign > 0 && amt < 0:
				amt = -amt // income: force positive
			case sign < 0 && amt > 0:
				amt = -amt // expense: force negative
			}
			if date == "" {
				date = today()
			}
			splits, err := parseSplits(splitSpecs, amt, acct.Decimals)
			if err != nil {
				return err
			}
			t, err := store.AddTransaction(acct.Name, ledger.Transaction{
				Date: date, Amount: amt, Category: category, Note: note, Splits: splits, Tags: tags,
			})
			if err != nil {
				return err
			}
			acct, _, _ = store.Account(acct.Name)
			commit(c, fmt.Sprintf("%s %s %s to %s", use, amt.Format(acct.Decimals), acct.Currency, acct.Name))
			fmt.Printf("Recorded %s %s (%s). Balance: %s %s\n",
				amt.Format(acct.Decimals), acct.Currency, t.ID,
				acct.Balance.Format(acct.Decimals), acct.Currency)
			warnUnknownCategories(store, affectedCategories(t))
			if amt < 0 {
				for _, cat := range affectedCategories(t) {
					warnOverBudget(store, c, cat, t.Date[:7])
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "category")
	cmd.Flags().StringVar(&note, "note", "", "note")
	cmd.Flags().StringVar(&date, "date", "", "date YYYY-MM-DD (default today)")
	cmd.Flags().StringArrayVar(&splitSpecs, "split", nil, "split part category=amount (repeatable; must sum to the amount)")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "tag label (repeatable)")
	return cmd
}

// parseSplits turns "category=amount" specs into splits whose sign matches the
// transaction amount. The caller's AddTransaction validates that they sum to it.
func parseSplits(specs []string, amt ledger.Money, decimals int) ([]ledger.Split, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	neg := amt < 0
	splits := make([]ledger.Split, 0, len(specs))
	for _, spec := range specs {
		name, valStr, ok := strings.Cut(spec, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, fmt.Errorf("bad --split %q (want category=amount)", spec)
		}
		mag, err := ledger.ParseMoney(strings.TrimSpace(valStr), decimals)
		if err != nil {
			return nil, fmt.Errorf("bad --split amount in %q: %w", spec, err)
		}
		if mag < 0 {
			return nil, fmt.Errorf("--split amounts must be positive magnitudes: %q", spec)
		}
		if neg {
			mag = -mag
		}
		splits = append(splits, ledger.Split{Category: name, Amount: mag})
	}
	return splits, nil
}

// affectedCategories lists the categories a transaction touches (its splits, or
// its single category), used for budget warnings.
func affectedCategories(t ledger.Transaction) []string {
	if len(t.Splits) == 0 {
		if t.Category == "" {
			return nil
		}
		return []string{t.Category}
	}
	cats := make([]string, 0, len(t.Splits))
	for _, s := range t.Splits {
		if s.Category != "" {
			cats = append(cats, s.Category)
		}
	}
	return cats
}

func listCmd() *cobra.Command {
	var month string
	cmd := &cobra.Command{
		Use:   "list <account>",
		Short: "List transactions for a month",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, store, err := mustCtx()
			if err != nil {
				return err
			}
			acct, ok, err := store.Account(args[0])
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("unknown account %q", args[0])
			}
			if month == "" {
				month = thisMonth()
			}
			txns, err := store.Transactions(acct.Name, month)
			if err != nil {
				return err
			}
			if jsonOut {
				if txns == nil {
					txns = []ledger.Transaction{}
				}
				return printJSON(txns)
			}
			if len(txns) == 0 {
				fmt.Printf("no transactions in %s for %s\n", month, acct.Name)
				return nil
			}
			w := tw()
			fmt.Fprintln(w, "DATE\tAMOUNT\tCATEGORY\tNOTE\tID")
			for _, t := range txns {
				cat := t.Category
				if len(t.Splits) > 0 {
					parts := make([]string, len(t.Splits))
					for i, s := range t.Splits {
						parts[i] = s.Category
					}
					cat = "split:" + strings.Join(parts, ",")
				}
				note := t.Note
				if t.Attachment != "" {
					note = "📎 " + note
				}
				if len(t.Tags) > 0 {
					note = strings.TrimSpace(note + " #" + strings.Join(t.Tags, " #"))
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					t.Date, t.Amount.Format(acct.Decimals), cat, note, t.ID)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&month, "month", "", "month YYYY-MM (default current)")
	return cmd
}

func balanceCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "balance [account]",
		Short: "Show account balance(s)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, store, err := mustCtx()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				acct, ok, err := store.Account(args[0])
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("unknown account %q", args[0])
				}
				if jsonOut {
					return printJSON(acct)
				}
				fmt.Printf("%s %s\n", acct.Balance.Format(acct.Decimals), acct.Currency)
				return nil
			}
			accts, err := store.LoadAccounts()
			if err != nil {
				return err
			}
			shown := accts[:0:0]
			for _, a := range accts {
				if !a.Archived || all {
					shown = append(shown, a)
				}
			}
			if jsonOut {
				return printJSON(shown)
			}
			w := tw()
			for _, a := range shown {
				name := a.Name
				if a.Archived {
					name += " (archived)"
				}
				fmt.Fprintf(w, "%s\t%s %s\n", name, a.Balance.Format(a.Decimals), a.Currency)
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "include archived accounts")
	return cmd
}

func summaryCmd() *cobra.Command {
	var account, month, in string
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Income/expense/net per category for a month",
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
			// Assume one currency/decimals for the display (personal use); fall
			// back to 2 if accounts disagree. With --in, convert every amount to
			// that currency so cross-currency accounts combine correctly.
			decimals := 2
			var rates ledger.Rates
			if in != "" {
				in = strings.ToUpper(in)
				decimals = ledger.CurrencyDecimals(in)
				if rates, err = store.LoadRates(); err != nil {
					return err
				}
			}
			type tot struct{ in, out ledger.Money }
			cats := map[string]*tot{}
			var totIn, totOut ledger.Money
			for _, a := range accts {
				if account != "" && a.Name != account {
					continue
				}
				if in == "" {
					decimals = a.Decimals
				}
				txns, err := store.Transactions(a.Name, month)
				if err != nil {
					return err
				}
				for _, t := range txns {
					if t.Transfer != "" {
						continue // transfers are not income/expense
					}
					for _, ln := range t.Lines() {
						amt := ln.Amount
						if in != "" && a.Currency != in {
							conv, err := rates.Convert(amt, a.Currency, in)
							if err != nil {
								return fmt.Errorf("cannot convert %s to %s: %w (set a rate with `duit fx set`)", a.Currency, in, err)
							}
							amt = conv
						}
						key := ln.Category
						if key == "" {
							key = "(uncategorized)"
						}
						if cats[key] == nil {
							cats[key] = &tot{}
						}
						if amt >= 0 {
							cats[key].in += amt
							totIn += amt
						} else {
							cats[key].out += amt
							totOut += amt
						}
					}
				}
			}
			keys := make([]string, 0, len(cats))
			for k := range cats {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if jsonOut {
				type catJSON struct {
					Category string       `json:"category"`
					Income   ledger.Money `json:"income"`
					Expense  ledger.Money `json:"expense"`
					Net      ledger.Money `json:"net"`
				}
				cats2 := make([]catJSON, 0, len(keys))
				for _, k := range keys {
					c := cats[k]
					cats2 = append(cats2, catJSON{k, c.in, c.out, c.in + c.out})
				}
				return printJSON(struct {
					Month      string       `json:"month"`
					Currency   string       `json:"currency,omitempty"`
					Categories []catJSON    `json:"categories"`
					Income     ledger.Money `json:"income"`
					Expense    ledger.Money `json:"expense"`
					Net        ledger.Money `json:"net"`
				}{month, in, cats2, totIn, totOut, totIn + totOut})
			}
			w := tw()
			title := "Summary " + month
			if in != "" {
				title += " (in " + in + ")"
			}
			fmt.Fprintln(w, title)
			fmt.Fprintln(w, "CATEGORY\tINCOME\tEXPENSE\tNET")
			for _, k := range keys {
				c := cats[k]
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", k,
					c.in.Format(decimals), c.out.Format(decimals), (c.in + c.out).Format(decimals))
			}
			fmt.Fprintf(w, "TOTAL\t%s\t%s\t%s\n",
				totIn.Format(decimals), totOut.Format(decimals), (totIn + totOut).Format(decimals))
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "limit to one account")
	cmd.Flags().StringVar(&month, "month", "", "month YYYY-MM (default current)")
	cmd.Flags().StringVar(&in, "in", "", "convert all amounts to this currency (needs fx rates)")
	return cmd
}

func recomputeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "recompute [account]",
		Short: "Rebuild cached balances from transaction files",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			var names []string
			if len(args) == 1 {
				names = []string{args[0]}
			} else {
				accts, err := store.LoadAccounts()
				if err != nil {
					return err
				}
				for _, a := range accts {
					names = append(names, a.Name)
				}
			}
			for _, n := range names {
				if _, err := store.Recompute(n); err != nil {
					return err
				}
				fmt.Printf("%s: recomputed\n", n)
			}
			commit(c, "recompute balances")
			return nil
		},
	}
}

// --- sync / mcp / tui ---

func syncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Commit pending changes and pull+push to the remote",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := mustCtx()
			if err != nil {
				return err
			}
			if c.Remote == "" {
				return fmt.Errorf("no remote configured — set one with `duit init`")
			}
			if _, err := gitsync.CommitAll(c.DataDir, "sync"); err != nil {
				return err
			}
			if err := gitsync.Sync(c.DataDir, c.Remote, resolveAuth(c)); err != nil {
				return err
			}
			fmt.Println("Synced with", c.Remote)
			return nil
		},
	}
}

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run the MCP server over stdio",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, store, err := mustCtx()
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()
			return mcpserver.Serve(ctx, store)
		},
	}
}

func tuiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive TUI",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return runTUI() },
	}
}
