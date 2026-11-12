package compiler

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LangConfig is both the on-disk shape (lang/*.yaml) and the admin API body.
//
// The JSON tags are not decoration. The admin handler decoded the request body
// straight into this struct, and with no json tags encoding/json matched keys
// to field names case-insensitively — so "compile" never matched CompileCmd and
// "copy_in" never matched CopyIn. Every form save silently wrote an empty
// compile command back over the working one, which is why the field appeared to
// vanish. The tags must stay in step with LanguageConfigResponse in
// internal/api/handler/admin_languages.go.
type LangConfig struct {
	Name                  string            `yaml:"name" json:"name"`
	Key                   string            `yaml:"key" json:"key"`
	CompileCmd            string            `yaml:"compile" json:"compile"`
	CopyIn                map[string]string `yaml:"copy_in" json:"copy_in"`
	TimeLimitMultiplier   float64           `yaml:"time_limit_multiplier" json:"time_limit_multiplier"`
	MemoryLimitMultiplier float64           `yaml:"memory_limit_multiplier" json:"memory_limit_multiplier"`
	SeccompRule           string            `yaml:"seccomp_rule" json:"seccomp_rule"`
	Extensions            []string          `yaml:"extensions" json:"extensions"`
	Mono                  bool              `yaml:"mono,omitempty" json:"mono"`
	Runtime               string            `yaml:"runtime,omitempty" json:"runtime"`
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
