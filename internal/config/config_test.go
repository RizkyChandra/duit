package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	in := &Config{
		DataDir:         "/data/duit",
		DefaultCurrency: "IDR",
		Remote:          "https://github.com/RizkyChandra/duit.git",
		Auth:            Auth{Method: "pat", Token: "secret"},
	}
	if err := Save(path, in); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if fi.Mode().Perm() != 0o600 {
		t.Errorf("config perm = %o want 600 (holds a token)", fi.Mode().Perm())
	}
	out, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if *out != *in {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}
