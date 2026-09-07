package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"ccaa/internal/codex"
	"ccaa/internal/config"
	"ccaa/internal/install"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	root := flag.NewFlagSet("ccaa", flag.ContinueOnError)
	root.SetOutput(stderr)
	var configPath string
	root.StringVar(&configPath, "config", "", "config file path")
	root.StringVar(&configPath, "c", "", "config file path")
	root.Usage = func() {
		printRootUsage(stdout)
	}

	if err := root.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	remaining := root.Args()
	if len(remaining) == 0 {
		printRootUsage(stdout)
		return 0
	}

	if remaining[0] == "help" {
		return runHelp(remaining[1:], stdout, stderr)
	}

	switch remaining[0] {
	case "init":
		resolvedConfigPath, err := chooseConfigPath(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "resolve config path: %v\n", err)
			return 1
		}
		return runInit(remaining[1:], resolvedConfigPath, stdout, stderr)
	case "add":
		resolvedConfigPath, err := chooseConfigPath(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "resolve config path: %v\n", err)
			return 1
		}
		return runAdd(remaining[1:], resolvedConfigPath, stdout, stderr)
	case "list":
		resolvedConfigPath, err := chooseConfigPath(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "resolve config path: %v\n", err)
			return 1
		}
		return runList(remaining[1:], resolvedConfigPath, stdout, stderr)
	case "current":
		resolvedConfigPath, err := chooseConfigPath(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "resolve config path: %v\n", err)
			return 1
		}
		return runCurrent(remaining[1:], resolvedConfigPath, stdout, stderr)
	case "use":
		resolvedConfigPath, err := chooseConfigPath(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "resolve config path: %v\n", err)
			return 1
		}
		return runUse(remaining[1:], resolvedConfigPath, stdout, stderr)
	case "openai":
		resolvedConfigPath, err := chooseConfigPath(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "resolve config path: %v\n", err)
			return 1
		}
		return runOpenAI(remaining[1:], resolvedConfigPath, stdout, stderr)
	case "install":
		return runInstall(remaining[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", remaining[0])
		printRootUsage(stderr)
		return 2
	}
}

func runInit(args []string, configPath string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printInitUsage(stdout)
	}
	var force bool
	fs.BoolVar(&force, "force", false, "overwrite existing config file")
	fs.BoolVar(&force, "f", false, "overwrite existing config file")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "init does not accept positional arguments")
		printInitUsage(stderr)
		return 2
	}

	resolvedPath, err := config.ResolvePath(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "resolve config path: %v\n", err)
		return 1
	}

	if _, err := os.Stat(resolvedPath); err == nil && !force {
		fmt.Fprintf(stderr, "config already exists: %s\n", resolvedPath)
		return 1
	}

	cfg := config.New()
	if err := config.Save(resolvedPath, cfg); err != nil {
		fmt.Fprintf(stderr, "write config file: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "initialized %s\n", resolvedPath)
	return 0
}

func runAdd(args []string, configPath string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printAddUsage(stdout)
	}
	var name string
	var baseURL string
	var apiKey string
	var provider string
	var description string
	fs.StringVar(&name, "name", "", "profile name")
	fs.StringVar(&name, "n", "", "profile name")
	fs.StringVar(&baseURL, "base-url", "", "base URL for Codex")
	fs.StringVar(&baseURL, "u", "", "base URL for Codex")
	fs.StringVar(&apiKey, "api-key", "", "experimental_bearer_token value")
	fs.StringVar(&apiKey, "k", "", "experimental_bearer_token value")
	fs.StringVar(&provider, "provider", "", "provider label")
	fs.StringVar(&provider, "p", "", "provider label")
	fs.StringVar(&description, "description", "", "description")
	fs.StringVar(&description, "d", "", "description")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "add does not accept positional arguments")
		printAddUsage(stderr)
		return 2
	}

	if strings.TrimSpace(name) == "" || strings.TrimSpace(baseURL) == "" || strings.TrimSpace(apiKey) == "" {
		fmt.Fprintln(stderr, "add requires --name/-n, --base-url/-u, and --api-key/-k")
		printAddUsage(stderr)
		return 2
	}

	cfg, err := loadOrCreateConfig(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}

	cfg.UpsertProfile(config.Profile{
		Name:        strings.TrimSpace(name),
		Provider:    strings.TrimSpace(provider),
		Description: strings.TrimSpace(description),
		BaseURL:     strings.TrimSpace(baseURL),
		APIKey:      strings.TrimSpace(apiKey),
	})

	if err := config.Save(configPath, cfg); err != nil {
		fmt.Fprintf(stderr, "save config: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "saved profile %s\n", name)
	return 0
}

func runList(args []string, configPath string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printListUsage(stdout)
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "list does not accept positional arguments")
		printListUsage(stderr)
		return 2
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "config not found: %s\nrun `ccaa init` first\n", configPath)
			return 1
		}

		fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}

	if len(cfg.Profiles) == 0 {
		fmt.Fprintln(stdout, "no profiles")
		return 0
	}

	for _, profile := range cfg.Profiles {
		marker := " "
		if profile.Name == cfg.CurrentProfile {
			marker = "*"
		}

		description := profile.Description
		if description == "" {
			description = "-"
		}

		provider := profile.Provider
		if provider == "" {
			provider = "-"
		}

		fmt.Fprintf(stdout, "%s %s | provider=%s | base_url=%s | %s\n", marker, profile.Name, provider, profile.BaseURL, description)
	}

	return 0
}

