# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
