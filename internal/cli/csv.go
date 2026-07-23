package cli

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RizkyChandra/duit/internal/ledger"
	"github.com/spf13/cobra"
)

// --- export ---

func exportCmd() *cobra.Command {
	var account, from, to, out string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export transactions to CSV",
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
			type row struct {
				date, account, amount, currency, category, note, id string
			}
			var rows []row
			for _, a := range accts {
				if account != "" && a.Name != account {
					continue
				}
				months, err := store.Months(a.Name)
				if err != nil {
					return err
				}
				for _, m := range months {
					txns, err := store.Transactions(a.Name, m)
					if err != nil {
						return err
					}
					for _, t := range txns {
						if from != "" && t.Date < from {
							continue
						}
						if to != "" && t.Date > to {
							continue
						}
						rows = append(rows, row{
							t.Date, a.Name, t.Amount.Format(a.Decimals),
							a.Currency, t.Category, t.Note, t.ID,
						})
					}
				}
			}
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].date < rows[j].date })

			f := os.Stdout
			if out != "" {
				f, err = os.Create(out)
				if err != nil {
					return err
				}
				defer f.Close()
			}
			w := csv.NewWriter(f)
			if err := w.Write([]string{"date", "account", "amount", "currency", "category", "note", "id"}); err != nil {
				return err
			}
			for _, r := range rows {
				if err := w.Write([]string{r.date, r.account, r.amount, r.currency, r.category, r.note, r.id}); err != nil {
					return err
				}
			}
			w.Flush()
			if err := w.Error(); err != nil {
				return err
			}
			if out != "" {
				fmt.Printf("Exported %d transactions to %s\n", len(rows), out)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "limit to one account")
	cmd.Flags().StringVar(&from, "from", "", "start date YYYY-MM-DD (inclusive)")
	cmd.Flags().StringVar(&to, "to", "", "end date YYYY-MM-DD (inclusive)")
	cmd.Flags().StringVar(&out, "out", "", "output file (default stdout)")
	return cmd
}

// --- import ---

func importCmd() *cobra.Command {
	var dateCol, amountCol, debitCol, creditCol, categoryCol, noteCol, dateFormat string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "import <account> <file>",
		Short: "Import transactions from a CSV file",
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

			f, err := os.Open(args[1])
			if err != nil {
				return err
			}
			defer f.Close()
			r := csv.NewReader(f)
			r.FieldsPerRecord = -1 // tolerate ragged rows
			records, err := r.ReadAll()
			if err != nil {
				return err
			}
			if len(records) < 2 {
				return fmt.Errorf("%s: no data rows", args[1])
			}
			header := records[0]

			m, err := buildMapping(header, colOverrides{
				date: dateCol, amount: amountCol, debit: debitCol,
				credit: creditCol, category: categoryCol, note: noteCol,
			})
			if err != nil {
				return err
			}

			txns, err := parseRows(records[1:], m, dateFormat, acct.Decimals)
			if err != nil {
				return err
			}

			if dryRun {
				fmt.Println("Dry run — nothing written.")
				fmt.Println("Column mapping:")
				printMapping(header, m)
				fmt.Printf("Parsed %d rows. First %d:\n", len(txns), min(5, len(txns)))
				for i := 0; i < len(txns) && i < 5; i++ {
					t := txns[i]
					fmt.Printf("  %s  %12s  %-12s  %s\n",
						t.Date, t.Amount.Format(acct.Decimals), t.Category, t.Note)
				}
				return nil
			}

			n, err := store.ImportTransactions(acct.Name, txns)
			if err != nil {
				return err
			}
			commit(c, fmt.Sprintf("import %d transactions into %s", n, acct.Name))
			fmt.Printf("Imported %d transactions into %s\n", n, acct.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&dateCol, "date-col", "", "date column name or 1-based index")
	cmd.Flags().StringVar(&amountCol, "amount-col", "", "signed-amount column name or 1-based index")
	cmd.Flags().StringVar(&debitCol, "debit-col", "", "debit column name or 1-based index")
	cmd.Flags().StringVar(&creditCol, "credit-col", "", "credit column name or 1-based index")
	cmd.Flags().StringVar(&categoryCol, "category-col", "", "category column name or 1-based index")
	cmd.Flags().StringVar(&noteCol, "note-col", "", "note column name or 1-based index")
	cmd.Flags().StringVar(&dateFormat, "dateformat", "", "explicit Go time layout for dates")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "parse and print the mapping without writing")
	return cmd
}

