package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchBaseURLUsesCurrentProviderSection(t *testing.T) {
	t.Parallel()

	content := []byte(`model_provider = "rightcode"
-model_provider = "aigocode"

[model_providers.rightcode]
name = "rightcode"
base_url = "https://old.example.com/v1"
___base_url = "https://keep-me.example.com/v1"

[model_providers.aigocode]
name = "aigocode"
base_url = "https://other.example.com/v1"
`)

	provider, err := CurrentProvider(content)
	if err != nil {
		t.Fatalf("CurrentProvider() error = %v", err)
	}

	if provider != "rightcode" {
		t.Fatalf("provider = %q, want rightcode", provider)
	}

	next, changed, err := PatchBaseURL(content, provider, "https://new.example.com/v1")
	if err != nil {
		t.Fatalf("PatchBaseURL() error = %v", err)
	}

	if !changed {
		t.Fatalf("PatchBaseURL() changed = false, want true")
	}

	got := string(next)
	if !strings.Contains(got, `base_url = "https://new.example.com/v1"`) {
		t.Fatalf("patched content missing new base_url:\n%s", got)
	}

	if !strings.Contains(got, `___base_url = "https://keep-me.example.com/v1"`) {
		t.Fatalf("patched content should keep ___base_url:\n%s", got)
	}

	if !strings.Contains(got, `[model_providers.aigocode]
name = "aigocode"
base_url = "https://other.example.com/v1"`) {
		t.Fatalf("patched content should keep other provider section:\n%s", got)
	}
}

func TestPatchOpenAIAPIKeyReplacesOnlyTargetKey(t *testing.T) {
	t.Parallel()

	content := []byte(`{
  "micu_OPENAI_API_KEY": "keep",
  "OPENAI_API_KEY": null,
  "-OPENAI_API_KEY": "keep-too"
}`)

	next, changed, err := PatchOpenAIAPIKey(content, "new-key")
	if err != nil {
		t.Fatalf("PatchOpenAIAPIKey() error = %v", err)
	}

	if !changed {
		t.Fatalf("PatchOpenAIAPIKey() changed = false, want true")
	}

	got := string(next)
	if !strings.Contains(got, `"OPENAI_API_KEY": "new-key"`) {
		t.Fatalf("patched content missing updated OPENAI_API_KEY:\n%s", got)
	}

	if !strings.Contains(got, `"micu_OPENAI_API_KEY": "keep"`) {
		t.Fatalf("patched content should keep micu_OPENAI_API_KEY:\n%s", got)
	}

	if !strings.Contains(got, `"-OPENAI_API_KEY": "keep-too"`) {
		t.Fatalf("patched content should keep -OPENAI_API_KEY:\n%s", got)
	}
}

func TestApplyUpdatesFilesAndCreatesBackups(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	authPath := filepath.Join(tempDir, "auth.json")

	configContent := `model_provider = "rightcode"

[model_providers.rightcode]
base_url = "https://old.example.com/v1"
`
	authContent := `{
  "OPENAI_API_KEY": "old-key"
}`

	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	if err := os.WriteFile(authPath, []byte(authContent), 0o600); err != nil {
		t.Fatalf("write auth fixture: %v", err)
	}

	result, err := Apply(SwitchRequest{
		ConfigPath: configPath,
		AuthPath:   authPath,
		BaseURL:    "https://new.example.com/v1",
		APIKey:     "new-key",
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if result.Provider != "rightcode" {
		t.Fatalf("Provider = %q, want rightcode", result.Provider)
	}

	if result.ConfigBackup == "" || result.AuthBackup == "" {
		t.Fatalf("expected backup paths, got %#v", result)
	}

	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read updated config: %v", err)
	}

	authBytes, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatalf("read updated auth: %v", err)
	}

	if !strings.Contains(string(configBytes), `base_url = "https://new.example.com/v1"`) {
		t.Fatalf("updated config missing new base_url:\n%s", string(configBytes))
	}

	if !strings.Contains(string(authBytes), `"OPENAI_API_KEY": "new-key"`) {
		t.Fatalf("updated auth missing new key:\n%s", string(authBytes))
	}

	if _, err := os.Stat(result.ConfigBackup); err != nil {
		t.Fatalf("config backup missing: %v", err)
	}

	if _, err := os.Stat(result.AuthBackup); err != nil {
		t.Fatalf("auth backup missing: %v", err)
	}
}
