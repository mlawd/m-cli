package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/mlawd/m-cli/internal/state"
	"github.com/spf13/cobra"
)

func newStackWatchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Watch the progress of a running stack pipeline",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := discoverRepoContext()
			if err != nil {
				return err
			}

			stacksFile, err := loadState(repo)
			if err != nil {
				return err
			}

			stack, err := requireCurrentStackWithPlan(stacksFile, repo, stackNameFromFlag(cmd))
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()

			for {
				// Reload state each iteration
				stacksFile, err = state.LoadStacks(repo.rootPath)
				if err != nil {
					return err
				}

				stack, _ = state.FindStack(stacksFile, stack.Name)
				if stack == nil {
					return fmt.Errorf("stack no longer exists")
				}

				// Clear screen
				fmt.Fprint(w, "\033[2J\033[H")

				// Header
				displayName := formatStackDisplayName(*stack)
				fmt.Fprintf(w, "%s  %d stages\n\n", displayName, len(stack.Stages))

				allDone := true
				for i := range stack.Stages {
					s := &stack.Stages[i]
					status := state.EffectiveStatus(s)
					icon := statusIcon(status)
					elapsed := ""

					if status == state.StatusImplementing || status == state.StatusAIReview {
						allDone = false
						if s.StartedAt != "" {
							if t, err := time.Parse(time.RFC3339, s.StartedAt); err == nil {
								elapsed = "  " + formatDuration(time.Since(t))
							}
						}
					} else if status == state.StatusPending {
						allDone = false
					}

					fmt.Fprintf(w, "   %s  %-20s %-16s%s\n", icon, s.ID, status, elapsed)
				}

				fmt.Fprintln(w)

				if allDone {
					outSuccess(w, "All stages complete. Press ctrl-c to exit.")
					return nil
				}

				fmt.Fprintln(w, "Press ctrl-c to detach (stack continues in background)")

				time.Sleep(2 * time.Second)
			}
		},
	}
}

func statusIcon(status string) string {
	switch status {
	case state.StatusPending:
		return "\u00b7" // ·
	case state.StatusImplementing:
		return "\u28f8" // ⠸ (spinner char)
	case state.StatusAIReview:
		return "\u28fc" // ⠼ (spinner char)
	case state.StatusHumanReview:
		return "\u2713" // ✓
	case state.StatusDone:
		return "\u2713" // ✓
	default:
		return "\u2717" // ✗
	}
}

func formatDuration(d time.Duration) string {
	d = d.Truncate(time.Second)
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60

	if minutes > 0 {
		return fmt.Sprintf("%dm %02ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

func formatStackRunSummary(stack *state.Stack) string {
	if stack == nil || len(stack.Stages) == 0 {
		return ""
	}

	activeIdx := -1
	activeStatus := ""
	completedCount := 0

	for i := range stack.Stages {
		s := state.EffectiveStatus(&stack.Stages[i])
		if s == state.StatusHumanReview || s == state.StatusDone {
			completedCount++
		}
		if s == state.StatusImplementing || s == state.StatusAIReview {
			activeIdx = i
			activeStatus = s
		}
	}

	if activeIdx < 0 && completedCount == 0 {
		return ""
	}

	displayName := formatStackDisplayName(*stack)
	if activeIdx >= 0 {
		return fmt.Sprintf("stack: %s \u2014 stage %d/%d: %s", displayName, activeIdx+1, len(stack.Stages), activeStatus)
	}

	if completedCount == len(stack.Stages) {
		return fmt.Sprintf("stack: %s \u2014 all %d stages complete", displayName, len(stack.Stages))
	}

	return fmt.Sprintf("stack: %s \u2014 %d/%d stages reviewed", displayName, completedCount, len(stack.Stages))
}

func startStageWorktreeOnly(repo *repoContext, stacksFile *state.Stacks, stack *state.Stack, stageIndex int, branch, parentBranch string) error {
	if stack == nil || stageIndex < 0 || stageIndex >= len(stack.Stages) {
		return fmt.Errorf("invalid stage index")
	}

	target := &stack.Stages[stageIndex]

	if !gitxBranchExists(repo.rootPath, branch) {
		if err := gitxCreateBranch(repo.rootPath, branch, parentBranch); err != nil {
			return err
		}
	}

	worktree := strings.TrimSpace(target.Worktree)
	if worktree == "" {
		worktree = stageWorktreePath(repo.rootPath, stack.Name, target.ID)
	}

	if _, err := statPath(worktree); err != nil {
		if err := mkdirAll(worktree); err != nil {
			return err
		}
		if err := gitxAddWorktree(repo.rootPath, worktree, branch); err != nil {
			return err
		}
	}

	target.Branch = branch
	target.Worktree = worktree
	target.Parent = parentBranch

	return state.SaveStacks(repo.rootPath, stacksFile)
}