// mapping holds resolved 0-based column indexes; -1 means absent.
type mapping struct{ date, amount, debit, credit, category, note int }

type colOverrides struct{ date, amount, debit, credit, category, note string }

// header keyword sets (case-insensitive substring match).
var (
	dateKeys     = []string{"date", "tanggal", "time"}
	amountKeys   = []string{"amount", "value", "jumlah", "nominal"}
	debitKeys    = []string{"debit", "withdrawal", "out"}
	creditKeys   = []string{"credit", "deposit", "in"}
	categoryKeys = []string{"category", "kategori"}
	noteKeys     = []string{"note", "description", "keterangan", "memo", "narrative", "details"}
)

// buildMapping resolves each column from an override (header name or 1-based
// index) or by auto-detecting header keywords.
func buildMapping(header []string, ov colOverrides) (mapping, error) {
	resolve := func(override string, keys []string) (int, error) {
		if override != "" {
			return resolveCol(header, override)
		}
		return detectCol(header, keys), nil
	}
	m := mapping{}
	var err error
	if m.date, err = resolve(ov.date, dateKeys); err != nil {
		return m, err
	}
	if m.amount, err = resolve(ov.amount, amountKeys); err != nil {
		return m, err
	}
	if m.debit, err = resolve(ov.debit, debitKeys); err != nil {
		return m, err
	}
	if m.credit, err = resolve(ov.credit, creditKeys); err != nil {
		return m, err
	}
	if m.category, err = resolve(ov.category, categoryKeys); err != nil {
		return m, err
	}
	if m.note, err = resolve(ov.note, noteKeys); err != nil {
		return m, err
	}
	// A signed amount column and a debit/credit column can collide (e.g. "in"
	// matches both amount-ish and credit). If a debit/credit pair is present,
	// prefer it and drop the ambiguous single amount.
	if m.debit >= 0 || m.credit >= 0 {
		m.amount = -1
	}
	if m.date < 0 {
		return m, fmt.Errorf("could not find a date column (use --date-col)")
	}
	if m.amount < 0 && m.debit < 0 && m.credit < 0 {
		return m, fmt.Errorf("could not find an amount or debit/credit column (use --amount-col or --debit-col/--credit-col)")
	}
	return m, nil
}

// resolveCol maps an override (1-based index or header name) to a 0-based index.
func resolveCol(header []string, override string) (int, error) {
	if n, err := strconv.Atoi(strings.TrimSpace(override)); err == nil {
		if n < 1 || n > len(header) {
			return -1, fmt.Errorf("column index %d out of range (1..%d)", n, len(header))
		}
		return n - 1, nil
	}
	want := strings.ToLower(strings.TrimSpace(override))
	for i, h := range header {
		if strings.ToLower(strings.TrimSpace(h)) == want {
			return i, nil
		}
	}
	return -1, fmt.Errorf("no column named %q", override)
}

// detectCol returns the first header whose trimmed lowercase name contains any
// keyword, or -1.
func detectCol(header []string, keys []string) int {
	for i, h := range header {
		name := strings.ToLower(strings.TrimSpace(h))
		for _, k := range keys {
			if strings.Contains(name, k) {
				return i
			}
		}
	}
	return -1
}

var dateLayouts = []string{
	"2006-01-02", "02/01/2006", "01/02/2006",
	"2006/01/02", "02-01-2006", "2 Jan 2006",
}

