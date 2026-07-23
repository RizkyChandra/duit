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
