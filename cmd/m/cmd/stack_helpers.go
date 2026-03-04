package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mlawd/m-cli/internal/config"
	"github.com/mlawd/m-cli/internal/gitx"
	"github.com/mlawd/m-cli/internal/state"
)

func stageWorktreePath(repoRoot, stackName, stageID string) string {
	return filepath.Join(state.StacksDir(repoRoot), filepath.FromSlash(stackName), filepath.FromSlash(stageID))
}

func gitxBranchExists(repoRoot, branch string) bool {
	return gitx.BranchExists(repoRoot, branch)
}

func gitxCreateBranch(repoRoot, branch, from string) error {
	return gitx.CreateBranch(repoRoot, branch, from)
}

func gitxAddWorktree(repoRoot, path, branch string) error {
	return gitx.AddWorktree(repoRoot, path, branch)
}

func statPath(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func mkdirAll(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

func ensureAgentDefinitions(repoRoot string, cfg *config.Config) error {
	harnessName := strings.ToLower(cfg.AgentHarness)
	return writeAgentDefinitions(repoRoot, harnessName)
}
