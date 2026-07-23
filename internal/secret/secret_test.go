package secret

import (
	"testing"

	"github.com/zalando/go-keyring"
)

func TestSecret(t *testing.T) {
	keyring.MockInit()

	if !Available() {
		t.Fatal("Available() = false under mock backend, want true")
	}

	// not-found: no token, no error
	if tok, err := GetToken("origin"); err != nil || tok != "" {
		t.Fatalf("GetToken(missing) = (%q, %v), want (\"\", nil)", tok, err)
	}

	// round-trip
	if err := SetToken("origin", "ghp_abc"); err != nil {
		t.Fatalf("SetToken: %v", err)
	}
	if tok, err := GetToken("origin"); err != nil || tok != "ghp_abc" {
		t.Fatalf("GetToken = (%q, %v), want (\"ghp_abc\", nil)", tok, err)
	}

	// delete, then gone
	if err := DeleteToken("origin"); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}
	if tok, err := GetToken("origin"); err != nil || tok != "" {
		t.Fatalf("GetToken after delete = (%q, %v), want (\"\", nil)", tok, err)
	}

	// delete of missing is not an error
	if err := DeleteToken("origin"); err != nil {
		t.Fatalf("DeleteToken(missing) = %v, want nil", err)
	}
}

func TestKeyFor(t *testing.T) {
	if got := keyFor(""); got != "default" {
		t.Errorf("keyFor(\"\") = %q, want \"default\"", got)
	}
	if got := keyFor("origin"); got != "origin" {
		t.Errorf("keyFor(\"origin\") = %q, want \"origin\"", got)
	}
}
