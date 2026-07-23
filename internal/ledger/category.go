package ledger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (s *Store) categoriesPath() string { return filepath.Join(s.Dir, "categories.json") }

// LoadCategories returns the curated category list, or nil if none exists.
func (s *Store) LoadCategories() ([]string, error) {
	var c []string
	err := readJSON(s.categoriesPath(), &c)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return c, err
}

// SaveCategories writes the category list sorted and unique.
func (s *Store) SaveCategories(c []string) error {
	seen := map[string]bool{}
	out := c[:0:0]
	for _, name := range c {
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return writeJSON(s.categoriesPath(), out)
}

// AddCategory adds a category to the registry, erroring if it already exists.
func (s *Store) AddCategory(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("category name is empty")
	}
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	cats, err := s.LoadCategories()
	if err != nil {
		return err
	}
	for _, c := range cats {
		if c == name {
			return fmt.Errorf("category %q already exists", name)
		}
	}
	return s.SaveCategories(append(cats, name))
}

// RemoveCategory removes a category from the registry (transactions that used it
// keep the string). Errors if it is not registered.
func (s *Store) RemoveCategory(name string) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	cats, err := s.LoadCategories()
	if err != nil {
		return err
	}
	out := cats[:0:0]
	found := false
	for _, c := range cats {
		if c == name {
			found = true
			continue
		}
		out = append(out, c)
	}
	if !found {
		return fmt.Errorf("category %q not found", name)
	}
	return s.SaveCategories(out)
}

// CategoryInUse reports how many transactions reference the category.
func (s *Store) CategoryInUse(name string) (int, error) {
	found, err := s.Find(FindFilter{Category: name})
	if err != nil {
		return 0, err
	}
	return len(found), nil
}

// RenameCategory renames oldName to newName in the registry and migrates every
// transaction, split, and budget. Returns the number of transactions changed.
// Balances are unaffected, so no recompute is needed.
func (s *Store) RenameCategory(oldName, newName string) (int, error) {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return 0, fmt.Errorf("new category name is empty")
	}
	if oldName == newName {
		return 0, nil
	}
	unlock, err := s.lock()
	if err != nil {
		return 0, err
	}
	defer unlock()

	// Registry.
	cats, err := s.LoadCategories()
	if err != nil {
		return 0, err
	}
	reg := cats[:0:0]
	for _, c := range cats {
		if c != oldName {
			reg = append(reg, c)
		}
	}
	if err := s.SaveCategories(append(reg, newName)); err != nil {
		return 0, err
	}

	// Transactions and their splits.
	accts, err := s.LoadAccounts()
	if err != nil {
		return 0, err
	}
	changed := 0
	for _, a := range accts {
		months, err := s.Months(a.Name)
		if err != nil {
			return 0, err
		}
		for _, m := range months {
			mf, err := s.LoadMonth(a.Name, m)
			if err != nil {
				return 0, err
			}
			dirty := false
			for i := range mf.Transactions {
				t := &mf.Transactions[i]
				if t.Category == oldName {
					t.Category = newName
					dirty = true
					changed++
				}
				for j := range t.Splits {
					if t.Splits[j].Category == oldName {
						t.Splits[j].Category = newName
						dirty = true
					}
				}
			}
			if dirty {
				if err := writeJSON(s.monthPath(a.Name, m), mf); err != nil {
					return 0, err
				}
			}
		}
	}

	// Budgets (if a budget already exists for newName, drop the old one).
	budgets, err := s.LoadBudgets()
	if err != nil {
		return 0, err
	}
	hasNew := false
	for _, b := range budgets {
		if b.Category == newName {
			hasNew = true
		}
	}
	nb := budgets[:0:0]
	bdirty := false
	for _, b := range budgets {
		if b.Category == oldName {
			bdirty = true
			if hasNew {
				continue // merge into existing newName budget
			}
			b.Category = newName
		}
		nb = append(nb, b)
	}
	if bdirty {
		if err := s.SaveBudgets(nb); err != nil {
			return 0, err
		}
	}
	return changed, nil
}
