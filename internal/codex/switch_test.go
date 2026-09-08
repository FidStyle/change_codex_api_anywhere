package codex

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

func TestPatchCredentials(t *testing.T) {
	for _, fixture := range []string{
		"",
		"model = 'keep'\n",
		"[model_providers]\n[model_providers.other]\nbase_url = 'keep'\n",
		"model_provider = 'vendor' # keep\n[model_providers.vendor] # keep\nname = 'Vendor'\nbase_url = 'old' # URL\nexperimental_bearer_token = 'old' # token\n[other]\nx = 1\n",
		"model_provider = 'vendor'\r\n[model_providers.vendor]\r\n# base_url = 'disabled'\r\nbase_url = 'old'\r\n",
		"[model_providers.'openai']",
		"[model_providers.openai]\n'base_url' = '''old\nurl'''\nexperimental_bearer_token = 'old'",
		"prompt = '''\n[model_providers.openai]\nbase_url = 'not a setting'\n'''\n",
	} {
		t.Run(fixture, func(t *testing.T) {
			url, token := "https://new.example/v1", "token\"\\\nvalue"
			next, provider, err := patchCredentials([]byte(fixture), url, token, false)
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				Provider  string `toml:"model_provider"`
				Providers map[string]struct {
					URL   string `toml:"base_url"`
					Token string `toml:"experimental_bearer_token"`
				} `toml:"model_providers"`
			}
			if err := toml.Unmarshal(next, &got); err != nil {
				t.Fatal(err)
			}
			if got.Provider != provider || got.Providers[provider].URL != url || got.Providers[provider].Token != token {
				t.Fatalf("wrong credentials: %s", next)
			}
			again, _, err := patchCredentials(next, url, token, false)
			if err != nil || !bytes.Equal(next, again) {
				t.Fatalf("not idempotent: %v\n%s", err, again)
			}
			cleared, _, err := patchCredentials(next, "", "", true)
			if err != nil {
				t.Fatal(err)
			}
			again, _, err = patchCredentials(cleared, "", "", true)
			if err != nil || !bytes.Equal(cleared, again) {
				t.Fatalf("clear not idempotent: %v", err)
			}
			if strings.Contains(fixture, "\r\n") && bytes.Contains(bytes.ReplaceAll(next, []byte("\r\n"), nil), []byte("\n")) {
				t.Fatal("mixed line endings")
			}
		})
	}
}

func TestRightcodeCredentialsChange(t *testing.T) {
	before := "model_provider = 'vendor' # unchanged\nmodel = 'keep'\n[model_providers.rightcode]\nbase_url = 'old' # URL\nexperimental_bearer_token = 'old'\nname = 'keep'\n[model_providers.other]\nbase_url = 'untouched'\nexperimental_bearer_token = 'untouched'\n"
	next, _, err := patchCredentials([]byte(before), "new-url", "new-token", false)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(before, "model_provider = 'vendor'", `model_provider = "rightcode"`, 1)
	want = strings.Replace(want, "base_url = 'old'", "base_url = 'new-url'", 1)
	want = strings.Replace(want, "experimental_bearer_token = 'old'", "experimental_bearer_token = 'new-token'", 1)
	if string(next) != want {
		t.Fatalf("unexpected patch:\n%s", next)
	}
	cleared, _, err := patchCredentials(next, "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	want = strings.Replace(want, "base_url = 'new-url' # URL\n", "", 1)
	want = strings.Replace(want, "experimental_bearer_token = 'new-token'\n", "", 1)
	if string(cleared) != want {
		t.Fatalf("unexpected clear:\n%s", cleared)
	}
}

func TestApplyBackupsAndNoAuthDependency(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	before := []byte("model_provider = 'openai'\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Apply(SwitchRequest{ConfigPath: path, BaseURL: "https://example/v1", APIKey: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(result.ConfigBackup)
	if err != nil || !bytes.Equal(backup, before) {
		t.Fatalf("bad backup: %v", err)
	}
	again, err := Apply(SwitchRequest{ConfigPath: path, BaseURL: "https://example/v1", APIKey: "secret"})
	if err != nil || again.ConfigChanged || again.ConfigBackup != "" {
		t.Fatalf("repeat apply: %#v %v", again, err)
	}
	cleared, err := ApplyOpenAI(OpenAIRequest{ConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.ConfigBackup == result.ConfigBackup {
		t.Fatal("backup overwritten")
	}
	if _, err := os.Stat(filepath.Join(dir, "auth.json")); !os.IsNotExist(err) {
		t.Fatal("created auth.json")
	}
}

func TestInvalidConfigIsNotWritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := []byte("broken = [")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(SwitchRequest{ConfigPath: path, BaseURL: "url", APIKey: "token"}); err == nil {
		t.Fatal("expected error")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("invalid config modified")
	}
}
