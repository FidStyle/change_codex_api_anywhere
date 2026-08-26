package cli

import (
	"bytes"
	"os"
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

func TestRunOpenAISwitchesConfiguredCodexFiles(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	ccaaConfigPath := filepath.Join(tempDir, "ccaa.toml")
	codexConfigPath := filepath.Join(tempDir, "codex.toml")
	codexAuthPath := filepath.Join(tempDir, "auth.json")
	authSourcePath := filepath.Join(tempDir, "openai.auth.json")

	cfg := config.New()
	cfg.Codex.ConfigPath = codexConfigPath
	cfg.Codex.AuthPath = codexAuthPath
	if err := config.Save(ccaaConfigPath, cfg); err != nil {
		t.Fatalf("save ccaa config: %v", err)
	}

	codexConfig := `model_provider = "vendor"

[model_providers.vendor]
base_url = "https://keep.example.com/v1"`
	if err := os.WriteFile(codexConfigPath, []byte(codexConfig), 0o600); err != nil {
		t.Fatalf("write codex config: %v", err)
	}
	if err := os.WriteFile(codexAuthPath, []byte(`{"OPENAI_API_KEY":"old"}`), 0o600); err != nil {
		t.Fatalf("write codex auth: %v", err)
	}
	sourceAuth := `{"auth_mode":"chatgpt","OPENAI_API_KEY":null,"tokens":{"access_token":"fixture"}}`
	if err := os.WriteFile(authSourcePath, []byte(sourceAuth), 0o600); err != nil {
		t.Fatalf("write OpenAI auth source: %v", err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Run([]string{
		"-c", ccaaConfigPath,
		"openai",
		"-a", authSourcePath,
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("Run() exitCode = %d; stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "switched to OpenAI") {
		t.Fatalf("OpenAI switch output missing: %s", stdout.String())
	}

	updatedConfig, err := os.ReadFile(codexConfigPath)
	if err != nil {
		t.Fatalf("read updated codex config: %v", err)
	}
	wantConfig := strings.Replace(codexConfig, `model_provider = "vendor"`, `model_provider = "openai"`, 1)
	if string(updatedConfig) != wantConfig {
		t.Fatalf("OpenAI switch changed more than model_provider:\n%s", string(updatedConfig))
	}

	updatedAuth, err := os.ReadFile(codexAuthPath)
	if err != nil {
		t.Fatalf("read updated codex auth: %v", err)
	}
	if string(updatedAuth) != sourceAuth {
		t.Fatalf("OpenAI auth was not copied: %s", string(updatedAuth))
	}

	cfg, err = config.Load(ccaaConfigPath)
	if err != nil {
		t.Fatalf("reload ccaa config: %v", err)
	}
	cfg.UpsertProfile(config.Profile{
		Name:    "vendor",
		BaseURL: "https://keep.example.com/v1",
		APIKey:  "new-key",
	})
	if err := config.Save(ccaaConfigPath, cfg); err != nil {
		t.Fatalf("save vendor profile: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = Run([]string{"-c", ccaaConfigPath, "use", "vendor"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("Run() use exitCode = %d; stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}

	updatedConfig, err = os.ReadFile(codexConfigPath)
	if err != nil {
		t.Fatalf("read restored codex config: %v", err)
	}
	wantConfig = codexConfig
	if string(updatedConfig) != wantConfig {
		t.Fatalf("normal profile did not restore provider/base_url:\n%s", string(updatedConfig))
	}

	updatedAuth, err = os.ReadFile(codexAuthPath)
	if err != nil {
		t.Fatalf("read restored codex auth: %v", err)
	}
	wantAuth := `{"auth_mode":"chatgpt","OPENAI_API_KEY":"new-key","tokens":{"access_token":"fixture"}}`
	if string(updatedAuth) != wantAuth {
		t.Fatalf("normal profile did not update null API key while preserving auth:\n%s", string(updatedAuth))
	}
}
