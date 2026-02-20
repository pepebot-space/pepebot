# 🐸 Pepebot v0.5.0 - Local Dashboard Interface

**Release Date:** 2026-02-20

## 🎉 What's New

### 🖥️ Local Dashboard Interface

Pepebot now includes a fully featured modern Web UI dashboard for managing your AI agent locally or deployed via static hosting like Cloudflare Pages.

**Start the dashboard:**
```bash
pepebot gateway
```
*The dashboard will automatically be available via your gateway interface (default: http://localhost:18790)*

### 🐸 Floating AI Assistant (Frog Panel)

Global access to Pepebot within the dashboard interface.
- Embedded chat interface that slides in from the right edge
- Context-aware prompting based on the active dashboard page
- Full Markdown and Code syntax highlighting support
- Image upload capabilities directly from the browser

### ☁️ Cloudflare Pages Ready

Complete restructure of the frontend architecture for zero-server static hosting.
- Removed intermediate proxy servers
- Direct API connections via CORS and localStorage configuration
- Upload payloads now converted locally to Base64 natively

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
Download the appropriate binary for your platform from the [releases page](https://github.com/pepebot-space/pepebot/releases/tag/v0.5.0).

## 🚀 Quick Start

1. **Start the gateway & dashboard:**
   ```bash
   pepebot gateway -v
   ```

2. **Access the Web Dashboard:**
   Open your browser and navigate to `http://localhost:18790` (or your configured gateway port)

## 📚 Documentation

- [Installation Guide](https://github.com/pepebot-space/pepebot/blob/main/docs/install.md)
- [API Reference](https://github.com/pepebot-space/pepebot/blob/main/docs/api.md)
- [Workflow Documentation](https://github.com/pepebot-space/pepebot/blob/main/docs/workflows.md)
- [Full Changelog](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md)

## 🔗 Links

- **GitHub**: https://github.com/pepebot-space/pepebot
- **Documentation**: https://github.com/pepebot-space/pepebot/tree/main/docs
- **Issues**: https://github.com/pepebot-space/pepebot/issues
- **Discussions**: https://github.com/pepebot-space/pepebot/discussions

## 📝 Full Changelog

For a complete list of changes, see [CHANGELOG.md](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md#050---2026-02-20).

---

**Note:** When upgrading from v0.4.x, all existing configurations and data are preserved. No migration needed. The dashboard will automatically serve traffic from your gateway configuration.
