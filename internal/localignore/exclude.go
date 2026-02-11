package localignore

import (
	"os"
	"path/filepath"
	"strings"
)

func EnsurePattern(gitCommonDir, pattern string) error {
	excludePath := filepath.Join(gitCommonDir, "info", "exclude")

	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return err
	}

	if _, err := os.Stat(excludePath); os.IsNotExist(err) {
		content := pattern + "\n"
		return os.WriteFile(excludePath, []byte(content), 0o644)
	} else if err != nil {
		return err
	}

	data, err := os.ReadFile(excludePath)
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == pattern {
			return nil
		}
	}

	appendText := pattern + "\n"
	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(appendText)
	return err
}
