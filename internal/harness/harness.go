package harness

import (
	"context"
	"fmt"
	"strings"

	"github.com/mlawd/m-cli/internal/config"
)

type AgentOpts struct {
	WorktreePath string
	StageContext string
	StackName    string
	StageID      string
	Phase        string // "implementing" | "ai_review"
	SystemPrompt string
}

type Harness interface {
	SpawnBuildAgent(ctx context.Context, opts AgentOpts) error
	SpawnReviewAgent(ctx context.Context, opts AgentOpts) error
}

func ForConfig(cfg *config.Config) (Harness, error) {
	switch strings.TrimSpace(strings.ToLower(cfg.AgentHarness)) {
	case "opencode":
		return &OpenCodeHarness{Config: cfg}, nil
	case "claude":
		return &ClaudeHarness{Config: cfg}, nil
	default:
		return nil, fmt.Errorf("unsupported agent harness %q", cfg.AgentHarness)
	}
}
