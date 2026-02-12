# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.3] - 2026-02-12

### Added
- **Verbose Logging Option**: Added `--verbose` flag to `gateway` command
  - Enables DEBUG level logging for detailed diagnostics
  - Shows incoming chat messages, API calls, and internal operations
  - Usage: `pepebot gateway --verbose` or `pepebot gateway -v`
  - Helpful for debugging and monitoring bot activity

### Fixed
- **Discord Image Sending**: Fixed images being sent to private messages instead of target channel
  - Added conversation context (channel and chat_id) to system prompt
  - AI now knows the correct chat_id to use when calling send_image tool
  - Prevents accidental PM uploads when user requests channel uploads
- **Discord Reply Image Reading**: Bot can now read and analyze images from replied messages
  - Added support for reading attachments from referenced messages (replies)
  - Previously only read attachments from current message
  - Enables image analysis when user replies to a message containing images

### Changed
- **Agent Context Builder**: Enhanced `BuildMessages()` to accept metadata parameter
  - System prompt now includes "Current Conversation Context" section
  - Provides clear instructions to AI about active channel and chat_id
- **Discord Message Handler**: Improved attachment processing
  - Checks both current message and referenced message for attachments
  - Better indication of attachment source in content text

### Technical Details
- Modified `pkg/agent/context.go`:
  - Updated `BuildMessages()` signature to include `metadata map[string]string`
  - Added conversation context to system prompt when metadata is available
- Modified `pkg/agent/loop.go`:
  - Updated `processMessage()` to ensure metadata includes channel information
  - Passes metadata to `BuildMessages()` for context awareness
- Modified `pkg/channels/discord.go`:
  - Added check for `m.ReferencedMessage.Attachments`
  - Processes attachments from both current and replied messages

## [0.2.2] - 2026-02-12

### Removed
- **Android x86_64 Support**: Removed x86_64/amd64 builds for Android
  - Keeping only ARM64 build which works on most Android devices
  - Simplified build matrix and reduced release artifacts
  - Removed from GitHub Actions CI/CD workflows
  - Updated documentation to reflect ARM64-only support

### Changed
- **Documentation**: Updated README.md and ANDROID.md
  - Simplified installation instructions for Android
  - Removed x86_64 download sections
  - Streamlined architecture detection steps
- **Makefile**: Simplified `build-android` target to ARM64 only
  - Removed x86_64 build commands
  - Cleaner build output

### Technical Details
- Removed android/amd64 from `.github/workflows/release.yml`
- Removed android/amd64 from `.github/workflows/ci.yml`
- Updated Makefile `build-android` and `build-all` targets
- Cleaned up documentation references to x86_64

## [0.2.1] - 2026-02-12

### Fixed
- **Android x86_64 Build Error**: Fixed CGO compilation error in GitHub Actions
  - Disabled CGO for all Android builds (ARM64 and x86_64)
  - Removed Android NDK dependency requirement
  - Fixed `android/log.h: No such file or directory` error
  - Simplified build process with static compilation

### Changed
- **Makefile**: Updated `build-android` target to build both ARM64 and x86_64 locally
  - Removed outdated CGO/NDK requirement notes
  - Added explicit `CGO_ENABLED=0` for Android builds
- **CI/CD**: Simplified release workflow by removing conditional CGO logic

## [0.2.0] - 2026-02-12

### Added
- **Multimodal Vision Support**: Bot can now read and analyze images from Discord
  - Support for OpenAI Vision API format with ContentBlock
  - Automatic image attachment processing
  - Compatible with multimodal LLM providers (GPT-4V, Claude 3, Gemini Pro Vision, etc.)
  - Images are passed directly to AI for analysis
- **Image Sending Capability**: Bot can now send images to Discord
  - New `SendImageTool` for AI to send images
  - Automatic image download from URLs
  - Support for local file paths
  - Upload to Discord CDN with proper file handling
  - Optional caption support for sent images
- **Android Platform Support**: Native Android binaries via Termux
  - ARM64 binary for modern Android devices (Android 5.0+)
  - x86_64 binary for Android emulators and x86 tablets
  - Automated GitHub Actions builds for Android
  - Comprehensive Android setup documentation (ANDROID.md)
  - Performance optimizations for mobile devices
- **Termux API Skill**: Complete Android device control integration
  - 30+ Termux API commands for hardware and system access
  - Battery monitoring and power management
  - Camera control (take photos with front/back camera)
  - Clipboard read/write operations
  - GPS and network location services
  - System notifications and alerts
  - Text-to-speech and speech-to-text
  - WiFi scanning and connection info
  - Sensor data access (accelerometer, gyroscope, light, etc.)
  - SMS and phone call capabilities
  - Storage and file system access
  - System controls (brightness, volume, vibration)
  - Complete documentation with 20+ automation examples

### Changed
- **Message Content Structure**: Updated from string to interface{} for multimodal support
  - Added `ContentBlock` type for text and image content
  - Added `ImageURL` type for vision API compatibility
  - Enhanced `buildUserMessage()` to format multimodal content
