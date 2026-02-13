# Pepebot v0.3.1 Release Notes

**Release Date:** 2026-02-13

## 🎯 Highlights

This release brings native WhatsApp Web support, eliminating the need for external bridge services. Setup is now as simple as scanning a QR code!

### ✨ What's New

**Native WhatsApp Integration**
- ✅ Direct WhatsApp Web connection (no bridge needed!)
- ✅ QR code login in terminal
- ✅ Persistent sessions via SQLite
- ✅ Pure Go implementation (works everywhere)
- ✅ Better stability and performance

### 🔧 Changes

**Simplified WhatsApp Setup**
- Old way: Install bridge → Configure WebSocket → Connect
- New way: Run pepebot → Scan QR code → Done!

**Config Changes**
- Removed: `bridge_url` (no longer needed)
- Added: `db_path` (session storage location)
- Default path: `~/.pepebot/whatsapp.db`

### 🐛 Bug Fixes

- Fixed "default agent not found" error in agent registry
- Default agent now guaranteed to exist on startup

### 🔄 Migration Guide

**For Existing WhatsApp Users:**

1. Update to v0.3.1
2. Run `pepebot gateway`
3. Scan QR code when prompted
4. Session saved automatically

Old `bridge_url` in config will be ignored (safe to leave or manually remove).

### 📦 What's Included

**New Dependencies:**
- `go.mau.fi/whatsmeow` - WhatsApp Web client
- `modernc.org/sqlite` - Pure Go SQLite (no CGO)
- `github.com/skip2/go-qrcode` - Terminal QR codes

**Removed Dependencies:**
- `gorilla/websocket` - No longer needed

### 🚀 Getting Started

**First Time Setup:**
```bash
# 1. Download/update pepebot
go install github.com/anak10thn/pepebot/cmd/pepebot@v0.3.1

# 2. Configure (if not already)
pepebot onboard

# 3. Start gateway
pepebot gateway

# 4. Scan QR code with WhatsApp
# Open WhatsApp > Linked Devices > Link a Device
```

**Subsequent Runs:**
```bash
pepebot gateway
# No QR code - auto-connects!
```

### 📝 Full Changelog

See [CHANGELOG.md](CHANGELOG.md) for complete details.

### 🙏 Credits

Built with:
- [whatsmeow](https://github.com/tulir/whatsmeow) by tulir
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) by cznic
- [go-qrcode](https://github.com/skip2/go-qrcode) by skip2

---

**Download:** [GitHub Releases](https://github.com/anak10thn/pepebot/releases/tag/v0.3.1)

**Support:** [Issues](https://github.com/anak10thn/pepebot/issues) • [Discussions](https://github.com/anak10thn/pepebot/discussions)
