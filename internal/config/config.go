package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

const (
	ConfigRelativePath      = ".ccaa/config.toml"
	DefaultCodexConfigPath  = "~/.codex/config.toml"
	defaultConfigFileMode   = 0o600
	defaultConfigFolderMode = 0o700
)

type File struct {
	Version        int         `toml:"version"`
	CurrentProfile string      `toml:"current_profile"`
	Codex          CodexConfig `toml:"codex"`
	Profiles       []Profile   `toml:"profiles"`
}

type CodexConfig struct {
	ConfigPath string `toml:"config_path"`
}

type Profile struct {
	Name        string `toml:"name"`
	Provider    string `toml:"provider,omitempty"`
	Description string `toml:"description,omitempty"`
	BaseURL     string `toml:"base_url"`
	APIKey      string `toml:"api_key"`
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(home, ConfigRelativePath), nil
}

func ResolvePath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	if path == "~" {
		return os.UserHomeDir()
	}

	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}

		return filepath.Join(home, path[2:]), nil
	}

	return filepath.Clean(path), nil
}

func New() *File {
	return &File{
		Version: 1,
		Codex: CodexConfig{
			ConfigPath: DefaultCodexConfigPath,
		},
		Profiles: []Profile{},
	}
}

func Load(path string) (*File, error) {
	resolvedPath, err := ResolvePath(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", resolvedPath, err)
	}

	cfg := New()
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", resolvedPath, err)
	}

	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func Save(path string, cfg *File) error {
	resolvedPath, err := ResolvePath(path)
	if err != nil {
		return err
	}

	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(resolvedPath), defaultConfigFolderMode); err != nil {
		return fmt.Errorf("create config directory for %s: %w", resolvedPath, err)
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config file %s: %w", resolvedPath, err)
	}

	return writeFileAtomic(resolvedPath, data, defaultConfigFileMode)
}

func (f *File) Validate() error {
	if f.Version == 0 {
		f.Version = 1
	}

	if strings.TrimSpace(f.Codex.ConfigPath) == "" {
		return errors.New("codex.config_path cannot be empty")
	}

	seen := make(map[string]struct{}, len(f.Profiles))
	for _, profile := range f.Profiles {
		if strings.TrimSpace(profile.Name) == "" {
			return errors.New("profile name cannot be empty")
		}

		if strings.TrimSpace(profile.BaseURL) == "" {
			return fmt.Errorf("profile %q base_url cannot be empty", profile.Name)
		}

		if strings.TrimSpace(profile.APIKey) == "" {
			return fmt.Errorf("profile %q api_key cannot be empty", profile.Name)
		}

		if _, exists := seen[profile.Name]; exists {
			return fmt.Errorf("duplicate profile name %q", profile.Name)
		}

		seen[profile.Name] = struct{}{}
	}

	if f.CurrentProfile != "" && f.CurrentProfile != "openai" {
		if _, ok := seen[f.CurrentProfile]; !ok {
			return fmt.Errorf("current_profile %q does not exist", f.CurrentProfile)
		}
	}

	return nil
}

func (f *File) FindProfile(name string) (*Profile, int) {
	for i := range f.Profiles {
		if f.Profiles[i].Name == name {
			return &f.Profiles[i], i
		}
	}

	return nil, -1
}

func (f *File) UpsertProfile(profile Profile) {
	if existing, index := f.FindProfile(profile.Name); existing != nil {
		f.Profiles[index] = profile
		return
	}

	f.Profiles = append(f.Profiles, profile)
}

func (f *File) applyDefaults() {
	if f.Version == 0 {
		f.Version = 1
	}

	if strings.TrimSpace(f.Codex.ConfigPath) == "" {
		f.Codex.ConfigPath = DefaultCodexConfigPath
	}

}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".ccaa-config-*.tmp")
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

	if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return os.Rename(src, dst)
}
