package codex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type SwitchRequest struct {
	ConfigPath string
	AuthPath   string
	BaseURL    string
	APIKey     string
}

type OpenAIRequest struct {
	ConfigPath     string
	AuthPath       string
	AuthSourcePath string
}

type SwitchResult struct {
	Provider      string
	ConfigChanged bool
	AuthChanged   bool
	ConfigBackup  string
	AuthBackup    string
}

var (
	modelProviderPattern = regexp.MustCompile(`^model_provider\s*=\s*"([^"]+)"\s*$`)
	openAIKeyPattern     = regexp.MustCompile(`("OPENAI_API_KEY"\s*:\s*)("(?:\\.|[^"\\])*")`)
	baseURLLinePattern   = regexp.MustCompile(`^(\s*)base_url\s*=`)
)

func Apply(req SwitchRequest) (*SwitchResult, error) {
	configData, configMode, err := readFileWithMode(req.ConfigPath)
	if err != nil {
		return nil, err
	}

	authData, authMode, err := readFileWithMode(req.AuthPath)
	if err != nil {
		return nil, err
	}

	provider, err := CurrentProvider(configData)
	if err != nil {
		return nil, fmt.Errorf("detect model_provider from %s: %w", req.ConfigPath, err)
	}

	nextConfig, configChanged, err := PatchBaseURL(configData, provider, req.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("patch base_url in %s: %w", req.ConfigPath, err)
	}

	nextAuth, authChanged, err := PatchOpenAIAPIKey(authData, req.APIKey)
	if err != nil {
		return nil, fmt.Errorf("patch OPENAI_API_KEY in %s: %w", req.AuthPath, err)
	}

	result := &SwitchResult{Provider: provider, ConfigChanged: configChanged, AuthChanged: authChanged}

	if configChanged {
		result.ConfigBackup, err = writeChangedFile(req.ConfigPath, configData, nextConfig, configMode)
		if err != nil {
			return nil, err
		}
	}

	if authChanged {
		result.AuthBackup, err = writeChangedFile(req.AuthPath, authData, nextAuth, authMode)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func ApplyOpenAI(req OpenAIRequest) (*SwitchResult, error) {
	configData, configMode, err := readFileWithMode(req.ConfigPath)
	if err != nil {
		return nil, err
	}

	authData, authMode, err := readFileWithMode(req.AuthPath)
	if err != nil {
		return nil, err
	}

	sourceAuthData, _, err := readFileWithMode(req.AuthSourcePath)
	if err != nil {
		return nil, err
	}

	if !json.Valid(bytes.TrimSpace(sourceAuthData)) {
		return nil, fmt.Errorf("validate auth source %s: invalid JSON", req.AuthSourcePath)
	}

	nextConfig, configChanged, err := PatchModelProvider(configData, "openai")
	if err != nil {
		return nil, fmt.Errorf("patch model_provider in %s: %w", req.ConfigPath, err)
	}

	authChanged := !bytes.Equal(authData, sourceAuthData)
	result := &SwitchResult{Provider: "openai", ConfigChanged: configChanged, AuthChanged: authChanged}

	if configChanged {
		result.ConfigBackup, err = writeChangedFile(req.ConfigPath, configData, nextConfig, configMode)
		if err != nil {
			return nil, err
		}
	}

	if authChanged {
		result.AuthBackup, err = writeChangedFile(req.AuthPath, authData, sourceAuthData, authMode)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func CurrentProvider(content []byte) (string, error) {
	lines := strings.Split(normalizeNewlines(string(content)), "\n")
	for _, line := range lines {
		matches := modelProviderPattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) == 2 {
			return matches[1], nil
		}
	}

	return "", fmt.Errorf("model_provider not found")
}

func PatchModelProvider(content []byte, provider string) ([]byte, bool, error) {
	if strings.TrimSpace(provider) == "" {
		return nil, false, fmt.Errorf("provider cannot be empty")
	}

	quotedProvider, err := json.Marshal(provider)
	if err != nil {
		return nil, false, fmt.Errorf("encode provider: %w", err)
	}

	newline := detectNewline(content)
	lines := strings.Split(normalizeNewlines(string(content)), "\n")
	for i, rawLine := range lines {
		if len(modelProviderPattern.FindStringSubmatch(strings.TrimSpace(rawLine))) != 2 {
			continue
		}

		indent := rawLine[:len(rawLine)-len(strings.TrimLeft(rawLine, " \t"))]
		replacement := indent + "model_provider = " + string(quotedProvider)
		if rawLine == replacement {
			return content, false, nil
		}

		lines[i] = replacement
		return []byte(strings.Join(lines, newline)), true, nil
	}

	return nil, false, fmt.Errorf("model_provider not found")
}

func writeChangedFile(path string, before []byte, after []byte, mode os.FileMode) (string, error) {
	backupPath, err := backupFile(path, before)
	if err != nil {
		return "", err
	}

	if err := writeFileAtomic(path, after, mode); err != nil {
		return "", err
	}

	return backupPath, nil
}

func PatchBaseURL(content []byte, provider string, baseURL string) ([]byte, bool, error) {
	if strings.TrimSpace(provider) == "" {
		return nil, false, fmt.Errorf("provider cannot be empty")
	}

	newline := detectNewline(content)
	lines := strings.Split(normalizeNewlines(string(content)), "\n")
	sectionHeader := "[model_providers." + provider + "]"
	sectionStart := -1
	sectionEnd := len(lines)

	for i, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == sectionHeader {
			sectionStart = i
			continue
		}

		if sectionStart >= 0 && strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sectionEnd = i
			break
		}
	}

	if sectionStart < 0 {
		return nil, false, fmt.Errorf("section %s not found", sectionHeader)
	}

	baseURLLineIndex := -1
	baseURLIndent := ""
	for i := sectionStart + 1; i < sectionEnd; i++ {
		matches := baseURLLinePattern.FindStringSubmatch(lines[i])
		if len(matches) == 2 {
			baseURLLineIndex = i
			baseURLIndent = matches[1]
			break
		}
	}

	quotedBaseURL, err := json.Marshal(baseURL)
	if err != nil {
		return nil, false, fmt.Errorf("encode base_url: %w", err)
	}

	replacement := baseURLIndent + "base_url = " + string(quotedBaseURL)
	if baseURLLineIndex >= 0 {
		if lines[baseURLLineIndex] == replacement {
			return content, false, nil
		}

		lines[baseURLLineIndex] = replacement
		return []byte(strings.Join(lines, newline)), true, nil
	}

	insertAt := sectionStart + 1
	lines = append(lines[:insertAt], append([]string{replacement}, lines[insertAt:]...)...)
	return []byte(strings.Join(lines, newline)), true, nil
}

func PatchOpenAIAPIKey(content []byte, apiKey string) ([]byte, bool, error) {
	quotedAPIKey, err := json.Marshal(apiKey)
	if err != nil {
		return nil, false, fmt.Errorf("encode api_key: %w", err)
	}

	if openAIKeyPattern.Match(content) {
		next := openAIKeyPattern.ReplaceAll(content, []byte(`${1}`+string(quotedAPIKey)))
		if bytes.Equal(next, content) {
			return content, false, nil
		}

		return next, true, nil
	}

	trimmed := bytes.TrimSpace(content)
	if bytes.Equal(trimmed, []byte("{}")) {
		newline := detectNewline(content)
		next := []byte("{" + newline + `  "OPENAI_API_KEY": ` + string(quotedAPIKey) + newline + "}")
		return next, true, nil
	}

	return nil, false, fmt.Errorf("OPENAI_API_KEY not found")
}

func readFileWithMode(path string) ([]byte, os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, fmt.Errorf("stat %s: %w", path, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("read %s: %w", path, err)
	}

	return data, info.Mode().Perm(), nil
}

func backupFile(path string, content []byte) (string, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	backupPath := filepath.Join(dir, fmt.Sprintf("%s.backup.%s", base, time.Now().Format("20060102_150405")))
	if err := os.WriteFile(backupPath, content, 0o600); err != nil {
		return "", fmt.Errorf("write backup %s: %w", backupPath, err)
	}

	return backupPath, nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".ccaa-switch-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}

	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp file for %s: %w", path, err)
	}

	if err := tmpFile.Chmod(mode); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("set temp file mode for %s: %w", path, err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file for %s: %w", path, err)
	}

	if err := replaceFile(tmpPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}

	return nil
}

func replaceFile(src string, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.Rename(src, dst)
}

func normalizeNewlines(value string) string {
	return strings.ReplaceAll(value, "\r\n", "\n")
}

func detectNewline(content []byte) string {
	if bytes.Contains(content, []byte("\r\n")) {
		return "\r\n"
	}

	return "\n"
}
