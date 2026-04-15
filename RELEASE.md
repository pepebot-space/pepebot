# 🐸 Pepebot v0.5.13 - Reply Context & WhatsApp Typing

**Release Date:** 2026-04-15

## 🎉 What's New

### 💬 Reply Context Now Visible to the Bot

When you reply to a message in **Discord**, **Telegram**, or **WhatsApp**, the bot now sees the full text of the message you're replying to. This means you can say "translate this" while replying to a message and the bot will know exactly what to translate — no need to copy-paste.

The quoted text is passed as context in the format:
```
[replying to username: original message text]
your new message
```

Previously, replies in Discord only forwarded media attachments (not text), and Telegram and WhatsApp ignored reply context entirely.

### ⌨️ WhatsApp Typing Indicator

The bot now shows a **typing/composing indicator** in WhatsApp while it's processing your message — just like it already did in Discord. The indicator appears immediately when your message is received and disappears when the response is sent.

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

- [Workflow Guide](https://github.com/pepebot-space/pepebot/blob/main/docs/workflows.md)
- [Installation Guide](https://github.com/pepebot-space/pepebot/blob/main/docs/install.md)
- [Full Changelog](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md)

## 🔗 Links

- **GitHub**: https://github.com/pepebot-space/pepebot
- **Documentation**: https://github.com/pepebot-space/pepebot/tree/main/docs
- **Issues**: https://github.com/pepebot-space/pepebot/issues
- **Discussions**: https://github.com/pepebot-space/pepebot/discussions
