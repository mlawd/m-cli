package gitx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyRootDotEnvFilesCopiesEnvFiles(t *testing.T) {
	rootDir := t.TempDir()
	worktreeDir := t.TempDir()

	mustWriteFile(t, filepath.Join(rootDir, ".env"), "ROOT=1")
	mustWriteFile(t, filepath.Join(rootDir, ".env.local"), "LOCAL=1")
	mustWriteFile(t, filepath.Join(rootDir, ".envrc"), "export ROOT=1")
	mustWriteFile(t, filepath.Join(rootDir, "not-env"), "skip")

	if err := os.Mkdir(filepath.Join(rootDir, ".env-dir"), 0o755); err != nil {
		t.Fatalf("mkdir .env-dir: %v", err)
	}

	if err := copyRootDotEnvFiles(rootDir, worktreeDir); err != nil {
		t.Fatalf("copyRootDotEnvFiles: %v", err)
	}

	mustHaveFileWithContents(t, filepath.Join(worktreeDir, ".env"), "ROOT=1")
	mustHaveFileWithContents(t, filepath.Join(worktreeDir, ".env.local"), "LOCAL=1")
	mustNotExist(t, filepath.Join(worktreeDir, ".envrc"))
	mustNotExist(t, filepath.Join(worktreeDir, "not-env"))
	mustNotExist(t, filepath.Join(worktreeDir, ".env-dir"))
}

func TestIsDotEnvFile(t *testing.T) {
	tests := []struct {
		name string
		file string
		want bool
	}{
		{name: "base env", file: ".env", want: true},
		{name: "env suffix", file: ".env.local", want: true},
		{name: "envrc", file: ".envrc", want: false},
		{name: "non env", file: "README.md", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDotEnvFile(tc.file); got != tc.want {
				t.Fatalf("isDotEnvFile(%q) = %v, want %v", tc.file, got, tc.want)
			}
		})
	}
}

func TestCopyRootDotEnvFilesDoesNotOverwrite(t *testing.T) {
	rootDir := t.TempDir()
	worktreeDir := t.TempDir()

	mustWriteFile(t, filepath.Join(rootDir, ".env"), "ROOT=1")
	mustWriteFile(t, filepath.Join(worktreeDir, ".env"), "WORKTREE=1")

	if err := copyRootDotEnvFiles(rootDir, worktreeDir); err != nil {
		t.Fatalf("copyRootDotEnvFiles: %v", err)
	}

	mustHaveFileWithContents(t, filepath.Join(worktreeDir, ".env"), "WORKTREE=1")
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustHaveFileWithContents(t *testing.T, path, expected string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(contents) != expected {
		t.Fatalf("unexpected contents for %s: got %q want %q", path, string(contents), expected)
	}
}

func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s not to exist, got err=%v", path, err)
	}
}
