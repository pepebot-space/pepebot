# 🐸 Pepebot v0.5.26 - One Model, One Place, One Spelling

**Release Date:** 2026-09-24

## ⚡ What's New

### Write the model one way

```json
{ "agents": { "defaults": { "model": "maiarouter:zai/glm-4.5v" } } }
```

Left of the colon is the endpoint, right of it is the model — sent exactly as written. No more wondering whether `maiarouter/zai/glm-5.3` means a provider, a vendor, or a model: slashes belong to the model id, and only the first colon splits.

If you prefer the old pair, it still works:

```json
{ "agents": { "defaults": { "provider": "maiarouter", "model": "zai/glm-4.5v" } } }
```

### Add any endpoint without waiting for a release

```json
{
  "providers": {
    "custom": {
      "9router": { "api_base": "http://localhost:20128/v1", "api_key": "..." },
      "ollama":  { "api_base": "http://minipepe:11434/v1", "api_key": "-", "model": "qwen3" }
    }
  }
}
```

Then just name it: `custom:9router/kr/claude-sonnet-4.5`, `custom:ollama/qwen3`. Anything that speaks the OpenAI shape — a gateway, a local model server, your own proxy — is a config edit now, not a code change.

### The model lives in one place, and pepebot asks before changing it

The model used to be set in two places — `config.json` and the agent registry — and the registry quietly won. Editing `config.json` looked like it did nothing, and a stale entry kept going out on the wire.

The registry is now the source of truth, `config.json` seeds a fresh install, and when the two disagree pepebot asks:

```
⚠️  Two different models are configured:
    agent registry : custom:mr/zai/glm-4.5v   (this is what runs today)
    config.json    : maiarouter:zai/glm-4.5v

Use the config.json model and overwrite the registry? [y/N]:
```

Answer once and it is remembered. Running as a service with no terminal? Nothing blocks — the registry wins and both values go into the log, so you can still see the mismatch.

Full details: [docs/providers.md](docs/providers.md)

## 📦 Installation

```bash
curl -fsSL https://raw.githubusercontent.com/pepebot-space/pepebot/main/install.sh | bash
```

## 🚀 Quick Start

```bash
pepebot onboard
pepebot gateway
```

## 🔗 Links

- [Changelog](CHANGELOG.md)
- [Providers Guide](docs/providers.md)
- [Memory Guide](docs/memory.md)
- [Documentation](docs/README.md)
