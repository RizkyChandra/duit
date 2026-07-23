// Package gitsync wraps embedded git (go-git) for duit's data directory, which
// is itself a git repo the user pushes to their own GitHub. It provides just
// enough to init the repo, commit the ledger, and sync (pull+push) to a remote.
package gitsync

import (
	"errors"
	"time"

	"github.com/RizkyChandra/duit/internal/config"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	httpauth "github.com/go-git/go-git/v5/plumbing/transport/http"
	sshauth "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

const mainBranch = "main"

// EnsureRepo makes dir a git repo (init with default branch "main" if needed)
// and, when remote is non-empty, points the "origin" remote at it (adding or
// updating). Idempotent.
func EnsureRepo(dir, remote string) error {
	r, err := git.PlainOpen(dir)
	if errors.Is(err, git.ErrRepositoryNotExists) {
		r, err = git.PlainInitWithOptions(dir, &git.PlainInitOptions{
			InitOptions: git.InitOptions{
				DefaultBranch: plumbing.NewBranchReferenceName(mainBranch),
			},
		})
	}
	if err != nil {
		return err
	}
	return setRemote(r, remote)
}

// setRemote sets origin's URL to remote (add or update). No-op when empty.
func setRemote(r *git.Repository, remote string) error {
	if remote == "" {
		return nil
	}
	cfg, err := r.Config()
	if err != nil {
		return err
	}
	cfg.Remotes["origin"] = &gitconfig.RemoteConfig{Name: "origin", URLs: []string{remote}}
	return r.SetConfig(cfg)
}

// CommitAll stages all changes and commits them with msg. Returns (false, nil)
// when the worktree is clean. The author comes from git config if set, else
// falls back to "duit <duit@localhost>".
func CommitAll(dir, msg string) (bool, error) {
	r, err := git.PlainOpen(dir)
	if err != nil {
		return false, err
	}
	w, err := r.Worktree()
	if err != nil {
		return false, err
	}
	status, err := w.Status()
	if err != nil {
		return false, err
	}
	if status.IsClean() {
		return false, nil
	}
	if err := w.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return false, err
	}
	if _, err := w.Commit(msg, &git.CommitOptions{Author: author(r)}); err != nil {
		return false, err
	}
	return true, nil
}

// author reads user.name/user.email from git config (all scopes), falling back
// to duit's defaults.
func author(r *git.Repository) *object.Signature {
	name, email := "duit", "duit@localhost"
	if cfg, err := r.ConfigScoped(gitconfig.SystemScope); err == nil {
		if cfg.User.Name != "" {
			name = cfg.User.Name
		}
		if cfg.User.Email != "" {
			email = cfg.User.Email
		}
	}
	return &object.Signature{Name: name, Email: email, When: time.Now()}
}

// Sync pulls (fast-forward) then pushes the "main" branch to origin using auth.
// "already up to date" and an empty/missing remote ref are treated as success.
func Sync(dir, remote string, auth config.Auth) error {
	r, err := git.PlainOpen(dir)
	if err != nil {
		return err
	}
	if err := setRemote(r, remote); err != nil {
		return err
	}
	am, err := authMethod(auth, remote)
	if err != nil {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}
	err = w.Pull(&git.PullOptions{
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName(mainBranch),
		Auth:          am,
	})
	if err != nil && !benignPull(err) {
		return err
	}

	err = r.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []gitconfig.RefSpec{gitconfig.RefSpec("refs/heads/main:refs/heads/main")},
		Auth:       am,
	})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

// benignPull reports whether a pull error should be ignored: already-up-to-date,
// or the remote has no matching ref yet (fresh remote / first sync).
func benignPull(err error) bool {
	return errors.Is(err, git.NoErrAlreadyUpToDate) ||
		errors.Is(err, transport.ErrEmptyRemoteRepository) ||
		errors.Is(err, git.NoMatchingRefSpecError{}) ||
		errors.Is(err, plumbing.ErrReferenceNotFound)
}

// authMethod maps duit's config.Auth to a go-git transport auth method. An
// empty or unknown method yields nil (no auth), which works for public/local
// remotes.
func authMethod(auth config.Auth, remote string) (transport.AuthMethod, error) {
	switch auth.Method {
	case "pat":
		return &httpauth.BasicAuth{Username: "x-access-token", Password: auth.Token}, nil
	case "ssh":
		if auth.SSHKey != "" {
			return sshauth.NewPublicKeysFromFile("git", auth.SSHKey, "")
		}
		return sshauth.NewSSHAgentAuth("git")
	default:
		return nil, nil
	}
}
