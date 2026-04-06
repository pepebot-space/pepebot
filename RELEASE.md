# 🐸 Pepebot v0.5.12 - Workflow Goals & Self-Updating Skills

**Release Date:** 2026-04-06

## 🎉 What's New

### 🔄 `pepebot update` Now Updates Builtin Skills

Previously `pepebot update` only replaced the binary. Now it also pulls the latest builtin skills from GitHub automatically.

```bash
pepebot update                # Update binary + builtin skills
pepebot update --only-binary  # Binary only (old behavior)
pepebot update --only-skills  # Skills only
```

### 🧠 Workflow Goal Steps Fixed

Goal steps in workflows now properly send the goal to the LLM and store the result for subsequent steps. Previously the raw goal text was passed through instead of the AI-generated output.

**Before (broken):**
```
discord message: "Buat pesan pengingat untuk tim ops..."  ← raw goal text
```

**After (fixed):**
```
discord message: "Halo Tim Operation! Jangan lupa cek #bugs-war ya. ..."  ← LLM output
```

Use `{{step_name_output}}` to reference goal results in later steps:
```json
{
  "name": "send_reminder",
  "tool": "discord_send",
  "args": {
    "channel_id": "123",
    "content": "{{generate_message_output}}"
  }
}
```

### ✨ Cleaner Goal Output

Goal steps now produce clean output without LLM preamble like "Tentu, ini draf pesannya..." — the AI outputs the requested content directly.

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