func parseRows(records [][]string, m mapping, dateFormat string, decimals int) ([]ledger.Transaction, error) {
	get := func(row []string, i int) string {
		if i < 0 || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	var txns []ledger.Transaction
	for lineno, row := range records {
		if len(row) == 0 || (len(row) == 1 && strings.TrimSpace(row[0]) == "") {
			continue // skip blank lines
		}
		date, err := parseDate(get(row, m.date), dateFormat)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", lineno+2, err)
		}
		var amt ledger.Money
		if m.amount >= 0 {
			amt, err = parseAmount(get(row, m.amount), decimals)
			if err != nil {
				return nil, fmt.Errorf("row %d: %w", lineno+2, err)
			}
		} else {
			var debit, credit ledger.Money
			if s := get(row, m.debit); s != "" {
				if debit, err = parseAmount(s, decimals); err != nil {
					return nil, fmt.Errorf("row %d: debit: %w", lineno+2, err)
				}
			}
			if s := get(row, m.credit); s != "" {
				if credit, err = parseAmount(s, decimals); err != nil {
					return nil, fmt.Errorf("row %d: credit: %w", lineno+2, err)
				}
			}
			// credit = income (+), debit = expense (−); use magnitudes.
			amt = abs(credit) - abs(debit)
		}
		txns = append(txns, ledger.Transaction{
			Date:     date,
			Amount:   amt,
			Category: get(row, m.category),
			Note:     get(row, m.note),
		})
	}
	return txns, nil
}

func parseDate(s, layout string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty date")
	}
	if layout != "" {
		t, err := time.Parse(layout, s)
		if err != nil {
			return "", fmt.Errorf("date %q does not match layout %q", s, layout)
		}
		return t.Format("2006-01-02"), nil
	}
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("unrecognized date %q (try --dateformat)", s)
}

// parseAmount cleans a raw cell — surrounding spaces, a leading currency symbol
// or letters, thousands separators, and (n) parentheses-negatives — then parses
// it with ParseMoney. Returns the signed magnitude of the cell.
func parseAmount(s string, decimals int) (ledger.Money, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	neg := false
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		neg = true
		s = s[1 : len(s)-1]
	}
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "-") {
		neg = !neg
		s = strings.TrimSpace(s[1:])
	} else if strings.HasPrefix(s, "+") {
		s = strings.TrimSpace(s[1:])
	}
	// Strip a leading currency symbol or letters (e.g. "$", "Rp", "USD").
	s = strings.TrimLeftFunc(s, func(r rune) bool {
		return !(r >= '0' && r <= '9') && r != '.' && r != ',' && r != '-'
	})
	s = strings.TrimSpace(s)
	// Remove thousands separators (commas), keep the decimal point.
	s = strings.ReplaceAll(s, ",", "")
	m, err := ledger.ParseMoney(s, decimals)
	if err != nil {
		return 0, fmt.Errorf("bad amount %q", s)
	}
	if neg {
		m = -m
	}
	return m, nil
}

func abs(m ledger.Money) ledger.Money {
	if m < 0 {
		return -m
	}
	return m
}

func printMapping(header []string, m mapping) {
	name := func(i int) string {
		if i < 0 || i >= len(header) {
			return "(none)"
		}
		return fmt.Sprintf("%s (col %d)", strings.TrimSpace(header[i]), i+1)
	}
	w := tw()
	fmt.Fprintf(w, "  date\t%s\n", name(m.date))
	if m.amount >= 0 {
		fmt.Fprintf(w, "  amount\t%s\n", name(m.amount))
	} else {
		fmt.Fprintf(w, "  debit\t%s\n", name(m.debit))
		fmt.Fprintf(w, "  credit\t%s\n", name(m.credit))
	}
	fmt.Fprintf(w, "  category\t%s\n", name(m.category))
	fmt.Fprintf(w, "  note\t%s\n", name(m.note))
	w.Flush()
}
