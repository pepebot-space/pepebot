# 🐸 Pepebot v0.5.1 - Workflow Skill & Agent Steps + ADB Activity Recorder

**Release Date:** 2026-02-21

## 🎉 What's New

### 🧩 Workflow Skill Steps

Workflows can now load skill content and combine it with goal instructions! Use the `skill` field to bring specialized knowledge into your workflow steps.

```json
{
  "name": "analyze_with_skill",
  "skill": "workflow",
  "goal": "Using this skill's knowledge, analyze the data from {{collect_output}}"
}
```

### 🤝 Workflow Agent Steps

Delegate workflow goals to other registered agents! Use the `agent` field to leverage different models and prompt configurations within a single workflow.

```json
{
  "name": "research",
  "agent": "researcher",
  "goal": "Research {{topic}} and provide a summary"
}
```

Agent responses are stored as `{{step_name_output}}` for use in subsequent steps.

### 🎬 ADB Activity Recorder

Record your Android device interactions and automatically generate replayable workflow files! Simply use your device while Pepebot watches — taps and swipes are captured in real-time via ADB.

**How to use:**
```
User: "Record my Android actions as a workflow named login_flow"
```

- Pepebot starts listening to touch events on your device
- Perform taps, swipes, and other interactions normally
- Press **Volume Down** to stop recording
- A workflow JSON file is automatically generated and saved

**What gets captured:**
- Tap gestures (short touch, small movement)
- Swipe gestures (long touch with displacement)
- Final screenshot and UI dump for verification

**Generated workflows** use standard `adb_tap` and `adb_swipe` steps with a `{{device}}` variable, so you can replay on any connected device.

### Key Features
- Real-time touch event streaming via `getevent -l`
- Automatic input device discovery and screen resolution detection
- Smart gesture classification with debounce filtering
- Post-recording screenshot + UI dump for LLM-based verification
- Standard workflow format — edit, extend, or chain with other workflows

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
