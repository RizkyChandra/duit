package ledger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Recurring is a rule that generates transactions on a cadence. Transactions are
// materialized only by an explicit ApplyRecurring (catch-up), never on a timer.
type Recurring struct {
	ID       string `json:"id"`
	Account  string `json:"account"`
	Amount   Money  `json:"amount"`
	Category string `json:"category,omitempty"`
	Note     string `json:"note,omitempty"`
	// To, when set, makes this a recurring transfer into that account (Amount is
	// the positive transfer magnitude, cross-currency auto-converted at apply).
	To          string `json:"to,omitempty"`
	Cadence     string `json:"cadence"`  // "daily" | "weekly" | "monthly"
	Interval    int    `json:"interval"` // >= 1
	Day         int    `json:"day"`      // day-of-month 1..31 for monthly; ignored otherwise
	Start       string `json:"start"`    // YYYY-MM-DD
	LastApplied string `json:"lastApplied"`
}

func (s *Store) recurringPath() string { return filepath.Join(s.Dir, "recurring.json") }

// LoadRecurring returns the recurring rules, or nil if none exist yet.
func (s *Store) LoadRecurring() ([]Recurring, error) {
	var r []Recurring
	err := readJSON(s.recurringPath(), &r)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return r, err
}

func (s *Store) SaveRecurring(r []Recurring) error {
	return writeJSON(s.recurringPath(), r)
}

// AddRecurring validates and appends r, assigning an ID if empty.
func (s *Store) AddRecurring(r Recurring) (Recurring, error) {
	switch r.Cadence {
	case "daily", "weekly", "monthly":
	default:
		return Recurring{}, fmt.Errorf("invalid cadence %q (want daily|weekly|monthly)", r.Cadence)
	}
	if !validDate(r.Start) {
		return Recurring{}, fmt.Errorf("invalid start date %q (want YYYY-MM-DD)", r.Start)
	}
	if r.To != "" && r.To == r.Account {
		return Recurring{}, fmt.Errorf("cannot transfer to the same account")
	}
	if r.Interval < 1 {
		r.Interval = 1
	}
	unlock, err := s.lock()
	if err != nil {
		return Recurring{}, err
	}
	defer unlock()
	rules, err := s.LoadRecurring()
	if err != nil {
		return Recurring{}, err
	}
	if r.ID == "" {
		r.ID = newID()
	}
	return r, s.SaveRecurring(append(rules, r))
}

// RemoveRecurring deletes a rule by ID, erroring if not found.
func (s *Store) RemoveRecurring(id string) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	rules, err := s.LoadRecurring()
	if err != nil {
		return err
	}
	out := rules[:0:0]
	found := false
	for _, r := range rules {
		if r.ID == id {
			found = true
			continue
		}
		out = append(out, r)
	}
	if !found {
		return fmt.Errorf("recurring rule %q not found", id)
	}
	return s.SaveRecurring(out)
}

// dueDates returns every occurrence of r in (LastApplied, until] (or [Start, until]
// if never applied), chronologically. Dates are YYYY-MM-DD.
func (r Recurring) dueDates(until time.Time) []string {
	start, err := time.Parse(dateLayout, r.Start)
	if err != nil {
		return nil
	}
	interval := r.Interval
	if interval < 1 {
		interval = 1
	}
	var last time.Time
	haveLast := false
	if r.LastApplied != "" {
		if lt, err := time.Parse(dateLayout, r.LastApplied); err == nil {
			last, haveLast = lt, true
		}
	}
	occurrence := func(k int) time.Time {
		switch r.Cadence {
		case "weekly":
			return start.AddDate(0, 0, interval*7*k)
		case "monthly":
			// First of the target month, then clamp the desired day to its length.
			m := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, interval*k, 0)
			day := r.Day
			if day < 1 {
				day = start.Day()
			}
			lastDay := time.Date(m.Year(), m.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
			if day > lastDay {
				day = lastDay
			}
			return time.Date(m.Year(), m.Month(), day, 0, 0, 0, 0, time.UTC)
		default: // daily
			return start.AddDate(0, 0, interval*k)
		}
	}
	var out []string
	for k := 0; ; k++ {
		occ := occurrence(k)
		if occ.After(until) {
			break
		}
		if haveLast && !occ.After(last) { // occ <= lastApplied: already applied
			continue
		}
		out = append(out, occ.Format(dateLayout))
	}
	return out
}

// ApplyRecurring materializes every due transaction up to and including `until`
// (YYYY-MM-DD), advancing each rule's LastApplied. It is idempotent: re-running
// with the same or an earlier `until` creates nothing. Returns the count created.
func (s *Store) ApplyRecurring(until string) (int, error) {
	untilT, err := time.Parse(dateLayout, until)
	if err != nil {
		return 0, fmt.Errorf("invalid until date %q (want YYYY-MM-DD)", until)
	}
	rules, err := s.LoadRecurring()
	if err != nil {
		return 0, err
	}
	// Compute all due dates first; AddTransaction locks internally so we must not
	// hold the lock while calling it.
	type job struct {
		idx  int
		date string
	}
	var jobs []job
	for i := range rules {
		for _, d := range rules[i].dueDates(untilT) {
			jobs = append(jobs, job{i, d})
		}
	}
	if len(jobs) == 0 {
		return 0, nil
	}
	for _, j := range jobs {
		r := rules[j.idx]
		if r.To != "" {
			amt := r.Amount
			if amt < 0 {
				amt = -amt
			}
			if _, _, err := s.Transfer(r.Account, r.To, amt, nil, j.date, r.Note); err != nil {
				return 0, err
			}
		} else if _, err := s.AddTransaction(r.Account, Transaction{
			Date:     j.date,
			Amount:   r.Amount,
			Category: r.Category,
			Note:     r.Note,
		}); err != nil {
			return 0, err
		}
		rules[j.idx].LastApplied = j.date // jobs are appended in chronological order per rule
	}
	unlock, err := s.lock()
	if err != nil {
		return len(jobs), err
	}
	defer unlock()
	return len(jobs), s.SaveRecurring(rules)
}
