# 🐸 Pepebot v0.5.3 - LLM Support for Workflow CLI

**Release Date:** 2026-02-22

## 🎉 What's New

### 🧠 LLM Support for `goal` steps in Workflow CLI

You can now execute workflows that contain `goal` steps directly from the terminal! Previously, `goal` steps required the full agent loop (`pepebot agent -m "run workflow X"`).

With v0.5.3, the `pepebot workflow` command integrates a dedicated `cliGoalProcessor` that uses your default configured LLM provider to process goals on the fly. 

```bash
# Run a complex workflow with goal steps directly
pepebot workflow run ui_automation
pepebot workflow run -f /tmp/smart_workflow.json
```
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
Download the appropriate binary for your platform from the [releases page](https://github.com/pepebot-space/pepebot/releases/tag/v0.5.1).

## 🚀 Quick Start

1. **Connect your Android device via USB** (enable USB debugging)
2. **Start recording:**
   ```bash
   pepebot agent -m "Record my actions as a workflow named my_recording"
   ```
3. **Interact with your device** (tap, swipe)
4. **Press Volume Down** to stop
5. **Replay:**
   ```bash
   pepebot agent -m "Execute workflow my_recording"
   ```

## 📚 Documentation

- [Workflow Documentation](https://github.com/pepebot-space/pepebot/blob/main/docs/workflows.md)
- [Installation Guide](https://github.com/pepebot-space/pepebot/blob/main/docs/install.md)
- [API Reference](https://github.com/pepebot-space/pepebot/blob/main/docs/api.md)
- [Full Changelog](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md)

## 🔗 Links

- **GitHub**: https://github.com/pepebot-space/pepebot
- **Documentation**: https://github.com/pepebot-space/pepebot/tree/main/docs
- **Issues**: https://github.com/pepebot-space/pepebot/issues
- **Discussions**: https://github.com/pepebot-space/pepebot/discussions

## 📝 Full Changelog

For a complete list of changes, see [CHANGELOG.md](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md#051---2026-02-21).

---

**Note:** When upgrading from v0.5.0, all existing configurations, workflows, and data are preserved. No migration needed.
