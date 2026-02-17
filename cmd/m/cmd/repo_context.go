package cmd

import (
	"fmt"

	"github.com/mlawd/m-cli/internal/gitx"
	"github.com/mlawd/m-cli/internal/state"
)

type repoContext struct {
	rootPath     string
	common       string
	worktreePath string
}

func discoverRepoContext() (*repoContext, error) {
	repo, err := gitx.DiscoverRepo(".")
	if err != nil {
		return nil, fmt.Errorf("discover repo: %w", err)
	}

	return &repoContext{
		rootPath:     gitx.SharedRoot(repo.TopLevel, repo.CommonDir),
		common:       repo.CommonDir,
		worktreePath: repo.TopLevel,
	}, nil
}

func loadState(ctx *repoContext) (*state.Stacks, error) {
	if err := state.EnsureInitialized(ctx.rootPath); err != nil {
		return nil, err
	}

	stacks, err := state.LoadStacks(ctx.rootPath)
	if err != nil {
		return nil, err
	}

	return stacks, nil
}
