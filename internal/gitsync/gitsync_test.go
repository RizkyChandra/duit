package gitsync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/RizkyChandra/duit/internal/config"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func TestEnsureRepoCommitAndSync(t *testing.T) {
	dir := t.TempDir()

	// 1. EnsureRepo turns a plain dir into a git repo.
	if err := EnsureRepo(dir, ""); err != nil {
		t.Fatalf("EnsureRepo: %v", err)
	}
	if _, err := git.PlainOpen(dir); err != nil {
		t.Fatalf("dir is not a repo after EnsureRepo: %v", err)
	}
	// Idempotent.
	if err := EnsureRepo(dir, ""); err != nil {
		t.Fatalf("EnsureRepo (second call): %v", err)
	}

	// 2. CommitAll commits a change, then reports a clean tree.
	if err := os.WriteFile(filepath.Join(dir, "ledger.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	committed, err := CommitAll(dir, "add ledger")
	if err != nil {
		t.Fatalf("CommitAll: %v", err)
	}
	if !committed {
		t.Fatal("CommitAll: expected true (a change was committed), got false")
	}
	committed, err = CommitAll(dir, "no-op")
	if err != nil {
		t.Fatalf("CommitAll (clean): %v", err)
	}
	if committed {
		t.Fatal("CommitAll on clean tree: expected false, got true")
	}

	// 3. Sync pushes to a local bare repo acting as the remote.
	barePath := t.TempDir()
	if _, err := git.PlainInit(barePath, true); err != nil {
		t.Fatalf("PlainInit bare: %v", err)
	}
	if err := EnsureRepo(dir, barePath); err != nil {
		t.Fatalf("EnsureRepo with remote: %v", err)
	}
	if err := Sync(dir, barePath, config.Auth{}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// Verify the commit reached the bare repo: resolve refs/heads/main there and
	// confirm ledger.txt is present in that commit's tree.
	bare, err := git.PlainOpen(barePath)
	if err != nil {
		t.Fatalf("open bare: %v", err)
	}
	ref, err := bare.Reference(plumbing.NewBranchReferenceName("main"), true)
	if err != nil {
		t.Fatalf("bare has no main ref after Sync: %v", err)
	}
	commit, err := bare.CommitObject(ref.Hash())
	if err != nil {
		t.Fatalf("bare commit: %v", err)
	}
	if _, err := commit.File("ledger.txt"); err != nil {
		t.Fatalf("ledger.txt did not reach the remote: %v", err)
	}
}
