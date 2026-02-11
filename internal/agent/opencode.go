package agent

import (
	"fmt"
	"os"
	"os/exec"
)

func StartOpenCode(dir string) error {
	return StartOpenCodeWithArgs(dir)
}

func StartOpenCodeWithArgs(dir string, args ...string) error {
	path, err := exec.LookPath("opencode")
	if err != nil {
		return fmt.Errorf("opencode not found in PATH")
	}

	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run opencode: %w", err)
	}

	return nil
}
