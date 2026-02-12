# Running Pepebot on Android 🐸📱

Pepebot can run natively on Android devices using [Termux](https://termux.dev), a powerful terminal emulator that provides a Linux environment.

## 📋 Requirements

- Android device (ARM64 recommended for best performance)
- Termux app installed
- At least 100MB free storage
- Internet connection

## 🚀 Quick Start

### 1. Install Termux

Download Termux from one of these sources:
- **F-Droid** (Recommended): https://f-droid.org/packages/com.termux/
- **GitHub Releases**: https://github.com/termux/termux-app/releases

⚠️ **Important**: Do NOT install Termux from Google Play Store (outdated version).

### 2. Setup Termux

Open Termux and run these commands:

```bash
# Update packages
pkg update && pkg upgrade -y

# Install required packages
pkg install wget tar -y
```

### 3. Download Pepebot

Download the ARM64 binary (works on most Android devices):

```bash
# Download
wget https://github.com/anak10thn/pepebot/releases/latest/download/pepebot-android-arm64.tar.gz

# Extract
tar xzf pepebot-android-arm64.tar.gz

# Make executable
chmod +x pepebot-android-arm64

# Move to easy location
mv pepebot-android-arm64 ~/pepebot
```

### 4. Run Setup Wizard

```bash
~/pepebot onboard
```

Follow the interactive setup wizard to configure:
- AI provider (MAIA Router recommended for Indonesian users)
- API keys
- Chat channels (optional)
- Workspace location

### 5. Start Using Pepebot

```bash
# Interactive chat
~/pepebot agent

# Start as gateway (for bot channels)
~/pepebot gateway
```

## 🔧 Configuration

Pepebot stores configuration and data in `~/.pepebot/`:

```
~/.pepebot/
├── config.json          # Main configuration
├── workspace/           # Workspace directory
│   ├── memory/         # Long-term memory
│   ├── skills/         # Custom skills
│   ├── AGENTS.md       # Agent instructions
│   ├── SOUL.md         # Personality configuration
│   └── USER.md         # User preferences
└── sessions/           # Chat sessions
```

### Edit Configuration

```bash
# Using nano editor (install if needed)
pkg install nano -y
nano ~/.pepebot/config.json
```

### Recommended Settings for Android

```json
{
  "agents": {
    "defaults": {
      "workspace": "~/.pepebot/workspace",
      "model": "maia/gemini-2.5-flash",
      "max_tokens": 4096,
      "temperature": 0.7,
      "max_tool_iterations": 20
    }
  },
  "providers": {
    "maiarouter": {
      "api_key": "YOUR_API_KEY",
      "api_base": "https://api.maiarouter.ai/v1"
    }
  }
}
```

## 💡 Tips for Android Usage

### 1. **Keep Termux Running**

To prevent Android from killing Termux:
1. Go to Android Settings → Apps → Termux
2. Battery → Unrestricted
3. Disable battery optimization

### 2. **Background Execution**

Use `tmux` to keep Pepebot running in background:

```bash
# Install tmux
pkg install tmux -y

# Create new session
tmux new -s pepebot

# Run pepebot
~/pepebot gateway

# Detach: Press Ctrl+B then D
# Reattach: tmux attach -t pepebot
```

### 3. **Create Shortcut**

Add to `~/.bashrc` for easy access:

```bash
echo 'alias pepebot="~/pepebot"' >> ~/.bashrc
source ~/.bashrc

# Now you can just type:
pepebot agent
```

### 4. **Storage Management**

Check storage usage:
```bash
du -sh ~/.pepebot
```

Clean up old sessions:
```bash
rm -rf ~/.pepebot/sessions/*
```

### 5. **Auto-start on Boot**

Install Termux:Boot app from F-Droid, then create startup script:

```bash
mkdir -p ~/.termux/boot
nano ~/.termux/boot/pepebot.sh
```

Add:
```bash
#!/data/data/com.termux/files/usr/bin/bash
termux-wake-lock
~/pepebot gateway &
```

Make executable:
```bash
chmod +x ~/.termux/boot/pepebot.sh
```

## 📱 Termux API Integration

Pepebot includes a powerful **Termux API skill** that lets you control Android device features!

### Install Termux API

```bash
# 1. Install Termux:API app from F-Droid
# https://f-droid.org/packages/com.termux.api/

# 2. Install API package in Termux
pkg install termux-api
```

### What You Can Do

The Termux API skill enables:
- 🔋 **Battery monitoring** - Check status, temperature, charging state
- 📸 **Camera control** - Take photos/videos
- 📋 **Clipboard** - Read/write system clipboard
- 📍 **Location** - GPS coordinates and tracking
- 🔔 **Notifications** - Display Android notifications
- 💬 **SMS** - Send/receive text messages
- 🔊 **Audio** - Text-to-speech, speech-to-text
- 🌐 **WiFi** - Network scanning and info
- 📱 **Sensors** - Accelerometer, gyroscope, light, etc.
- And much more!

### Quick Examples

```bash
# Start interactive mode
~/pepebot agent

# Try these commands:
> Check my battery percentage
> Take a photo with back camera
> What's my current GPS location?
> Send me a notification saying "Hello"
> What's in my clipboard?
> Speak "Hello from Pepebot"
```

**📚 Complete Documentation**: See [skills/termux-api](https://github.com/anak10thn/pepebot/tree/main/skills/termux-api) for full API reference and automation examples.

## 🤖 Using with Telegram Bot

Perfect for running Telegram bot on Android:

1. Setup Telegram bot with @BotFather
2. Configure in `config.json`:

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "YOUR_BOT_TOKEN",
      "allow_from": ["YOUR_TELEGRAM_USER_ID"]
    }
  }
}
```

3. Run as gateway:
```bash
~/pepebot gateway
```

4. Message your bot on Telegram!

## 📊 Performance

### Recommended Models for Android

**Fast & Free:**
- `maia/gemini-2.5-flash` (Recommended)
- `maia/llama-3.3-70b`

**Balanced:**
- `maia/gemini-3-pro-preview`
- `maia/claude-3-5-haiku`

**High Quality:**
- `maia/claude-3-5-sonnet`
- `maia/gpt-4o-mini`

### Resource Usage

- **Binary Size**: ~10MB
- **RAM Usage**: 50-150MB (depending on model)
- **Storage**: ~100MB (with workspace)
- **Network**: Varies by usage

## 🐛 Troubleshooting

### "Permission denied" error

```bash
chmod +x ~/pepebot
```

### "Cannot connect to server"

Check internet connection:
```bash
ping -c 3 google.com
```

### Termux keeps closing

Disable battery optimization for Termux in Android settings.

### Out of storage

Clear Termux cache:
```bash
apt clean
pkg clean
```

### Commands not found

Reinstall required packages:
```bash
pkg update
pkg install wget tar -y
```

## 🔄 Updating Pepebot

```bash
# Backup config
cp ~/.pepebot/config.json ~/config.json.backup

