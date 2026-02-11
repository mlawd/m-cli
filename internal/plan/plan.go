package plan

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var stageIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type File struct {
	Version int         `yaml:"version"`
	Title   string      `yaml:"title"`
	Stages  []FileStage `yaml:"stages"`
}

type FileStage struct {
	ID             string     `yaml:"id"`
	Title          string     `yaml:"title"`
	Outcome        string     `yaml:"outcome"`
	Implementation []string   `yaml:"implementation"`
	Validation     []string   `yaml:"validation"`
	Risks          []FileRisk `yaml:"risks"`
}

type FileRisk struct {
	Risk       string `yaml:"risk"`
	Mitigation string `yaml:"mitigation"`
}

func ParseFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	frontmatter, err := extractFrontmatter(string(data))
	if err != nil {
		return nil, err
	}

	var parsed File
	if err := yaml.Unmarshal([]byte(frontmatter), &parsed); err != nil {
		return nil, fmt.Errorf("parse plan frontmatter: %w", err)
	}

	if err := Validate(&parsed); err != nil {
		return nil, err
	}

	return &parsed, nil
}

func Validate(p *File) error {
	if p == nil {
		return fmt.Errorf("plan is empty")
	}
	if p.Version != 2 {
		return fmt.Errorf("plan version must be 2")
	}
	if len(p.Stages) == 0 {
		return fmt.Errorf("plan must include at least one stage")
	}

	seen := make(map[string]struct{}, len(p.Stages))
	for i, stage := range p.Stages {
		stageID := strings.TrimSpace(stage.ID)
		if stageID == "" {
			return fmt.Errorf("stage %d is missing id", i+1)
		}
		if !stageIDPattern.MatchString(stageID) {
			return fmt.Errorf("stage %q has invalid id; use kebab-case letters/numbers", stageID)
		}
		if strings.TrimSpace(stage.Title) == "" {
			return fmt.Errorf("stage %q is missing title", stageID)
		}
		if strings.TrimSpace(stage.Outcome) == "" {
			return fmt.Errorf("stage %q is missing outcome", stageID)
		}
		if err := validateStringList(stage.Implementation); err != nil {
			return fmt.Errorf("stage %q has invalid implementation list: %w", stageID, err)
		}
		if err := validateStringList(stage.Validation); err != nil {
			return fmt.Errorf("stage %q has invalid validation list: %w", stageID, err)
		}
		if len(stage.Risks) == 0 {
			return fmt.Errorf("stage %q must include at least one risk", stageID)
		}
		for idx, risk := range stage.Risks {
			if strings.TrimSpace(risk.Risk) == "" {
				return fmt.Errorf("stage %q risk %d is missing risk", stageID, idx+1)
			}
			if strings.TrimSpace(risk.Mitigation) == "" {
				return fmt.Errorf("stage %q risk %d is missing mitigation", stageID, idx+1)
			}
		}
		if _, exists := seen[stageID]; exists {
			return fmt.Errorf("duplicate stage id %q", stageID)
		}
		seen[stageID] = struct{}{}
	}

	return nil
}

func extractFrontmatter(raw string) (string, error) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return "", fmt.Errorf("plan file must start with YAML frontmatter delimited by ---")
	}

	remaining := normalized[len("---\n"):]
	idx := strings.Index(remaining, "\n---\n")
	if idx == -1 {
		if strings.HasSuffix(remaining, "\n---") {
			idx = len(remaining) - len("\n---")
		} else {
			return "", fmt.Errorf("plan file is missing closing frontmatter delimiter ---")
		}
	}

	frontmatter := strings.TrimSpace(remaining[:idx])
	if frontmatter == "" {
		return "", fmt.Errorf("plan frontmatter is empty")
	}

	return frontmatter, nil
}

func validateStringList(items []string) error {
	if len(items) == 0 {
		return fmt.Errorf("must include at least one item")
	}

	for idx, item := range items {
		if strings.TrimSpace(item) == "" {
			return fmt.Errorf("item %d is empty", idx+1)
		}
	}

	return nil
}
