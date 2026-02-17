# 🐸 Pepebot v0.4.1 - Multimodal File Support

**Release Date:** 2026-02-17

## 🎉 What's New

### 📎 Multimodal File Support
Pepebot can now handle **all file types**, not just images! Send PDFs, documents, audio, and video files directly to your AI agent.

**Supported File Types:**
- 📄 **Documents**: PDF, DOC, DOCX, XLS, XLSX, PPT, PPTX, TXT, CSV, RTF, ODT, ODS, ODP
- 🎵 **Audio**: MP3, WAV, OGG, M4A, FLAC, AAC, WMA, OPUS
- 🎬 **Video**: MP4, AVI, MOV, WMV, FLV, WEBM, MKV, M4V, 3GP
- 🖼️ **Images**: JPG, PNG, GIF, WebP, BMP, SVG (already supported, now improved)

**Key Features:**
- ✅ Automatic file type detection (50+ MIME types)
- ✅ Smart base64 encoding for local files
- ✅ OpenAI-compatible API format
- ✅ New `send_file` tool for sending any file type

### 📱 Enhanced Channel Support
All chat channels now fully support media files!

**Telegram:**
- ✅ Send/receive images, videos, audio, documents
- ✅ Automatic file type detection
- ✅ Caption support with HTML formatting

**Discord:**
- ✅ Full media support (verified working)
- ✅ Multi-file attachments
- ✅ Automatic download for URLs and local files

**WhatsApp:**
- ✅ Send/receive images, videos, audio, documents
- ✅ Automatic media download from WhatsApp servers
- ✅ Caption extraction for media messages
- ✅ Proper handling of media without captions

### 🔧 Technical Improvements

**API Compatibility:**
- OpenAI-compliant file format (`file_data` field)
- Support for both base64 data URLs and uploaded file IDs
- Simplified ContentBlock structure for better maintainability

**Context Builder:**
- Automatic local file path → base64 conversion
- Smart file type detection
- Mixed content support (text + images + files)
- Enhanced logging for debugging

### 🐛 Bug Fixes

**WhatsApp Media Reception:**
- Fixed issue where images sent to WhatsApp bot were not processed
- Added proper media download handlers
- Caption extraction now works correctly

**LLM File Processing:**
- Fixed "Invalid image received" error for local file paths
- LLM providers now receive base64-encoded data URLs
- Compatible with Gemini, OpenAI, and other providers via LiteLLM

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
Download the appropriate binary for your platform from the [releases page](https://github.com/pepebot-space/pepebot/releases/tag/v0.4.1).

## 🚀 Quick Start

1. **Initialize configuration:**
   ```bash
   pepebot onboard
   ```

2. **Start the gateway:**
   ```bash
   pepebot gateway
   ```

3. **Send files to your bot:**
   - Send images, PDFs, audio, or video files
   - Bot will automatically process and analyze them
   - Works across Telegram, Discord, and WhatsApp!

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

## 🙏 Contributors

Thank you to everyone who contributed to this release!

## 📝 Full Changelog

For a complete list of changes, see [CHANGELOG.md](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md#041---2026-02-17).

---

**Note:** When upgrading from v0.4.0, all existing configurations and data are preserved. No migration needed!
