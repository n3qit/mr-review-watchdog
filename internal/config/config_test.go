package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}
	return path
}

func TestLoad_Defaults(t *testing.T) {
	path := writeTempConfig(t, `
gitlab:
  base_url: "https://gitlab.example.com"
  token: "glpat-xxx"

mattermost:
  base_url: "https://mattermost.example.com"
  token: "mm-token"
  channel_id: "chan1"

repositories:
  - group/project-a
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Check.MinReviewers != DefaultMinReviewers {
		t.Errorf("MinReviewers = %d, want default %d", cfg.Check.MinReviewers, DefaultMinReviewers)
	}
	if cfg.Check.MinAgeHours != DefaultMinAgeHours {
		t.Errorf("MinAgeHours = %d, want default %d", cfg.Check.MinAgeHours, DefaultMinAgeHours)
	}
}

func TestLoad_CustomCheckValues(t *testing.T) {
	path := writeTempConfig(t, `
gitlab:
  base_url: "https://gitlab.example.com"
  token: "glpat-xxx"

mattermost:
  base_url: "https://mattermost.example.com"
  token: "mm-token"
  channel_id: "chan1"

check:
  min_reviewers: 3
  min_age_hours: 48

repositories:
  - group/project-a
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Check.MinReviewers != 3 {
		t.Errorf("MinReviewers = %d, want 3", cfg.Check.MinReviewers)
	}
	if cfg.Check.MinAgeHours != 48 {
		t.Errorf("MinAgeHours = %d, want 48", cfg.Check.MinAgeHours)
	}
}

func TestLoad_MissingRequiredFields(t *testing.T) {
	path := writeTempConfig(t, `
gitlab:
  base_url: "https://gitlab.example.com"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing required fields, got nil")
	}
}

func TestLoad_EmptyRepositories(t *testing.T) {
	path := writeTempConfig(t, `
gitlab:
  base_url: "https://gitlab.example.com"
  token: "glpat-xxx"

mattermost:
  base_url: "https://mattermost.example.com"
  token: "mm-token"
  channel_id: "chan1"

repositories: []
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty repositories, got nil")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}
