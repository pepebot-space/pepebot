# 🐸 Pepebot v0.5.11 - Workflow Goal Step Fix

**Release Date:** 2026-04-06

## 🎉 What's New

### 🔧 Workflow Goal Steps Now Work Correctly

Goal steps in workflows previously passed the raw goal text to the next step instead of the LLM-generated output. This caused workflows like:

```json
{
  "name": "generate_message",
  "goal": "Buat pesan pengingat dengan pantun..."
}
```

...to output the literal goal text instead of the AI-generated message.

**Now fixed:**
- Goal steps are processed by the LLM in both gateway and CLI modes
- Use `{{step_name_output}}` to get the LLM result in subsequent steps
- `{{step_name_goal}}` still available for the original goal text if needed

### 📝 Documentation Corrected

Updated workflow docs and skill prompts that previously recommended `{{step_name_goal}}` for getting goal results. The correct variable is `{{step_name_output}}`.

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
