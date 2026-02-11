package paths

import (
	"fmt"
	"path/filepath"
	"strings"
)

func EnsureValidStackName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("stack name cannot be empty")
	}
	if strings.Contains(trimmed, " ") {
		return fmt.Errorf("stack name cannot contain spaces")
	}
	if strings.HasPrefix(trimmed, "/") || strings.HasSuffix(trimmed, "/") {
		return fmt.Errorf("stack name cannot start or end with /")
	}
	if strings.Contains(trimmed, "//") {
		return fmt.Errorf("stack name cannot contain empty path segments")
	}

	return nil
}

func RepoNameFromURL(repoURL string) string {
	name := repoURL
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.TrimSuffix(name, ".git")
	name = strings.TrimSpace(name)
	if name == "" {
		return "repo"
	}

	return filepath.Clean(name)
}
