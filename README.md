<div align="center">
<img src="assets/logo.webp" alt="Pepebot" width="512">

<h1>🐸 Pepebot</h1>
<h3>Ultra-Lightweight Personal AI Agent in Go</h3>

<p>
<img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go">
<img src="https://img.shields.io/badge/Arch-x86__64%2C%20ARM64%2C%20RISC--V-blue" alt="Hardware">
<img src="https://img.shields.io/badge/license-MIT-green" alt="License">
</p>

</div>

## 📋 Description

Pepebot is an ultra-lightweight and efficient personal AI agent. Pepebot is designed to provide a powerful AI assistant experience while maintaining minimal resource usage.

## ✨ Key Features

- 🤖 **Multi-Provider LLM**: Support for various AI providers including [MAIA Router](https://maiarouter.ai) (recommended), Anthropic, OpenAI, OpenRouter, Groq, Zhipu, Gemini, and vLLM
- 🌏 **Indonesian-Friendly**: MAIA Router integration with QRIS payment support and 52+ free models
- 💬 **Multi-Channel**: Integration with Telegram, Discord, WhatsApp, MaixCam, and Feishu
- 🛠️ **Tools System**: Filesystem operations, shell execution, web search, and more
- 🎯 **Skills System**: Customizable and extensible skill system
- 🚀 **Lightweight & Fast**: Small binary size with high performance
- 🔧 **Gateway Server**: HTTP server for custom integrations
- 💻 **CLI Interface**: Interactive command-line interface
- 🎙️ **Voice Support**: Audio/voice message transcription

## 📦 Installation

### Prerequisites

- Go 1.21 or higher
- Git

### Build from Source

```bash
# Clone repository
git clone https://github.com/anak10thn/pepebot.git
cd pepebot

# Build binary
make build

# Install to system (default: ~/.local/bin)
make install
```

### Build for Other Platforms

```bash
# Build for all platforms
make build-all
```

Supported platforms:
- Linux (x86_64, ARM64, RISC-V)
- macOS (x86_64, ARM64)
- Windows (x86_64)
- **Android (ARM64)** 📱 - See [ANDROID.md](ANDROID.md) for Termux setup

### Build for Android

```bash
# Build Android binary
make build-android
```

For detailed Android setup instructions, see **[ANDROID.md](ANDROID.md)**.

## ⚙️ Configuration

### 1. Create Configuration File

```bash
# Copy configuration template
cp config.example.json ~/.pepebot/config.json

# Edit as needed
nano ~/.pepebot/config.json
```

### 2. Configuration Structure

#### Agent Configuration

```json
{
  "agents": {
    "defaults": {
      "workspace": "~/.pepebot/workspace",
      "model": "maia/gemini-3-pro-preview",
      "max_tokens": 8192,
      "temperature": 0.7,
      "max_tool_iterations": 20
    }
  }
}
```

The default model is set to `maia/gemini-3-pro-preview` which uses MAIA Router. You can change this to any supported model from the providers below.

#### Provider Configuration

**MAIA Router (Recommended)**

[MAIA Router](https://maiarouter.ai) is a universal AI gateway that provides access to 200+ AI models (including 52+ free models) through a single OpenAI-compatible API. Perfect for Indonesian users with local payment support (QRIS).

```json
{
  "agents": {
    "defaults": {
      "model": "maia/gemini-3-pro-preview"
    }
  },
  "providers": {
    "maiarouter": {
      "api_key": "YOUR_MAIA_API_KEY",
      "api_base": "https://api.maiarouter.ai/v1"
    }
  }
}
```

To get your API key:
1. Visit [maiarouter.ai](https://maiarouter.ai) or [router.maia.id](https://router.maia.id)
2. Create an account
3. Generate your API key from the dashboard

Popular models available:
- `maia/gemini-3-pro-preview` (Recommended, free tier available)
- `maia/gemini-2.5-flash`
- `maia/claude-3-5-sonnet`
- `maia/gpt-4o`
- And 200+ more models

**Anthropic (Claude)**
```json
{
  "providers": {
    "anthropic": {
      "api_key": "sk-ant-xxx",
      "api_base": ""
    }
  }
}
```

**OpenAI**
```json
{
  "providers": {
    "openai": {
      "api_key": "sk-xxx",
      "api_base": ""
    }
  }
}
```

**OpenRouter**
```json
{
  "providers": {
    "openrouter": {
      "api_key": "sk-or-v1-xxx",
      "api_base": ""
    }
  }
}
```

**Groq**
```json
{
  "providers": {
    "groq": {
      "api_key": "gsk_xxx",
      "api_base": ""
    }
  }
}
```

**Zhipu (GLM)**
```json
{
  "providers": {
    "zhipu": {
      "api_key": "xxx",
      "api_base": ""
    }
  }
}
```

#### Channel Configuration

**Telegram Bot**
```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
      "allow_from": ["123456789"]
    }
  }
}
```

**Discord Bot**
```json
{
  "channels": {
    "discord": {
      "enabled": true,
      "token": "MTIzNDU2Nzg5MDEyMzQ1Njc4OQ.ABCDEF.xxxxxxxxxxxxxxxxxxxxxxxx",
      "allow_from": ["user_id_1", "user_id_2"]
    }
  }
}
```

**WhatsApp (via Bridge)**
```json
{
  "channels": {
    "whatsapp": {
      "enabled": true,
      "bridge_url": "ws://localhost:3001",
      "allow_from": ["628123456789@s.whatsapp.net"]
    }
  }
}
```

**MaixCam (IoT Device)**
```json
{
  "channels": {
    "maixcam": {
      "enabled": true,
      "host": "0.0.0.0",
      "port": 18790,
      "allow_from": []
    }
  }
}
```

**Feishu (Lark)**
```json
{
  "channels": {
    "feishu": {
      "enabled": true,
      "app_id": "cli_xxx",
      "app_secret": "xxx",
      "encrypt_key": "xxx",
      "verification_token": "xxx",
      "allow_from": []
    }
  }
}
```

#### Web Search Configuration

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

#### Gateway Configuration

```json
{
  "gateway": {
    "host": "0.0.0.0",
    "port": 18790
  }
}
```

## 🚀 Usage

### CLI Mode (Interactive)

```bash
pepebot
```

Then type your commands or questions:

```
🐸 > Hello! How are you?
🐸 > Create a Python script for web scraping
🐸 > /weather Jakarta
```

### Bot Mode (Daemon)

Run with configured channels:

```bash
# Telegram bot
pepebot

# Or use systemd for auto-start
sudo systemctl enable pepebot
sudo systemctl start pepebot
```

### Environment Variables

```bash
# Set model manually
export PEPEBOT_MODEL="claude-3-5-sonnet-20241022"

# Set workspace directory
export PEPEBOT_WORKSPACE="~/my-workspace"

# Set config path
export PEPEBOT_CONFIG="~/my-config.json"
```

## 🎯 Skills

Pepebot has an extensible skill system. Skills are prompt templates that provide special capabilities to the bot.

### Built-in Skills

1. **github** - GitHub operations and automation
2. **summarize** - Summarize text or documents
3. **tmux** - Tmux session management
4. **weather** - Weather information
5. **skill-creator** - Create new skills

### Using Skills

```bash
# In CLI
🐸 > /weather Jakarta

# Via bot (Telegram/Discord)
/weather Jakarta
```

### Creating New Skills

1. Create a new directory at `~/.pepebot/workspace/skills/my-skill/`
2. Create a `SKILL.md` file with the format:

```markdown
---
name: my-skill
description: My skill description
enabled: true
---

# My Skill Prompt

This is the prompt for my skill.

## Parameters

- param1: Description of parameter 1
- param2: Description of parameter 2
```

3. Reload or restart the bot to use the new skill

### Install Skills to Workspace

```bash
make install-skills
```

## 🔧 Development

### Project Structure

```
pepebot/
├── cmd/pepebot/          # Main application
├── pkg/
│   ├── agent/            # Agent logic & tool execution
│   ├── bus/              # Event bus for communication
│   ├── channels/         # Channel integrations
│   ├── config/           # Configuration management
│   ├── cron/             # Scheduled tasks
│   ├── heartbeat/        # Health monitoring
│   ├── logger/           # Logging system
│   ├── providers/        # LLM provider interfaces
│   ├── session/          # Session management
│   ├── skills/           # Skills loader & installer
│   ├── tools/            # Tool implementations
│   └── voice/            # Voice transcription
├── skills/               # Built-in skills
├── assets/               # Logo and assets
├── config.example.json   # Configuration template
└── Makefile             # Build automation
```

### Build Commands

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Format code
make fmt

# Update dependencies
make deps

# Clean build artifacts
make clean

# Build and run
make run

# Show help
make help
```

### Testing

```bash
# Run tests (if available)
go test ./...

# Run with verbose output
go test -v ./...
```

## 📝 Examples

### Basic Conversation

```bash
🐸 > Explain Go channels
```

### File Operations

```bash
🐸 > Create a hello.py file with a hello world program
🐸 > Read config.json and explain its structure
```

### Web Search

```bash
🐸 > Search for the latest information about Go 1.22
```

### Shell Commands

```bash
🐸 > Run command: ls -la
🐸 > Check the status of this git repository
```

## 🔒 Security Notes

- **API Keys**: Don't commit `config.json` file to git
- **Allow List**: Use `allow_from` to restrict access
- **Permissions**: Tools have access to filesystem and shell
- **Network**: Gateway server is exposed on the network (watch your firewall)

## 🤝 Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Create a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Inspired by [nanobot](https://github.com/HKUDS/nanobot) from HKUDS
- Built with ❤️ using Go

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/anak10thn/pepebot/issues)
- **Discussions**: [GitHub Discussions](https://github.com/anak10thn/pepebot/discussions)

---

<div align="center">
Made with 🐸 by Pepebot Contributors
</div>
