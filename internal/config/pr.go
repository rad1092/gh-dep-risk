package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const PRConfigFileName = ".gh-dep-risk.yml"

type PRConfig struct {
	ConfigPath string   `yaml:"-"`
	Lang       *string  `yaml:"lang"`
	FailLevel  *string  `yaml:"fail_level"`
	Comment    *bool    `yaml:"comment"`
	Paths      PathList `yaml:"path"`
	NoRegistry *bool    `yaml:"no_registry"`
}

type PathList struct {
	Values []string
	Set    bool
}

func LoadPRConfig(cwd string) (PRConfig, bool, error) {
	configPath := filepath.Join(cwd, PRConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return PRConfig{}, false, nil
		}
		return PRConfig{}, false, fmt.Errorf("read config %s: %w", configPath, err)
	}

	var cfg PRConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return PRConfig{}, true, fmt.Errorf("invalid config file %s: %w", configPath, err)
	}
	cfg.ConfigPath = configPath
	return cfg, true, nil
}

func (p *PathList) UnmarshalYAML(node *yaml.Node) error {
	p.Set = true
	switch node.Kind {
	case yaml.ScalarNode:
		var value string
		if err := node.Decode(&value); err != nil {
			return err
		}
		p.Values = []string{value}
		return nil
	case yaml.SequenceNode:
		values := make([]string, 0, len(node.Content))
		for _, item := range node.Content {
			var value string
			if err := item.Decode(&value); err != nil {
				return err
			}
			values = append(values, value)
		}
		p.Values = values
		return nil
	default:
		return fmt.Errorf("path must be a string or list of strings")
	}
}