# Download new version
cd ~
rm pepebot
wget https://github.com/anak10thn/pepebot/releases/latest/download/pepebot-android-arm64.tar.gz
tar xzf pepebot-android-arm64.tar.gz
chmod +x pepebot-android-arm64
mv pepebot-android-arm64 pepebot

# Restore config
cp ~/config.json.backup ~/.pepebot/config.json
```

## 📚 Examples

### Personal AI Assistant

```bash
# Start interactive chat
~/pepebot agent

# Ask questions
> What's the weather in Jakarta?
> Write a Python script to parse JSON
> Explain quantum computing
```

### Telegram Bot

```bash
# Run as Telegram bot
~/pepebot gateway

# Bot runs in background
# Message your bot on Telegram
# Bot responds with AI-powered answers
```

### File Operations

```bash
# Start interactive mode
~/pepebot agent

# Commands in chat:
> Create a file named notes.txt with my shopping list
> Read the content of notes.txt
> Search for Python files in current directory
```

## 🌟 Advanced Usage

### Custom Skills

Create custom skills in `~/.pepebot/workspace/skills/`:

```bash
mkdir -p ~/.pepebot/workspace/skills/my-skill
nano ~/.pepebot/workspace/skills/my-skill/SKILL.md
```

### Web Search

Enable web search with Brave API:

```json
{
  "tools": {
    "web": {
      "search": {
        "api_key": "YOUR_BRAVE_API_KEY",
        "max_results": 5
      }
    }
  }
}
```

Get API key: https://brave.com/search/api/

## 🔗 Links

- **Termux Documentation**: https://wiki.termux.com
- **Pepebot Docs**: https://github.com/anak10thn/pepebot
- **MAIA Router**: https://maiarouter.ai
- **Support**: https://github.com/anak10thn/pepebot/issues

## 🎉 Success!

You now have a powerful AI assistant running natively on your Android device! 🐸

For more help, visit our [GitHub repository](https://github.com/anak10thn/pepebot).

---

Made with 🐸 by Pepebot Contributors
