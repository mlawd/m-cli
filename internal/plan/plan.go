package plan

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

var stageIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type File struct {
	Version int         `yaml:"version"`
	Title   string      `yaml:"title"`
	Stages  []FileStage `yaml:"stages"`
}

type FileStage struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
}

func ParseFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var parsed File
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parse plan YAML: %w", err)
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
	if p.Version != 1 {
		return fmt.Errorf("plan version must be 1")
	}
	if len(p.Stages) == 0 {
		return fmt.Errorf("plan must include at least one stage")
	}

	seen := make(map[string]struct{}, len(p.Stages))
	for i, stage := range p.Stages {
		if stage.ID == "" {
			return fmt.Errorf("stage %d is missing id", i+1)
		}
		if !stageIDPattern.MatchString(stage.ID) {
			return fmt.Errorf("stage %q has invalid id; use kebab-case letters/numbers", stage.ID)
		}
		if stage.Title == "" {
			return fmt.Errorf("stage %q is missing title", stage.ID)
		}
		if _, exists := seen[stage.ID]; exists {
			return fmt.Errorf("duplicate stage id %q", stage.ID)
		}
		seen[stage.ID] = struct{}{}
	}

	return nil
}
