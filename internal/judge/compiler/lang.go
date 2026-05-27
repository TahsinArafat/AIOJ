package compiler

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type LangConfig struct {
	Name                  string            `yaml:"name"`
	Key                   string            `yaml:"key"`
	CompileCmd            string            `yaml:"compile"`
	CopyIn                map[string]string `yaml:"copy_in"`
	TimeLimitMultiplier   float64           `yaml:"time_limit_multiplier"`
	MemoryLimitMultiplier float64           `yaml:"memory_limit_multiplier"`
	SeccompRule           string            `yaml:"seccomp_rule"`
	Extensions            []string          `yaml:"extensions"`
	Mono                  bool              `yaml:"mono,omitempty"`
	Runtime               string            `yaml:"runtime,omitempty"`
}

func LoadLanguages(dir string) (map[string]*LangConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read lang dir: %w", err)
	}
	langs := make(map[string]*LangConfig)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var cfg LangConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			continue
		}
		if cfg.Key == "" {
			continue
		}
		langs[cfg.Key] = &cfg
	}
	return langs, nil
}
