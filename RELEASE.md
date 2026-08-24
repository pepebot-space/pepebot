# 🐸 Pepebot v0.5.17 - OpenCode Go: Full Menu, Working Tools

**Release Date:** 2026-08-24

## 🎉 What's New

### 🚀 Refreshed OpenCode Go model catalog

OpenCode Go quietly grew from 3 models to ~30. Pepebot now knows about them.

- **New default:** `minimax-m3` (was `minimax-m2.5`) — newer, same high volume limit.
- **Now selectable:** the whole `kimi-k3` / `glm-5.3` / `deepseek-v4` / `qwen3.8-max` line-up, plus `mimo`, `hy3`, `grok-4.5` and friends.
- **Onboarding and docs updated**, so `pepebot onboard` no longer offers you a menu from last season.

Already running an OpenCode Go model? Nothing breaks — your configured `model` keeps working. Only the default for fresh setups changed.

### 🐛 Tool calling on OpenCode Go actually works now

Any OpenCode Go turn that used a tool died with an HTTP 400. Pepebot was replaying tool calls with a `null` argument object, which the API refuses. Fixed and verified end-to-end on `minimax-m3`, `kimi-k3`, `glm-5.3`, `deepseek-v4-pro` and `qwen3.8-max` — single tool, multiple tools, multi-turn.

## 📦 Installation

```bash
curl -fsSL https://raw.githubusercontent.com/pepebot-space/pepebot/main/install.sh | bash
```

Or with Homebrew:

```bash
brew tap pepebot-space/tap https://github.com/pepebot-space/homebrew-tap
brew install pepebot
```

## 🚀 Quick Start

```json
{
  "agents": {
    "defaults": {
      "model": "minimax-m3",
      "provider": "opencodego"
    }
  },
  "providers": {
    "opencodego": {
      "api_key": "YOUR_OPENCODE_API_KEY"
    }
  }
}
```

Want the live list of what OpenCode Go is serving right now?

```bash
curl -H "x-api-key: $OPENCODEGO_API_KEY" https://opencode.ai/zen/go/v1/models
```

## 🔗 Links

- [Changelog](./CHANGELOG.md)
- [README](./README.md)
- [OpenCode Go docs](https://opencode.ai/docs/go)
- [Get an API key](https://opencode.ai/auth)
