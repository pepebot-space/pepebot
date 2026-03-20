# 🐸 Pepebot v0.5.9 - Live API Video Toggle

**Release Date:** 2026-03-20

## 🎉 What's New

### 🎥 Live API Video Toggle (`live.video`)

Live realtime sessions now support a simple config switch for video mode:

```json
{
  "live": {
    "video": true
  }
}
```

You can also set it via environment variable:

```bash
export PEPEBOT_LIVE_VIDEO=true
```

### ✅ Provider Video Capability Check

When connecting to `/v1/live`, the server now includes video capability metadata in the connected payload:

- `video.requested`
- `video.supported`
- `video.enabled`

Explicit video support is currently available for:

- `vertex`
- `gemini`

For those providers, enabling `live.video=true` also auto-ensures `generation_config.responseModalities` contains `VIDEO`.

If `live.video=true` is used with a non-video provider, session still runs for audio/text and a warning is logged.

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
Download the appropriate binary for your platform from the [releases page](https://github.com/pepebot-space/pepebot/releases).

## 🚀 Quick Start

1. Enable live mode and video toggle in config or env
2. Start gateway: `pepebot gateway`
3. Connect WebSocket client to `ws://localhost:18790/v1/live`

## 📚 Documentation

- [Live API Guide](https://github.com/pepebot-space/pepebot/blob/main/docs/live-api.md)
- [Installation Guide](https://github.com/pepebot-space/pepebot/blob/main/docs/install.md)
- [Full Changelog](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md)

## 🔗 Links

- **GitHub**: https://github.com/pepebot-space/pepebot
- **Documentation**: https://github.com/pepebot-space/pepebot/tree/main/docs
- **Issues**: https://github.com/pepebot-space/pepebot/issues
- **Discussions**: https://github.com/pepebot-space/pepebot/discussions
