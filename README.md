# ccaa

`ccaa` is a small Go CLI that switches Codex between saved URL/token profiles.

It stores its own state in one file:

- `~/.ccaa/config.toml`

It patches only:

- `~/.codex/config.toml`

It never reads or writes `auth.json` or changes `model`. Profile switching sets `model_provider = "rightcode"`.

## Build

```bash
go build -o ccaa .
./ccaa install
```

## Quick Start

```bash
./ccaa init
./ccaa add -n main -p vendor-a -u https://example.com/v1 -k sk-xxx -d "monthly route"
./ccaa list
./ccaa use main
./ccaa openai
./ccaa use openai
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

[[profiles]]
name = "main"
provider = "vendor-a"
description = "monthly route"
base_url = "https://example.com/v1"
api_key = "sk-xxx"
```

## Notes

- `use NAME` sets `model_provider = "rightcode"`, then writes the profile's `base_url` and `api_key` into `base_url` and `experimental_bearer_token` in the fixed `[model_providers.rightcode]` section. Missing fields or the section are created.
- The profile's `provider` is only a display label and is ignored for switching.
- `openai` and `use openai` delete those two overrides from `[model_providers.rightcode]`. `use opneai` is also accepted. No saved OpenAI profile or auth source is required.
- OpenAI switching clears overrides only; it does not repair an old custom provider or restore credentials overwritten by earlier versions. Prepare your desired provider and login separately if needed.
- `--auth-source` and `codex.auth_path` are no longer used.
- `install` copies the current binary into a PATH location appropriate for the current OS
- Before a change, `ccaa` creates a uniquely named, owner-only backup beside the Codex config. Repeated identical switches do not create backups.
