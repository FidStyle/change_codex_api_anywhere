# ccaa

`ccaa` is a small Go CLI that switches Codex between saved `base_url + OPENAI_API_KEY` profiles.

It stores its own state in one file:

- `~/.ccaa/config.toml`

It patches these Codex files directly:

- `~/.codex/config.toml`
- `~/.codex/auth.json`

## Build

```bash
go build ./...
./ccaa install
```

## Quick Start

```bash
./ccaa init
./ccaa add -n main -p vendor-a -u https://example.com/v1 -k sk-xxx -d "monthly route"
./ccaa list
./ccaa use main
./ccaa openai
./ccaa current
./ccaa help add
./ccaa install
```

## Config Shape

```toml
version = 1
current_profile = "main"

[codex]
config_path = "~/.codex/config.toml"
auth_path = "~/.codex/auth.json"

[[profiles]]
name = "main"
provider = "vendor-a"
description = "monthly route"
base_url = "https://example.com/v1"
api_key = "sk-xxx"
```

## Notes

- `use` switches to the profile's `provider` when set, updates that provider's `base_url`, and rewrites `auth.json` to only contain `OPENAI_API_KEY`
- `openai` changes only `model_provider` to `openai` and copies the provided auth JSON without changing any `base_url`
- `openai --auth-source PATH` overrides the default OpenAI auth JSON source path
- `install` copies the current binary into a PATH location appropriate for the current OS
- before writing, `ccaa` creates timestamped backups next to both Codex files
