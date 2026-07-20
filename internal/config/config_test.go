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

func TestLoad_TeamGroupOptional(t *testing.T) {
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

	if cfg.GitLab.TeamGroup != "" {
		t.Errorf("TeamGroup = %q, want empty when not set", cfg.GitLab.TeamGroup)
	}
}

func TestLoad_TeamGroupSet(t *testing.T) {
	path := writeTempConfig(t, `
gitlab:
  base_url: "https://gitlab.example.com"
  token: "glpat-xxx"
  team_group: "group/our-team"

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

	if cfg.GitLab.TeamGroup != "group/our-team" {
		t.Errorf("TeamGroup = %q, want %q", cfg.GitLab.TeamGroup, "group/our-team")
	}
}

func TestLoad_CalendarBaseURLOptional(t *testing.T) {
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

	if cfg.Calendar.BaseURL != "" {
		t.Errorf("Calendar.BaseURL = %q, want empty when not set", cfg.Calendar.BaseURL)
	}
}

func TestLoad_CalendarBaseURLSet(t *testing.T) {
	path := writeTempConfig(t, `
gitlab:
  base_url: "https://gitlab.example.com"
  token: "glpat-xxx"

mattermost:
  base_url: "https://mattermost.example.com"
  token: "mm-token"
  channel_id: "chan1"

calendar:
  base_url: "https://calendar.internal.example.com"

repositories:
  - group/project-a
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Calendar.BaseURL != "https://calendar.internal.example.com" {
		t.Errorf("Calendar.BaseURL = %q, want %q", cfg.Calendar.BaseURL, "https://calendar.internal.example.com")
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