- **Discord Media Handling**: Improved file attachment processing
  - New `sendWithMedia()` method for file uploads
  - Enhanced `downloadMedia()` for URL and local file handling
  - Better error handling for media operations
- **Agent System Prompt**: Updated to include image sending capabilities
- **OutboundMessage Structure**: Added `Media []string` field for attachments

### Fixed
- **Vision Not Working**: Fixed media not being passed to LLM
  - Changed `nil` to `msg.Media` in processMessage() call
  - Properly propagate media attachments through agent loop
- **Content Length Calculation**: Fixed for interface{} content type
  - Added `getContentLength()` helper for proper type switching
  - Handles both string and []ContentBlock content

### Technical Details
- Modified `pkg/providers/types.go` for multimodal message structure
- Enhanced `pkg/agent/context.go` with vision API formatting
- Updated `pkg/agent/loop.go` for proper media handling
- Implemented `pkg/tools/send_image.go` for image sending
- Extended `pkg/channels/discord.go` with media upload support
- Added Android build targets to CI/CD workflows
- Created comprehensive Termux API skill documentation

### Platform Support
- **New**: Android (ARM64, x86_64) via Termux
- Existing: Linux, macOS, Windows, FreeBSD
- Architectures: x86_64, ARM64, ARMv6, ARMv7, RISC-V, MIPS variants

## [0.1.2] - 2026-02-11

### Added
- **Discord Typing Indicator**: Bot now displays "is typing..." status while processing messages
  - Automatic typing indicator when bot receives a message
  - Auto-refresh every 8 seconds to keep indicator active
  - Automatically stops when response is sent
  - Timeout protection (2 minutes max)
  - Thread-safe implementation with mutex for concurrent channels
- Enhanced user experience with real-time visual feedback

### Changed
- Improved Discord message handling flow
- Updated Discord channel implementation with typing state management

### Technical Details
- Added `typingChannels` map to track active typing indicators
- Implemented `keepTyping()` goroutine for continuous typing refresh
- Added `stopTyping()` and `storeTypingChannel()` helper methods
- Thread-safe typing indicator management using `sync.RWMutex`

## [0.1.1] - 2026-02-10

### Added
- **Docker Support**: Complete Docker configuration for containerized deployment
  - Multi-stage Dockerfile for optimal image size
  - Docker Compose configuration for easy deployment
  - Health check support
  - Volume mounting for persistent data
- **BUILD.md**: Comprehensive build and deployment documentation

### Changed
- **Discord Bot Improvements**:
  - Enhanced message handling logic
  - Better mention detection
  - Improved reply-to-message handling
  - Added frog emoji (🐸) reaction to acknowledged messages
  - Optimized message splitting for long responses

### Fixed
- Discord bot message processing reliability
- Message handling in group channels vs DMs

## [0.1.0] - 2026-02-09

### Added
- **MAIA Router Integration**: Added support for MAIA Router as the recommended AI provider
  - Access to 200+ AI models including 52+ free models
  - Indonesian-friendly with QRIS payment support
  - Set as default model: `maia/gemini-3-pro-preview`
  - OpenAI-compatible API interface
- **Comprehensive Documentation**:
  - Restructured README with clear sections
  - Added provider configuration examples
  - Included quick start guide
  - Multi-platform build instructions

### Changed
- **Branding Update**: Complete rebrand from picoclaw to Pepebot
  - Replaced lobster emoji (🦞) with frog emoji (🐸) throughout codebase
  - Updated all documentation and references
  - New logo and visual identity
  - Simplified project description
  - Removed outdated nanobot references

### Technical Details
- Optimized asset files and documentation structure
- Updated configuration examples for better clarity
- Enhanced provider selection in onboarding wizard

## [0.0.1] - 2026-02-08

### Added
- Initial project setup as Pepebot
- Multi-provider LLM support (Anthropic, OpenAI, OpenRouter, Groq, Zhipu, Gemini, vLLM)
- Multi-channel integration (Telegram, Discord, WhatsApp, MaixCam, Feishu)
- Tools system (filesystem operations, shell execution, web search)
- Skills system (customizable and extensible)
- CLI interface with interactive mode
- Gateway server for custom integrations
- Voice/audio message transcription support
- Session management
- Configuration system with JSON and environment variable support
- Workspace management
- Memory system for context persistence

### Features
- Ultra-lightweight design (~10MB binary)
- High performance with minimal resource usage
- Cross-platform support (Linux, macOS, Windows)
- Multi-architecture support (x86_64, ARM64, RISC-V)
- MIT License - Free and open source

---

## Legend

- `Added` for new features
- `Changed` for changes in existing functionality
- `Deprecated` for soon-to-be removed features
- `Removed` for now removed features
- `Fixed` for any bug fixes
- `Security` for vulnerability fixes

---

For more details, see the [README](README.md) and [BUILD](BUILD.md) documentation.
