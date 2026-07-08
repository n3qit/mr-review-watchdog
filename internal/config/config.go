// Package config определяет структуру конфигурации mr-review-watchdog,
// загрузку из YAML-файла и валидацию обязательных полей.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	DefaultMinReviewers = 2
	DefaultMinAgeHours  = 24
)

type Config struct {
	GitLab       GitLabConfig     `yaml:"gitlab"`
	Mattermost   MattermostConfig `yaml:"mattermost"`
	Check        CheckConfig      `yaml:"check"`
	Repositories []string         `yaml:"repositories"`
}

type GitLabConfig struct {
	BaseURL string `yaml:"base_url"`
	Token   string `yaml:"token"`
}

type MattermostConfig struct {
	BaseURL   string `yaml:"base_url"`
	Token     string `yaml:"token"`
	ChannelID string `yaml:"channel_id"`
}

type CheckConfig struct {
	MinReviewers int `yaml:"min_reviewers"`
	MinAgeHours  int `yaml:"min_age_hours"`
}

// Load читает конфигурацию из YAML-файла по указанному пути,
// применяет значения по умолчанию и валидирует обязательные поля.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("чтение конфигурационного файла %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("разбор YAML в файле %q: %w", path, err)
	}

	cfg.applyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Check.MinReviewers == 0 {
		c.Check.MinReviewers = DefaultMinReviewers
	}
	if c.Check.MinAgeHours == 0 {
		c.Check.MinAgeHours = DefaultMinAgeHours
	}
}

// Validate проверяет наличие обязательных полей конфигурации.
func (c *Config) Validate() error {
	var missing []string

	if c.GitLab.BaseURL == "" {
		missing = append(missing, "gitlab.base_url")
	}
	if c.GitLab.Token == "" {
		missing = append(missing, "gitlab.token")
	}
	if c.Mattermost.BaseURL == "" {
		missing = append(missing, "mattermost.base_url")
	}
	if c.Mattermost.Token == "" {
		missing = append(missing, "mattermost.token")
	}
	if c.Mattermost.ChannelID == "" {
		missing = append(missing, "mattermost.channel_id")
	}
	if len(c.Repositories) == 0 {
		missing = append(missing, "repositories")
	}

	if len(missing) > 0 {
		return fmt.Errorf("невалидная конфигурация: отсутствуют обязательные поля: %v", missing)
	}

	return nil
}
