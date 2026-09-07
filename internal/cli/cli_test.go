package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ccaa/internal/config"
)

func TestHelp(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"help", "add"}, {"help", "install"}, {"help", "openai"}} {
		var out, errs bytes.Buffer
		if code := Run(args, &out, &errs); code != 0 || !strings.Contains(out.String(), "Usage:") {
			t.Fatalf("%v: %d %s", args, code, errs.String())
		}
	}
}

func TestSwitchRoundTrip(t *testing.T) {
	for _, openAIArgs := range [][]string{{"openai"}, {"use", "openai"}, {"use", "-n", "openai"}, {"use", "opneai"}} {
		t.Run(strings.Join(openAIArgs, " "), func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "ccaa.toml")
			codexPath := filepath.Join(dir, "config.toml")
			authPath := filepath.Join(dir, "auth.json")
			auth := []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"untouched"}}`)
			if err := os.WriteFile(authPath, auth, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(codexPath, []byte("model_provider = 'vendor' # keep\n[model_providers.rightcode]\nname = 'keep'\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg := config.New()
			cfg.Codex.ConfigPath = codexPath
			if err := config.Save(configPath, cfg); err != nil {
				t.Fatal(err)
			}
			run := func(args ...string) string {
				t.Helper()
				var out, errs bytes.Buffer
				if code := Run(append([]string{"-c", configPath}, args...), &out, &errs); code != 0 {
					t.Fatalf("%v: %d %s", args, code, errs.String())
				}
				gotAuth, err := os.ReadFile(authPath)
				if err != nil || !bytes.Equal(gotAuth, auth) {
					t.Fatal("auth changed")
				}
				return out.String()
			}
			run("add", "-n", "main", "-p", "ignored", "-u", "https://example/v1", "-k", "test-token", "-d", "description")
			for i := 0; i < 2; i++ {
				run("use", "main")
				data, err := os.ReadFile(codexPath)
				if err != nil || !strings.Contains(string(data), "experimental_bearer_token = 'test-token'") || !strings.Contains(string(data), "model_provider = 'vendor' # keep") {
					t.Fatalf("bad switch: %v\n%s", err, data)
				}
				run(openAIArgs...)
				data, err = os.ReadFile(codexPath)
				if err != nil || strings.Contains(string(data), "base_url") || strings.Contains(string(data), "experimental_bearer_token") {
					t.Fatalf("bad clear: %v\n%s", err, data)
				}
				if got := run("current"); got != "openai\n" {
					t.Fatalf("current: %s", got)
				}
			}
			backups, err := filepath.Glob(authPath + ".backup.*")
			if err != nil || len(backups) != 0 {
				t.Fatal("auth backed up")
			}
		})
	}
}
