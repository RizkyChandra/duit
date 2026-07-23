package ledger

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Transaction is a single signed entry: positive is income, negative is expense.
type Transaction struct {
	ID       string `json:"id"`
	Date     string `json:"date"` // YYYY-MM-DD
	Amount   Money  `json:"amount"`
	Category string `json:"category,omitempty"`
	Note     string `json:"note,omitempty"`
	// Transfer, when set, names the counterparty account of a transfer leg.
	// Such legs move money between accounts and are NOT income or expense, so
	// summaries, reports, and budgets exclude them.
	Transfer string `json:"transfer,omitempty"`
	// Splits, when non-empty, break the amount across categories (each Split's
	// Amount carries the same sign as Amount and they must sum to it). Reports
	// and budgets attribute each split to its own category.
	Splits []Split `json:"splits,omitempty"`
	// Attachment is a receipt path relative to the data directory, if any.
	Attachment string `json:"attachment,omitempty"`
	// Tags are free-form labels for cross-cutting filtering (independent of the
	// single Category).
	Tags []string `json:"tags,omitempty"`
}

// Split is one category portion of a split transaction.
type Split struct {
	Category string `json:"category"`
	Amount   Money  `json:"amount"`
	Note     string `json:"note,omitempty"`
}

// Line is a (category, amount) contribution used by summaries, reports, and
// budgets. A plain transaction yields one line; a split yields one per split.
type Line struct {
	Category string
	Amount   Money
}

// Lines returns the transaction's category contributions: its splits when set,
// otherwise a single line for its own category.
func (t Transaction) Lines() []Line {
	if len(t.Splits) == 0 {
		return []Line{{Category: t.Category, Amount: t.Amount}}
	}
	out := make([]Line, len(t.Splits))
	for i, s := range t.Splits {
		out[i] = Line{Category: s.Category, Amount: s.Amount}
	}
	return out
}

const dateLayout = "2006-01-02"

func newID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func validDate(s string) bool {
	_, err := time.Parse(dateLayout, s)
	return err == nil
}
