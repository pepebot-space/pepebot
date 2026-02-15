# Pepebot v0.2.3 Release Notes 🐸

## What's New

### 🔧 Bug Fixes

#### Discord Image Sending Fixed
Fixed a critical issue where images were being sent to private messages instead of the target channel.

**What was fixed:**
- ✅ Images now correctly sent to the intended channel
- ✅ Added conversation context (channel and chat_id) to system prompt
- ✅ AI now knows the correct chat_id to use when calling send_image tool
- ✅ Prevents accidental PM uploads when user requests channel uploads

#### Discord Reply Image Reading
Bot can now read and analyze images from replied messages!

**New capabilities:**
- 📸 Support for reading attachments from referenced messages (replies)
- 🔍 Previously only read attachments from current message
- 💬 Enables image analysis when user replies to a message containing images

### 🚀 Technical Improvements

- **Agent Context Builder**: Enhanced `BuildMessages()` to accept metadata parameter
  - System prompt now includes "Current Conversation Context" section
  - Provides clear instructions to AI about active channel and chat_id
- **Discord Message Handler**: Improved attachment processing
  - Checks both current message and referenced message for attachments
  - Better indication of attachment source in content text

### 📝 Modified Files

- `pkg/agent/context.go`: Updated `BuildMessages()` signature to include metadata
- `pkg/agent/loop.go`: Updated `processMessage()` to ensure metadata includes channel information
- `pkg/channels/discord.go`: Added check for `m.ReferencedMessage.Attachments`

## Installation

### Binary Download
Download the pre-built binary for your platform from the [releases page](https://github.com/pepebot-space/pepebot/releases/tag/v0.2.3).

### Build from Source
```bash
git clone https://github.com/pepebot-space/pepebot.git
cd pepebot
make build
```

### Docker
```bash
docker pull anak10thn/pepebot:0.2.3
# or
docker-compose up -d
```

## Upgrade Guide

If you're upgrading from v0.2.2:
1. Stop your current Pepebot instance
2. Replace the binary with the new version
3. Restart Pepebot
4. No configuration changes required!

```bash
# Stop the bot
pkill pepebot

# Replace binary
cp build/pepebot ~/.local/bin/pepebot

# Restart
pepebot gateway
```

## Configuration

No configuration changes are required for this release. All fixes work automatically.

## What's Next

Looking ahead to future releases:
- 🎨 More Discord integration improvements
- 🔍 Enhanced error handling
- 📊 Better logging and debugging
- 🚀 Performance optimizations

## Full Changelog

See [CHANGELOG.md](CHANGELOG.md) for complete version history.

## Breaking Changes

None! This release is fully backward compatible with v0.2.2.

## Contributors

Thanks to everyone who contributed to this release! 🎉

## Support

- 📖 [Documentation](README.md)
- 🐛 [Report Issues](https://github.com/pepebot-space/pepebot/issues)
- 💬 [Discussions](https://github.com/pepebot-space/pepebot/discussions)

---

**Note**: This is a bug fix release focused on improving Discord image handling. All existing features remain fully functional.

Made with 🐸 by Pepebot Contributors
