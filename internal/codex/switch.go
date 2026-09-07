package codex

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
)

type SwitchRequest struct {
	ConfigPath string
	BaseURL    string
	APIKey     string
}

type OpenAIRequest struct{ ConfigPath string }

type SwitchResult struct {
	Provider      string
	ConfigChanged bool
	ConfigBackup  string
}

func Apply(req SwitchRequest) (*SwitchResult, error) {
	if strings.TrimSpace(req.BaseURL) == "" || strings.TrimSpace(req.APIKey) == "" {
		return nil, fmt.Errorf("base_url and token cannot be empty")
	}
	return apply(req.ConfigPath, req.BaseURL, req.APIKey, false)
}

func ApplyOpenAI(req OpenAIRequest) (*SwitchResult, error) {
	return apply(req.ConfigPath, "", "", true)
}

func apply(path, baseURL, token string, clear bool) (*SwitchResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	before, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	after, provider, err := patchCredentials(before, baseURL, token, clear)
	if err != nil {
		return nil, fmt.Errorf("patch %s: %w", path, err)
	}
	result := &SwitchResult{Provider: provider, ConfigChanged: !bytes.Equal(before, after)}
	if !result.ConfigChanged {
		return result, nil
	}
	backup, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".backup.*")
	if err != nil {
		return nil, err
	}
	result.ConfigBackup = backup.Name()
	_, writeErr := backup.Write(before)
	closeErr := backup.Close()
	if writeErr != nil {
		return nil, writeErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := writeFileAtomic(path, after, info.Mode().Perm()); err != nil {
		return nil, err
	}
	return result, nil
}

// TOML node ranges preserve unrelated settings, comments and multiline strings.
func patchCredentials(content []byte, baseURL, token string, clear bool) ([]byte, string, error) {
	var cfg struct {
		Provider string `toml:"model_provider"`
	}
	if err := toml.Unmarshal(content, &cfg); err != nil {
		return nil, "", err
	}
	provider := cfg.Provider
	if provider == "" {
		provider = "openai"
	}
	type edit struct {
		start, end int
		text       string
	}
	var edits []edit
	found := map[string]bool{}
	var section []string
	insertAt := -1
	newline := "\n"
	if bytes.Contains(content, []byte("\r\n")) {
		newline = "\r\n"
	}
	var parser unstable.Parser
	parser.Reset(content)
	for parser.NextExpression() {
		node := parser.Expression()
		if node.Kind != unstable.Table && node.Kind != unstable.ArrayTable && node.Kind != unstable.KeyValue {
			continue
		}
		var keys []string
		keyStart := 0
		it := node.Key()
		for it.Next() {
			if len(keys) == 0 {
				keyStart = int(it.Node().Raw.Offset)
			}
			keys = append(keys, string(it.Node().Data))
		}
		if node.Kind != unstable.KeyValue {
			section = keys
			if node.Kind == unstable.Table && len(keys) == 2 && keys[0] == "model_providers" && keys[1] == provider {
				insertAt = len(content)
				if offset := bytes.IndexByte(content[keyStart:], '\n'); offset >= 0 {
					insertAt = keyStart + offset + 1
				}
			}
			continue
		}
		path := append(append([]string{}, section...), keys...)
		if len(path) != 3 || path[0] != "model_providers" || path[1] != provider {
			continue
		}
		key := path[2]
		if key != "base_url" && key != "experimental_bearer_token" {
			continue
		}
		value := node.Value()
		if value.Kind != unstable.String {
			return nil, "", fmt.Errorf("%s must be a string", key)
		}
		found[key] = true
		start, end := int(value.Raw.Offset), int(value.Raw.Offset+value.Raw.Length)
		if clear {
			start = bytes.LastIndexByte(content[:keyStart], '\n') + 1
			if offset := bytes.IndexByte(content[end:], '\n'); offset >= 0 {
				end += offset + 1
			} else {
				end = len(content)
			}
			edits = append(edits, edit{start: start, end: end})
		} else {
			v := baseURL
			if key == "experimental_bearer_token" {
				v = token
			}
			encoded, err := toml.Marshal(map[string]string{"v": v})
			if err != nil {
				return nil, "", err
			}
			edits = append(edits, edit{start, end, strings.TrimSpace(strings.SplitN(string(encoded), "=", 2)[1])})
		}
	}
	if err := parser.Error(); err != nil {
		return nil, "", err
	}
	if !clear {
		missing := map[string]string{}
		if !found["base_url"] {
			missing["base_url"] = baseURL
		}
		if !found["experimental_bearer_token"] {
			missing["experimental_bearer_token"] = token
		}
		if len(missing) > 0 {
			var data []byte
			var err error
			if insertAt < 0 {
				encoded, encodeErr := toml.Marshal(map[string]string{"provider": provider})
				if encodeErr != nil {
					return nil, "", encodeErr
				}
				header := "[model_providers." + strings.TrimSpace(strings.SplitN(string(encoded), "=", 2)[1]) + "]\n"
				data, err = toml.Marshal(missing)
				data = append([]byte(header), data...)
				insertAt = len(content)
			} else {
				data, err = toml.Marshal(missing)
			}
			if err != nil {
				return nil, "", err
			}
			text := strings.ReplaceAll(string(data), "\n", newline)
			if insertAt > 0 && content[insertAt-1] != '\n' {
				text = newline + text
			}
			edits = append(edits, edit{insertAt, insertAt, text})
		}
	}
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	next := append([]byte{}, content...)
	for _, e := range edits {
		next = append(append(append([]byte{}, next[:e.start]...), e.text...), next[e.end:]...)
	}
	var validated map[string]any
	if err := toml.Unmarshal(next, &validated); err != nil {
		return nil, "", fmt.Errorf("validate updated TOML: %w", err)
	}
	return next, provider, nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ccaa-switch-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
