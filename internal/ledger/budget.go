package ledger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Budget is a monthly spending limit for a category.
//
// ponytail: single-currency. Limit and Spent are raw Money summed across all
// accounts regardless of currency; correct only when accounts share one.
// Upgrade path: v0.3 FX converts before summing.
type Budget struct {
	Category string `json:"category"`
	Limit    Money  `json:"limit"`
}

func (s *Store) budgetsPath() string { return filepath.Join(s.Dir, "budgets.json") }

// LoadBudgets returns the budget list, or nil if none exists yet.
func (s *Store) LoadBudgets() ([]Budget, error) {
	var b []Budget
	err := readJSON(s.budgetsPath(), &b)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

func (s *Store) SaveBudgets(b []Budget) error {
	return writeJSON(s.budgetsPath(), b)
}

// SetBudget upserts a category's limit.
func (s *Store) SetBudget(category string, limit Money) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	budgets, err := s.LoadBudgets()
	if err != nil {
		return err
	}
	for i := range budgets {
		if budgets[i].Category == category {
			budgets[i].Limit = limit
			return s.SaveBudgets(budgets)
		}
	}
	return s.SaveBudgets(append(budgets, Budget{Category: category, Limit: limit}))
}

// RemoveBudget deletes a category's budget, erroring if it is not set.
func (s *Store) RemoveBudget(category string) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	budgets, err := s.LoadBudgets()
	if err != nil {
		return err
	}
	out := budgets[:0:0]
	found := false
	for _, b := range budgets {
		if b.Category == category {
			found = true
			continue
		}
		out = append(out, b)
	}
	if !found {
		return fmt.Errorf("budget %q not found", category)
	}
	return s.SaveBudgets(out)
}

// BudgetLine reports a category's budget against actual spending for a month.
type BudgetLine struct {
	Category  string
	Limit     Money
	Spent     Money
	Remaining Money
	Over      bool
}

// BudgetStatus reports each budget's spending for month (YYYY-MM). Spent is the
// positive magnitude of EXPENSE transactions (negative amounts) in that category,
// summed across all accounts. When a rates.json table exists, each account's
// spend is converted to the base currency before summing (so cross-currency
// budgets are correct); otherwise raw amounts are summed (single-currency).
// Lines are sorted by Category.
func (s *Store) BudgetStatus(month string) ([]BudgetLine, error) {
	budgets, err := s.LoadBudgets()
	if err != nil {
		return nil, err
	}
	accts, err := s.LoadAccounts()
	if err != nil {
		return nil, err
	}
	rates, err := s.LoadRates()
	if err != nil {
		return nil, err
	}
	target := rates.Base
	// spent[category] = -sum(negative amounts) across all accounts for month,
	// converted to the base currency when a rate table is present.
	spent := make(map[string]Money)
	for _, a := range accts {
		txns, err := s.Transactions(a.Name, month)
		if err != nil {
			return nil, err
		}
		for _, t := range txns {
			if t.Transfer != "" {
				continue // transfer legs are not spending
			}
			for _, ln := range t.Lines() {
				if ln.Amount >= 0 {
					continue // income is not spending
				}
				mag := -ln.Amount
				if target != "" && a.Currency != target {
					if conv, err := rates.Convert(mag, a.Currency, target); err == nil {
						mag = conv // best-effort: fall back to raw magnitude if no rate
					}
				}
				spent[ln.Category] += mag
			}
		}
	}
	lines := make([]BudgetLine, 0, len(budgets))
	for _, b := range budgets {
		sp := spent[b.Category]
		lines = append(lines, BudgetLine{
			Category:  b.Category,
			Limit:     b.Limit,
			Spent:     sp,
			Remaining: b.Limit - sp,
			Over:      sp > b.Limit,
		})
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].Category < lines[j].Category })
	return lines, nil
}
