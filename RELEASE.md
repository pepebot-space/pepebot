# 🐸 Pepebot v0.5.29 - Claude, With PDFs

**Release Date:** 2026-10-05

## ⚡ What's New

### Point pepebot at Claude and send it documents

```json
{
  "providers": { "anthropic": { "api_key": "sk-ant-...", "api_base": "https://api.anthropic.com" } },
  "agents": { "defaults": { "model": "anthropic:claude-sonnet-5-5" } }
}
```

Text, images, and **PDFs** all work.

The PDF part took a change of road. Anthropic serves an OpenAI-compatible endpoint, and pepebot was using it — fine for text and images, but hand it a document and it answers with a 400. Pepebot now talks to Anthropic's own Messages API instead, which takes documents natively. Same config, no flags.

Worse than the 400, until now a PDF sent through this provider was **silently dropped**: the request succeeded and the model answered as if you had attached nothing at all. That is fixed.

### Two fixes you would have hit immediately

**`temperature` no longer breaks every request.** The current Claude models removed sampling parameters — Sonnet 5.5 rejects a request carrying `temperature` outright. Pepebot no longer sends it to Anthropic.

**The provider is called what it is.** `OpenCodeProvider` always spoke Anthropic's wire format; it is now `AnthropicProvider`, and errors say which endpoint they came from. Existing opencode setups are untouched.

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
