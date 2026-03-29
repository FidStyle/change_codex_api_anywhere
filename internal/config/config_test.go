package config

import (
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	cfg := New()
	cfg.CurrentProfile = "main"
	cfg.UpsertProfile(Profile{
		Name:        "main",
		Provider:    "vendor-a",
		Description: "primary route",
		BaseURL:     "https://example.com/v1",
		APIKey:      "sk-test",
	})

	if err := Save(configPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.CurrentProfile != "main" {
		t.Fatalf("CurrentProfile = %q, want %q", loaded.CurrentProfile, "main")
	}

	if len(loaded.Profiles) != 1 {
		t.Fatalf("len(Profiles) = %d, want 1", len(loaded.Profiles))
	}

	if loaded.Profiles[0].BaseURL != "https://example.com/v1" {
		t.Fatalf("BaseURL = %q, want expected value", loaded.Profiles[0].BaseURL)
	}
}
