# 🐸 Pepebot v0.4.2 - ADB Tools Overhaul

**Release Date:** 2026-02-18

## 🎉 What's New

### 📱 ADB Tools - Completely Rewritten

All ADB tools have been overhauled for reliability, inspired by the [phone-use skill](https://github.com/pepebot-space/skills/tree/main/phone-use) approach. If you've been experiencing errors with Android automation — this release fixes them.

### 🆕 New Tools

**`adb_open_app`** - Launch apps by package name
```
adb_open_app(package: "com.android.settings")
```
- Smart launcher: tries `am start`, falls back to `monkey`
- No more manual shell commands to open apps!

**`adb_keyevent`** - Send hardware key events
```
adb_keyevent(keycode: 4)  → BACK
adb_keyevent(keycode: 3)  → HOME
```
- Supports all Android keycodes with human-readable names

### 🔧 Major Improvements

**Screenshot** (`adb_screenshot`):
- ⚡ **3x faster**: Direct PNG capture via `exec-out` (was: screencap → pull → rm)
- ✅ PNG signature validation
- ✅ Returns base64 when no filename given
- ✅ No more `/sdcard` write permission errors

**UI Dump** (`adb_ui_dump`):
- ✅ Multiple path fallback (`/sdcard/` → `/data/local/tmp/`)
- ✅ `exec-out cat` with `shell cat` fallback
- ✅ XML structure validation
- ✅ Works on more devices and Android versions

**Text Input** (`adb_input_text`):
- ✅ Proper escaping for 20+ shell metacharacters (`$`, `&`, `|`, quotes, etc.)
- ✅ Auto-chunking at 80 chars (no more length limit errors)
- ✅ Multi-line support with automatic Enter keys
- ✅ New `press_enter` option

**Tap** (`adb_tap`):
- ✅ New `long_press` mode (hold 550ms)
- ✅ New `count` for double-tap / multi-tap

**Swipe** (`adb_swipe`):
- ✅ New `direction` mode: just say `up`, `down`, `left`, `right`
- ✅ Coordinate-based swipe still works (backward compatible)
- ✅ More natural default duration (220ms)

### 🐛 Bug Fixes

- Fixed screenshot failures from race conditions in 3-step capture process
- Fixed UI dump errors on devices where `/sdcard` is read-only
- Fixed special characters breaking text input (`$`, `&`, `|`, `;`, quotes)
- Fixed long text input failures from ADB command length limits

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
Download the appropriate binary for your platform from the [releases page](https://github.com/pepebot-space/pepebot/releases/tag/v0.4.2).

## 🚀 Quick Start

1. **Initialize configuration:**
   ```bash
   pepebot onboard
   ```

2. **Start the gateway:**
   ```bash
   pepebot gateway
   ```

3. **Try Android automation:**
   ```
   "Open Settings on my phone"
   "Take a screenshot"
   "Tap the Wi-Fi option"
   "Scroll down"
   ```

## 📚 Documentation

- [Installation Guide](https://github.com/pepebot-space/pepebot/blob/main/docs/install.md)
- [Workflow Documentation](https://github.com/pepebot-space/pepebot/blob/main/docs/workflows.md)
- [API Reference](https://github.com/pepebot-space/pepebot/blob/main/docs/api.md)
- [Full Changelog](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md)

## 🔗 Links

- **GitHub**: https://github.com/pepebot-space/pepebot
- **Documentation**: https://github.com/pepebot-space/pepebot/tree/main/docs
- **Issues**: https://github.com/pepebot-space/pepebot/issues
- **Discussions**: https://github.com/pepebot-space/pepebot/discussions

## 📝 Full Changelog

For a complete list of changes, see [CHANGELOG.md](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md#042---2026-02-18).

---

**Note:** When upgrading from v0.4.1, all existing configurations and data are preserved. No migration needed. New tools (`adb_open_app`, `adb_keyevent`) are automatically registered when ADB is available.
