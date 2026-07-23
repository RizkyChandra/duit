package ledger

import "fmt"

// Issue is a data-integrity problem found by Verify.
type Issue struct {
	Account string `json:"account"`
	Kind    string `json:"kind"`
	Detail  string `json:"detail"`
}

// Verify checks integrity without modifying anything: each account's cached
// balance against the sum of its transactions, every month's opening/closing
// running totals, and that split transactions sum to their amount. It returns
// the issues found (empty when the ledger is consistent).
func (s *Store) Verify() ([]Issue, error) {
	accts, err := s.LoadAccounts()
	if err != nil {
		return nil, err
	}
	var issues []Issue
	for _, a := range accts {
		months, err := s.Months(a.Name)
		if err != nil {
			return nil, err
		}
		var running Money
		for _, m := range months {
			mf, err := s.LoadMonth(a.Name, m)
			if err != nil {
				issues = append(issues, Issue{a.Name, "unreadable", fmt.Sprintf("%s: %v", m, err)})
				continue
			}
			if mf.Opening != running {
				issues = append(issues, Issue{a.Name, "opening", fmt.Sprintf("%s opening=%d expected=%d", m, mf.Opening, running)})
			}
			var sum Money
			for _, t := range mf.Transactions {
				sum += t.Amount
				if err := validateSplits(t); err != nil {
					issues = append(issues, Issue{a.Name, "split", fmt.Sprintf("%s tx %s: %v", m, t.ID, err)})
				}
			}
			running += sum
			if mf.Closing != running {
				issues = append(issues, Issue{a.Name, "closing", fmt.Sprintf("%s closing=%d expected=%d", m, mf.Closing, running)})
			}
		}
		if a.Balance != running {
			issues = append(issues, Issue{a.Name, "balance", fmt.Sprintf("cached=%d expected=%d", a.Balance, running)})
		}
	}
	return issues, nil
}
