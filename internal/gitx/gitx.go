package gitx

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type RepoInfo struct {
	TopLevel      string
	CommonDir     string
	DefaultBranch string
}

func SharedRoot(topLevel, commonDir string) string {
	common := strings.TrimSpace(commonDir)
	if common != "" {
		common = filepath.Clean(common)
		if filepath.Base(common) == ".git" {
			return filepath.Dir(common)
		}

		return common
	}

	return strings.TrimSpace(topLevel)
}

func Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return out, fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}

	return out, nil
}

func DiscoverRepo(startDir string) (*RepoInfo, error) {
	common, err := Run(startDir, "rev-parse", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(common) {
		base, baseErr := Run(startDir, "rev-parse", "--show-toplevel")
		if baseErr != nil {
			base, baseErr = Run(startDir, "rev-parse", "--absolute-git-dir")
			if baseErr != nil {
				return nil, baseErr
			}
		}
		common = filepath.Clean(filepath.Join(base, common))
	}

	top, err := Run(startDir, "rev-parse", "--show-toplevel")
	if err != nil {
		top = SharedRoot("", common)
	}

	defaultBranch, err := DetectDefaultBranch(startDir)
	if err != nil {
		return nil, err
	}

	return &RepoInfo{
		TopLevel:      top,
		CommonDir:     common,
		DefaultBranch: defaultBranch,
	}, nil
}

func DetectDefaultBranch(dir string) (string, error) {
	if symRef, err := Run(dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return strings.TrimPrefix(symRef, "origin/"), nil
	}

	if _, err := Run(dir, "show-ref", "--verify", "--quiet", "refs/heads/main"); err == nil {
		return "main", nil
	}

	if _, err := Run(dir, "show-ref", "--verify", "--quiet", "refs/heads/master"); err == nil {
		return "master", nil
	}

	branch, err := Run(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}

	return branch, nil
}

func BranchExists(dir, branch string) bool {
	_, err := Run(dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

func RemoteBranchExists(dir, remote, branch string) bool {
	remote = strings.TrimSpace(remote)
	branch = strings.TrimSpace(branch)
	if remote == "" || branch == "" {
		return false
	}

	_, err := Run(dir, "ls-remote", "--exit-code", "--heads", remote, "refs/heads/"+branch)
	return err == nil
}

func CreateBranch(dir, branch, from string) error {
	_, err := Run(dir, "branch", branch, from)
	return err
}

func AddWorktree(dir, path, branch string) error {
	_, err := Run(dir, "worktree", "add", path, branch)
	return err
}

func AddWorktreeFromRemote(dir, path, localBranch, remoteBranch string) error {
	_, err := Run(dir, "worktree", "add", "-b", localBranch, path, remoteBranch)
	return err
}
