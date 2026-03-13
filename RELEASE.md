# 🐸 Pepebot v0.5.8 - OpenCode Go Provider

**Release Date:** 2026-03-13

## 🎉 What's New

### 🚀 OpenCode Go Provider

Pepebot now supports **OpenCode Go** — a low-cost subscription for open coding models hosted globally (US, EU, Singapore).

- **Affordable:** $5 first month, then $10/month
- **Global:** Models hosted in US, EU, Singapore for stable international access
- **High Limits:** Generous usage limits (up to 100K requests/month for minimax-m2.5)
- **Default Model:** `minimax-m2.5` (recommended)

**Available Models:**
- `minimax-m2.5` — Best value, high volume limit
- `kimi-k2.5`
- `glm-5`

**Quick Setup:**

```json
{
  "agents": {
    "defaults": {
      "model": "minimax-m2.5",
      "provider": "opencodego"
    }
  },
  "providers": {
    "opencodego": {
      "api_key": "your-opencode-api-key"
    }
  }
}
```

**Important:** Always set `provider: "opencodego"` to avoid conflicts with other providers.

Or via environment:

```bash
export OPENCODEGO_API_KEY="your-api-key"
export PEPEBOT_AGENTS_DEFAULTS_MODEL="minimax-m2.5"
export PEPEBOT_AGENTS_DEFAULTS_PROVIDER="opencodego"
```

Get your API key at [opencode.ai/auth](https://opencode.ai/auth)

---

## 📦 Installation

### Using Install Script (Recommended)
```bash
curl -fsSL https://raw.githubusercontent.com/pepebot-space/pepebot/main/install.sh | bash
```

### Using Homebrew
```bash
brew tap pepebot-space/pepebot
brew install pepebot
```

### Using Docker
```bash
docker pull ghcr.io/pepebot-space/pepebot:latest
docker run -it --rm pepebot:latest
```

### Manual Download
Download the appropriate binary for your platform from the [releases page](https://github.com/pepebot-space/pepebot/releases/tag/v0.5.6).

## 🚀 Quick Start

1. Create a service account at [Google Cloud Console](https://console.cloud.google.com/iam-admin/serviceaccounts)
2. Download the JSON key file
3. Configure Pepebot:
   ```bash
   pepebot onboard
   # Or manually set environment variables
   ```
4. Start chatting:
   ```bash
   pepebot agent -m "Hello from Vertex AI!"
   ```

## 📚 Documentation

- [Workflow Documentation](https://github.com/pepebot-space/pepebot/blob/main/docs/workflows.md)
- [Installation Guide](https://github.com/pepebot-space/pepebot/blob/main/docs/install.md)
- [Full Changelog](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md)

## 🔗 Links

- **GitHub**: https://github.com/pepebot-space/pepebot
- **Documentation**: https://github.com/pepebot-space/pepebot/tree/main/docs
- **Issues**: https://github.com/pepebot-space/pepebot/issues
- **Discussions**: https://github.com/pepebot-space/pepebot/discussions

## 📝 Full Changelog

For a complete list of changes, see [CHANGELOG.md](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md#056---2026-03-02).
