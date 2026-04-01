# 🐸 Pepebot v0.5.10 - Default Model Update & Android Release Fix

**Release Date:** 2026-04-01

## 🎉 What's New

### ⚡ MAIA Router Default Model Updated

Pepebot now uses a faster MAIA default model out of the box:

- From: `maia/gemini-3-pro-preview`
- To: `maia/gemini-2.5-flash`

This update is applied in:

- Default agent configuration
- Interactive onboarding defaults
- `manage_agent` model example guidance

### 🤖 Android Release Trigger Reliability

Fixed GitHub Actions Android dispatch flow to avoid token resolution failures:

- `trigger-android` now supports token lookup from both:
  - `secrets.ANDROID_REPO_TOKEN`
  - `vars.ANDROID_REPO_TOKEN`
- Added explicit pre-check with clear error message if token is missing

This prevents the previous opaque error:

`Parameter token or opts.auth is required`

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

1. Run setup wizard: `pepebot onboard`
2. Start interactive agent: `pepebot agent`
3. (Optional) Start API gateway: `pepebot gateway`

## 📚 Documentation

- [Live API Guide](https://github.com/pepebot-space/pepebot/blob/main/docs/live-api.md)
- [Installation Guide](https://github.com/pepebot-space/pepebot/blob/main/docs/install.md)
- [Full Changelog](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md)

## 🔗 Links

- **GitHub**: https://github.com/pepebot-space/pepebot
- **Documentation**: https://github.com/pepebot-space/pepebot/tree/main/docs
- **Issues**: https://github.com/pepebot-space/pepebot/issues
- **Discussions**: https://github.com/pepebot-space/pepebot/discussions
