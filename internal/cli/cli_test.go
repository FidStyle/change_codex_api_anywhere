package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"ccaa/internal/config"
)

func TestRunShowsRootHelpWithDashH(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"-h"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("Run() exitCode = %d, want 0; stderr=%s", exitCode, stderr.String())
	}

	if !strings.Contains(stdout.String(), "ccaa help [command]") {
		t.Fatalf("root help missing dedicated help usage:\n%s", stdout.String())
	}
}

func TestRunHelpAddShowsShortFlags(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"help", "add"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("Run() exitCode = %d, want 0; stderr=%s", exitCode, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "-n, --name") || !strings.Contains(output, "-u, --base-url") || !strings.Contains(output, "-k, --api-key") {
		t.Fatalf("add help missing short flags:\n%s", output)
	}
}

func TestRunHelpInstallShowsUsage(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"help", "install"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("Run() exitCode = %d, want 0; stderr=%s", exitCode, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "ccaa install") || !strings.Contains(output, "Linux: prefer a writable system PATH directory") {
		t.Fatalf("install help missing expected text:\n%s", output)
	}
}

func TestRunAddSupportsShortFlags(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{
		"-c", configPath,
		"add",
		"-n", "main",
		"-u", "https://example.com/v1",
		"-k", "sk-test",
		"-p", "vendor-a",
		"-d", "primary route",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("Run() exitCode = %d, want 0; stderr=%s", exitCode, stderr.String())
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	profile, _ := cfg.FindProfile("main")
	if profile == nil {
		t.Fatalf("profile main not found in saved config")
	}

	if profile.BaseURL != "https://example.com/v1" {
		t.Fatalf("BaseURL = %q, want expected value", profile.BaseURL)
	}

	if profile.APIKey != "sk-test" {
		t.Fatalf("APIKey = %q, want expected value", profile.APIKey)
	}
}
