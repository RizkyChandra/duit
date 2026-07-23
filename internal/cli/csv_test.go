package cli

import "testing"

func TestParseAmount(t *testing.T) {
	cases := []struct {
		in       string
		decimals int
		want     int64
	}{
		{"12.34", 2, 1234},
		{"$1,234.56", 2, 123456},
		{"Rp 15000", 0, 15000},
		{"(50.00)", 2, -5000},
		{"-5", 2, -500},
		{"  +7.5 ", 2, 750},
		{"USD 10", 2, 1000},
		{"", 2, 0},
	}
	for _, c := range cases {
		got, err := parseAmount(c.in, c.decimals)
		if err != nil {
			t.Errorf("parseAmount(%q): %v", c.in, err)
			continue
		}
		if int64(got) != c.want {
			t.Errorf("parseAmount(%q) = %d want %d", c.in, int64(got), c.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2026-01-02", "2026-01-02"},
		{"02/01/2026", "2026-01-02"}, // DD/MM/YYYY tried before MM/DD
		{"2026/01/02", "2026-01-02"},
		{"2 Jan 2026", "2026-01-02"},
	}
	for _, c := range cases {
		got, err := parseDate(c.in, "")
		if err != nil {
			t.Errorf("parseDate(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseDate(%q) = %q want %q", c.in, got, c.want)
		}
	}
	if _, err := parseDate("nonsense", ""); err == nil {
		t.Error("expected error on unrecognized date")
	}
}

// Round-trip: an exported-style CSV parses back with auto-detected columns and
// a debit/credit variant nets to the right signed amount.
func TestBuildMappingAndParse(t *testing.T) {
	header := []string{"Date", "Amount", "Category", "Description"}
	m, err := buildMapping(header, colOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	if m.date != 0 || m.amount != 1 || m.category != 2 || m.note != 3 {
		t.Fatalf("mapping = %+v", m)
	}
	rows := [][]string{{"2026-01-05", "-25.00", "food", "lunch"}}
	txns, err := parseRows(rows, m, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(txns) != 1 || txns[0].Amount != -2500 || txns[0].Category != "food" {
		t.Fatalf("parsed %+v", txns)
	}

	// Debit/credit pair: credit is income (+), debit is expense (−).
	dcHeader := []string{"Tanggal", "Debit", "Credit", "Keterangan"}
	dm, err := buildMapping(dcHeader, colOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	if dm.amount >= 0 {
		t.Fatalf("expected no signed amount col, got %d", dm.amount)
	}
	dcRows := [][]string{
		{"05/01/2026", "30.00", "", "atm"},   // debit → -3000
		{"06/01/2026", "", "100.00", "wage"}, // credit → +10000
	}
	dcTxns, err := parseRows(dcRows, dm, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if dcTxns[0].Amount != -3000 || dcTxns[1].Amount != 10000 {
		t.Fatalf("debit/credit parsed %+v", dcTxns)
	}
}