func runCurrent(args []string, configPath string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("current", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printCurrentUsage(stdout)
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "current does not accept positional arguments")
		printCurrentUsage(stderr)
		return 2
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "config not found: %s\nrun `ccaa init` first\n", configPath)
			return 1
		}

		fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}

	if cfg.CurrentProfile == "" {
		fmt.Fprintln(stdout, "no current profile")
		return 0
	}

	if cfg.CurrentProfile == "openai" {
		fmt.Fprintln(stdout, "openai")
		return 0
	}
	profile, _ := cfg.FindProfile(cfg.CurrentProfile)
	if profile == nil {
		fmt.Fprintf(stderr, "current profile %q is missing from config\n", cfg.CurrentProfile)
		return 1
	}

	fmt.Fprintf(stdout, "%s | base_url=%s\n", profile.Name, profile.BaseURL)
	return 0
}

func runUse(args []string, configPath string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("use", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printUseUsage(stdout)
	}
	var name string
	fs.StringVar(&name, "name", "", "profile name")
	fs.StringVar(&name, "n", "", "profile name")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	switch {
	case strings.TrimSpace(name) == "" && len(fs.Args()) == 1:
		name = fs.Args()[0]
	case strings.TrimSpace(name) != "" && len(fs.Args()) == 0:
	case strings.TrimSpace(name) == "" && len(fs.Args()) == 0:
		fmt.Fprintln(stderr, "use requires a profile name")
		printUseUsage(stderr)
		return 2
	default:
		fmt.Fprintln(stderr, "use accepts either one positional profile name or --name/-n")
		printUseUsage(stderr)
		return 2
	}

	if name == "openai" || name == "opneai" {
		return runOpenAI(nil, configPath, stdout, stderr)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "config not found: %s\nrun `ccaa init` first\n", configPath)
			return 1
		}

		fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}

	profile, _ := cfg.FindProfile(name)
	if profile == nil {
		fmt.Fprintf(stderr, "profile %q not found\n", name)
		return 1
	}

	codexConfigPath, err := config.ResolvePath(cfg.Codex.ConfigPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	result, err := codex.Apply(codex.SwitchRequest{
		ConfigPath: codexConfigPath,
		BaseURL:    profile.BaseURL,
		APIKey:     profile.APIKey,
	})
	if err != nil {
		fmt.Fprintf(stderr, "switch profile %q: %v\n", profile.Name, err)
		return 1
	}

	cfg.CurrentProfile = profile.Name
	if err := config.Save(configPath, cfg); err != nil {
		fmt.Fprintf(stderr, "save config: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "switched to %s via provider section %s\n", profile.Name, result.Provider)
	if result.ConfigBackup != "" {
		fmt.Fprintf(stdout, "config backup: %s\n", result.ConfigBackup)
	}
	return 0
}

func runOpenAI(args []string, configPath string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("openai", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printOpenAIUsage(stdout)
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "openai does not accept positional arguments")
		printOpenAIUsage(stderr)
		return 2
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "config not found: %s\nrun `ccaa init` first\n", configPath)
			return 1
		}

		fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}

	codexConfigPath, err := config.ResolvePath(cfg.Codex.ConfigPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	result, err := codex.ApplyOpenAI(codex.OpenAIRequest{
		ConfigPath: codexConfigPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "switch to OpenAI: %v\n", err)
		return 1
	}

	cfg.CurrentProfile = "openai"
	if err := config.Save(configPath, cfg); err != nil {
		fmt.Fprintf(stderr, "save config: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "switched to OpenAI: cleared overrides in provider section %s\n", result.Provider)
	if result.ConfigBackup != "" {
		fmt.Fprintf(stdout, "config backup: %s\n", result.ConfigBackup)
	}
	return 0
}

func runInstall(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printInstallUsage(stdout)
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if len(fs.Args()) != 0 {
		fmt.Fprintln(stderr, "install does not accept positional arguments")
		printInstallUsage(stderr)
		return 2
	}

	result, err := install.Run()
	if err != nil {
		fmt.Fprintf(stderr, "install failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "installed %s\n", result.TargetPath)
	for _, note := range result.Notes {
		fmt.Fprintf(stdout, "note: %s\n", note)
	}

	return 0
}

func chooseConfigPath(flagPath string) (string, error) {
	if strings.TrimSpace(flagPath) != "" {
		return config.ResolvePath(flagPath)
	}

	if envPath := strings.TrimSpace(os.Getenv("CCAA_CONFIG")); envPath != "" {
		return config.ResolvePath(envPath)
	}

	return config.DefaultPath()
}

func loadOrCreateConfig(path string) (*config.File, error) {
	cfg, err := config.Load(path)
	if err == nil {
		return cfg, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	return config.New(), nil
}

func runHelp(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printRootUsage(stdout)
		return 0
	}

	if len(args) != 1 {
		fmt.Fprintln(stderr, "help accepts at most one command name")
		printRootUsage(stderr)
		return 2
	}

	switch args[0] {
	case "init":
		printInitUsage(stdout)
	case "add":
		printAddUsage(stdout)
	case "list":
		printListUsage(stdout)
	case "current":
		printCurrentUsage(stdout)
	case "use":
		printUseUsage(stdout)
	case "openai":
		printOpenAIUsage(stdout)
	case "install":
		printInstallUsage(stdout)
	case "help":
		printHelpUsage(stdout)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printRootUsage(stderr)
		return 2
	}

	return 0
}

func printRootUsage(w io.Writer) {
	fmt.Fprintln(w, "ccaa manages Codex base_url and experimental_bearer_token profiles.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  ccaa [-c /path/to/config.toml] <command> [options]")
	fmt.Fprintln(w, "  ccaa help [command]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Global Options:")
	fmt.Fprintln(w, "  -c, --config   Config file path")
	fmt.Fprintln(w, "  -h, --help     Show help")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  init       Create ~/.ccaa/config.toml")
	fmt.Fprintln(w, "  add        Add or replace a profile in the config file")
	fmt.Fprintln(w, "  list       List profiles")
	fmt.Fprintln(w, "  current    Show current profile")
	fmt.Fprintln(w, "  use        Update URL/token in ~/.codex/config.toml (or use openai to clear them)")
	fmt.Fprintln(w, "  openai     Remove base_url and experimental_bearer_token overrides")
	fmt.Fprintln(w, "  install    Install the current binary into a callable location")
	fmt.Fprintln(w, "  help       Show root help or help for one command")
}

func printInitUsage(w io.Writer) {
	fmt.Fprintln(w, "Create the single ccaa config file.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  ccaa [-c /path/to/config.toml] init [options]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -f, --force    Overwrite an existing config file")
	fmt.Fprintln(w, "  -h, --help     Show help")
}

func printAddUsage(w io.Writer) {
	fmt.Fprintln(w, "Add or replace one profile in the ccaa config file.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  ccaa [-c /path/to/config.toml] add -n NAME -u BASE_URL -k API_KEY [options]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -n, --name           Profile name")
	fmt.Fprintln(w, "  -u, --base-url       Base URL to write into ~/.codex/config.toml")
	fmt.Fprintln(w, "  -k, --api-key        experimental_bearer_token to write into Codex config")
	fmt.Fprintln(w, "  -p, --provider       Optional display label (does not select a provider)")
	fmt.Fprintln(w, "  -d, --description    Optional description")
	fmt.Fprintln(w, "  -h, --help           Show help")
}

func printListUsage(w io.Writer) {
	fmt.Fprintln(w, "List all saved profiles.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  ccaa [-c /path/to/config.toml] list")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -h, --help     Show help")
}

func printCurrentUsage(w io.Writer) {
	fmt.Fprintln(w, "Show the current profile recorded in ~/.ccaa/config.toml.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  ccaa [-c /path/to/config.toml] current")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -h, --help     Show help")
}

func printUseUsage(w io.Writer) {
	fmt.Fprintln(w, "Switch Codex to one saved profile.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  ccaa [-c /path/to/config.toml] use <name>")
	fmt.Fprintln(w, "  ccaa [-c /path/to/config.toml] use -n <name>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -n, --name     Profile name")
	fmt.Fprintln(w, "  -h, --help     Show help")
}

func printOpenAIUsage(w io.Writer) {
	fmt.Fprintln(w, "Remove URL/token overrides; leave auth.json and model_provider untouched.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  ccaa [-c /path/to/config.toml] openai")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -h, --help          Show help")
}

func printInstallUsage(w io.Writer) {
	fmt.Fprintln(w, "Install the current ccaa binary into a PATH location.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  ccaa install")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Behavior:")
	fmt.Fprintln(w, "  Linux: prefer a writable system PATH directory, otherwise use ~/.local/bin")
	fmt.Fprintln(w, "  Windows: copy to %LocalAppData%\\ccaa\\bin and add that directory to the user PATH")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -h, --help     Show help")
}

func printHelpUsage(w io.Writer) {
	fmt.Fprintln(w, "Show help for ccaa or one subcommand.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  ccaa help")
	fmt.Fprintln(w, "  ccaa help <command>")
}
