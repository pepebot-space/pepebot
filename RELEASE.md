# 🐸 Pepebot v0.5.4 - Platform Messaging Tools

**Release Date:** 2026-02-22

## 🎉 What's New

### 📨 Direct Telegram & Discord Messaging (No Gateway Required!)

Send messages, images, and files to Telegram and Discord from the agent or any workflow — even without the gateway running. Perfect for scheduled notifications and automated reports.

**Telegram:**
```json
{
  "name": "notify",
  "tool": "telegram_send",
  "args": {
    "chat_id": "123456789",
    "text": "Daily report:\n{{fetch_data}}"
  }
}
```

**Discord:**
```json
{
  "name": "post",
  "tool": "discord_send",
  "args": {
    "channel_id": "987654321",
    "content": "Build complete! {{build_output}}"
  }
}
```

**WhatsApp** (requires gateway):
```json
{
  "name": "alert",
  "tool": "whatsapp_send",
  "args": {
    "jid": "628123456789@s.whatsapp.net",
    "text": "Alert: {{message}}"
  }
}
```

**File sending** (photos, video, audio, documents):
```json
{
  "tool": "telegram_send",
  "args": {
    "chat_id": "123456789",
    "file_path": "/path/to/screenshot.png",
    "caption": "Today's snapshot"
  }
}
```

Telegram auto-detects file type: images → `sendPhoto`, video → `sendVideo`, audio → `sendAudio`, everything else → `sendDocument`.

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
Download the appropriate binary for your platform from the [releases page](https://github.com/pepebot-space/pepebot/releases/tag/v0.5.4).

## 🚀 Quick Start

1. **Configure your Telegram or Discord token** in `~/.pepebot/config.json`
2. **Use in a workflow:**
   ```bash
   pepebot workflow run daily_report
   ```
3. **Or from the agent:**
   ```bash
   pepebot agent -m "Send a summary to my Telegram chat 123456789"
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

For a complete list of changes, see [CHANGELOG.md](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md#054---2026-02-22).

---

**Note:** When upgrading from v0.5.3, all existing configurations, workflows, and data are preserved. No migration needed.
