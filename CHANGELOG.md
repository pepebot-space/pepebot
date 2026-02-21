# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.5.1] - 2026-02-21

### Added
- **Workflow Skill Steps**: Workflows can now load skill content and combine with goal instructions
  - New `skill` field on workflow steps that loads skill definitions at execution time
  - Combined skill content + goal stored as `{{step_name_output}}` for subsequent steps
  - Requires `goal` field alongside `skill`; cannot be combined with `tool` or `agent`
  - Uses existing SkillsLoader infrastructure (workspace + builtin skills)
- **Workflow Agent Steps**: Workflows can now delegate goals to other registered agents
  - New `agent` field on workflow steps for cross-agent collaboration
  - Agent processes goal with ephemeral session key, response stored as `{{step_name_output}}`
  - Requires `goal` field alongside `agent`; cannot be combined with `tool` or `skill`
  - Graceful failure in standalone mode (CLI without gateway/AgentManager)
  - Dependency injection via `WorkflowAgentProcessor` interface to avoid circular imports
- **Dashboard Workflow View**: Updated step display with 4 step type badges
  - Tool steps: blue badge (existing)
  - Skill steps: purple badge with skill name
  - Agent steps: amber badge with agent name
  - Goal steps: text display (existing)
- **Graceful Gateway Restart**: `/restart` command and `POST /v1/restart` API endpoint
  - Stops all services (HTTP server, channels, cron, heartbeat) then re-initializes from fresh config
  - Available via chat command `/restart` on any channel (Telegram, Discord, WhatsApp, etc.)
  - Available via HTTP API `POST /v1/restart` for dashboard/programmatic use
  - `SIGHUP` signal also triggers graceful restart (e.g. `kill -HUP <pid>`)
  - Config changes (model, API keys, channels) take effect without full process restart
- **ADB Activity Recorder** (`adb_record_workflow`): Generate workflow files by recording device interactions
  - Real-time touch event capture via `adb shell getevent -l` streaming
  - Automatic touch input device discovery and screen resolution detection
  - Event parsing state machine: BTN_TOUCH DOWN/UP, ABS_MT_POSITION_X/Y, SYN_REPORT
  - Raw-to-pixel coordinate mapping using device input range and screen dimensions
  - Gesture classification: taps (< 30px movement, < 300ms), swipes (>= 50px movement)
  - Time-based debounce filtering (200ms window) to eliminate jitter
  - Press Volume Down on device to stop recording
  - Post-recording screenshot and UI dump capture for verification
  - Generated workflow includes `adb_tap`/`adb_swipe` steps with `{{device}}` variable
  - Final `verify_final_state` goal step with screenshot path and UI dump for LLM verification
  - Configurable max recording duration (default: 300s)
- **ADB Streaming Helper** (`execAdbStreaming`): New `AdbHelper` method for long-running ADB commands
  - Returns running `*exec.Cmd` and `io.ReadCloser` for line-by-line stdout processing
  - Used by the activity recorder for continuous `getevent` output

### Technical Details
- Added `pkg/tools/adb_recorder.go`: Event parser, gesture classifier, coordinate mapper, and tool (~450 lines)
- Added `pkg/tools/adb_recorder_test.go`: 18 unit tests covering event parsing, coordinate mapping, gesture classification, debounce, workflow building, and device info parsing
- Modified `pkg/tools/adb.go`: Added `execAdbStreaming()` method and `io` import
- Modified `pkg/agent/loop.go`: Registered `NewAdbRecordWorkflowTool` in both `NewAgentLoop` and `NewAgentLoopWithDefinition`
- Updated `docs/workflows.md`: Added ADB Activity Recorder documentation section

## [0.5.0] - 2026-02-20

### Added
- **Local Dashboard Interface**: Fully rewritten Vue/Vite static dashboard for managing Pepebot
  - `pepebot gateway` now serves a modern Web UI connecting to the local API
  - Gateway configuration routing mechanism targeting `http://localhost:18790/v1`
  - Dashboard features Pages for Agents, Skills, Workflows, Configuration, and Sessions
  - Setup screen for configuring the API gateway URL and multi-gateway management
  - Ready for static deployment (Cloudflare Pages, Vercel, Netlify)
- **Floating AI Assistant (Frog Panel)**: Global access to Pepebot within the dashboard
  - Embedded chat interface that slides in from the right edge
  - Implemented SSE context streaming identical to terminal functionality
  - Context-aware prompting based on the active dashboard page (e.g. creating skills on the Skills page)
  - Full Markdown and Code syntax highlighting support in web chat
  - Image upload capabilities via Data URL/Base64 encoding directly from the browser

### Changed
- **Dashboard Architecture**: Complete restructure for static hosting
  - Removed intermediate Express.js proxy layer and SQLite uploads db
  - Moved Vite application from `client/` to root `dashboard/` workspace
  - Image payloads now converted locally via `FileReader` instead of an `api/upload` endpoint
- **Gateway Security**: Added CORS wildcard origins for cross-network Web UI access

## [0.4.3] - 2026-02-18

### Added
- **OpenAI-Compatible Gateway API**: Full HTTP API server with SSE streaming support
  - `POST /v1/chat/completions` — OpenAI-compatible chat endpoint with streaming (`stream: true`) and non-streaming modes
  - `GET /v1/models` — List available models from enabled agents
  - `GET /v1/agents` — List registered agents (raw registry.json)
  - `GET /v1/sessions` — List active web sessions (filtered by `web:` prefix)
  - `POST /v1/sessions/{key}/new` — Clear and create new chat session
  - `POST /v1/sessions/{key}/stop` — Stop in-flight LLM processing for a session
  - `DELETE /v1/sessions/{key}` — Delete a specific session
  - `GET /health` — Health check endpoint
  - CORS support for dashboard and browser-based clients
  - Custom headers: `X-Agent` (select agent) and `X-Session-Key` (session routing)
  - SSE streaming follows OpenAI format: `data: {"choices":[{"delta":{"content":"..."}}]}` with `data: [DONE]` termination

- **LLM Provider Streaming**: Added `ChatStream()` method to provider interface
  - SSE line-by-line parsing with `data:` prefix handling
  - `[DONE]` sentinel and `finish_reason: "stop"` detection
  - `StreamChunk` and `StreamCallback` types for streaming pipeline

- **Agent Stream Processing**: Added `ProcessDirectStream()` to agent loop
  - Tool iterations use non-streaming `Chat()` (tools are server-side, invisible to API client)
  - Final LLM response streams via `ChatStream()` directly to HTTP client
  - Session persistence identical to non-streaming `processMessage()`

- **Gateway Server Package**: New `pkg/gateway/` package
  - `server.go` — HTTP server lifecycle, route registration, CORS middleware
  - `handlers.go` — All endpoint handlers with OpenAI-compatible request/response types

### Changed
- **Gateway Command**: `pepebot gateway` now starts HTTP API server alongside chat channels
  - HTTP server starts on configured `gateway.host:gateway.port` (default: `0.0.0.0:18790`)
  - Graceful shutdown of HTTP server on SIGINT
- **AgentManager**: Added public methods for gateway integration
  - `ProcessDirectStream()`, `ProcessDirect()`, `ClearSession()`, `StopSession()`, `GetSessions()`, `GetConfig()`
- **SessionManager**: Added `ListSessions(prefix)` for filtered session listing
- **LLMProvider Interface**: Added `ChatStream()` method (implemented in `HTTPProvider`)

### Technical Details
- New files: `pkg/gateway/server.go`, `pkg/gateway/handlers.go`
- Modified: `pkg/providers/types.go` (StreamChunk, StreamCallback, ChatStream interface)
- Modified: `pkg/providers/http_provider.go` (ChatStream implementation)
- Modified: `pkg/agent/loop.go` (ProcessDirectStream, Sessions getter)
- Modified: `pkg/agent/manager.go` (gateway delegation methods)
- Modified: `pkg/session/manager.go` (ListSessions, DeleteSession)
- Modified: `cmd/pepebot/main.go` (gateway server integration)
- No external dependencies added — uses `net/http` stdlib

## [0.4.2] - 2026-02-18

### Added
- **ADB Open App Tool** (`adb_open_app`): Launch Android apps by package name
  - Uses `am start` with launcher intent, fallback to `monkey` launcher
  - Common package name reference in tool description
- **ADB Key Event Tool** (`adb_keyevent`): Send hardware key events to Android device
  - Supports all Android keycodes (Home, Back, Volume, Power, Enter, etc.)
  - Human-readable keycode name mapping in output

### Changed
- **ADB Screenshot Tool** (`adb_screenshot`): Rewritten for reliability
  - Uses `exec-out screencap -p` for direct PNG binary capture (was: shell screencap → pull → rm, 3 steps reduced to 1)
  - PNG signature validation to detect corrupt captures
  - Returns base64-encoded PNG when no filename is provided
  - Dedicated 15s timeout (was: generic 30s)
- **ADB UI Dump Tool** (`adb_ui_dump`): Rewritten for reliability
  - Multiple dump path fallback: `/sdcard/` → `/data/local/tmp/` → default path
  - Uses `exec-out cat` with fallback to `shell cat` for reading dump file
  - XML structure validation (`<hierarchy` or `<node` tag check)
  - Strips garbage content before `<?xml` declaration
  - 200ms delay after dump to ensure file is fully written
  - Increased timeout from 12s to 15s for uiautomator dump
  - Better error messages with debugging hints (screen locked, accessibility unavailable)
- **ADB Input Text Tool** (`adb_input_text`): Rewritten for reliability
  - Proper shell metacharacter escaping (20+ characters: `$`, `` ` ``, `"`, `'`, `&`, `|`, `;`, etc.)
  - Text chunking at 80 characters to avoid ADB input length limits
  - Multi-line text support with automatic Enter key between lines
  - New `press_enter` parameter to send Enter after input
  - Per-chunk timeout of 10s (was: single 30s for entire input)
- **ADB Tap Tool** (`adb_tap`): Enhanced with new gestures
  - New `long_press` parameter: simulates long press via swipe-to-same-point (550ms)
  - New `count` parameter: multi-tap support (e.g., double-tap with count=2)
  - Dedicated 8s timeout per tap (was: generic 30s)
- **ADB Swipe Tool** (`adb_swipe`): Enhanced with direction mode
  - New `direction` parameter: swipe by direction (`up`, `down`, `left`, `right`)
  - New `distance` parameter: swipe distance in pixels for direction mode (default: 500)
  - Backward compatible: coordinate-based swipe (x1,y1 → x2,y2) still works
  - Boundary clamping to prevent negative coordinates
  - Default duration changed from 300ms to 220ms for more natural feel
  - Renamed parameters: `x1`/`y1` → `x`/`y` (x2/y2 still available for coordinate mode)
- **ADB Devices Tool** (`adb_devices`): Enhanced output
  - Uses `devices -l` for additional device info (model, device name, transport)
  - Parses and includes key-value metadata in JSON output
- **ADB Helper**: Per-operation timeouts
  - `execAdb` now accepts timeout parameter instead of hardcoded 30s
  - New `execAdbBinary` method for binary output (used by exec-out commands)
  - Cleaner error messages: stderr-first, no stdout pollution in error output

### Fixed
- **ADB Screenshot**: Fixed frequent failures caused by 3-step capture process (screencap → pull → rm) which was prone to race conditions and /sdcard write permission issues
- **ADB UI Dump**: Fixed "failed to dump UI" errors on devices where /sdcard is read-only or uiautomator output goes to stderr
- **ADB Input Text**: Fixed special characters ($, &, |, ;, quotes, etc.) causing shell injection or broken input
- **ADB Input Text**: Fixed long text input failures caused by ADB command length limits

## [0.4.1] - 2026-02-17

### Added
- **Multimodal File Support**: Extended LLM multimodal capabilities beyond images
  - Support for documents (PDF, DOC, DOCX, XLS, XLSX, PPT, PPTX, TXT, CSV, RTF, ODT, ODS, ODP)
  - Support for audio files (MP3, WAV, OGG, M4A, FLAC, AAC, WMA, OPUS)
  - Support for video files (MP4, AVI, MOV, WMV, FLV, WEBM, MKV, M4V, 3GP)
  - Automatic file type detection via MIME type analysis (50+ MIME types)
  - File type detector utility (`pkg/providers/filetype.go`)
  - New tool: `send_file` - generic tool for sending all file types to chat channels
  - OpenAI-compatible API format for file attachments (`file_data` field)
  - Automatic base64 encoding for local file paths
  - Support for both base64 data URLs and uploaded file IDs

- **Channel Media Support**: All chat channels can now send and receive files
  - **Telegram**: Send/receive images, videos, audio, documents with auto-type detection
  - **Discord**: Full media support (already implemented, verified)
  - **WhatsApp**: Send/receive images, videos, audio, documents with proper download handling
  - Auto-download media from WhatsApp to `/tmp/pepebot_whatsapp/`
  - Base64 encoding for local files before sending to LLM providers

- **Enhanced Docker Image**: Production-ready container with utilities
  - Cron daemon support for scheduled tasks (Ubuntu `cron`)
  - Tmux for terminal multiplexing and session management
  - Systemctl replacement for service management ([docker-systemctl-replacement](https://github.com/gdraheim/docker-systemctl-replacement))
  - Entrypoint script that runs cron daemon alongside pepebot gateway
  - Example crontab configuration (`docker/crontab.example`)
  - Comprehensive Docker deployment guide (`docker/README.md`)
  - Based on Ubuntu 24.04 LTS for stability and compatibility
  - Common utilities included: vim, nano, htop, curl, ping, net-tools
  - GitHub Actions workflow fixed with `packages: write` permission for GHCR push

### Changed
- **ContentBlock Structure**: Simplified and standardized for API compatibility
  - Removed separate `DocumentURL`, `AudioURL`, `VideoURL` types
  - Unified all non-image files under `file` type with `FileData` struct
  - Images continue using `image_url` format for vision API compatibility
  - Added support for `file_id` (uploaded files) and `file_data` (base64) formats
  - Full compliance with OpenAI API specification for PDF files

- **Context Builder**: Enhanced multimodal message building
  - Automatic local file path to base64 data URL conversion
  - Smart file type detection and appropriate content block creation
  - Support for mixed content (text + images + files in single message)
  - Logging for media processing and conversion steps

### Fixed
- **WhatsApp Media Reception**: Fixed issue where images sent to WhatsApp bot were not processed
  - Added image/video/audio/document message handlers
  - Proper media download from WhatsApp servers
  - Caption extraction for media messages
  - Graceful handling of media without captions
- **LLM File Processing**: Fixed "Invalid image received" error for local file paths
  - LLM providers now receive base64-encoded data URLs instead of file paths
  - Proper MIME type detection and inclusion in data URLs
  - Compatible with Gemini, OpenAI, and other providers via LiteLLM

## [0.4.0] - 2026-02-16

### Added
- **ADB Tools**: Android device control via Android Debug Bridge
  - 7 new tools for Android automation: `adb_devices`, `adb_shell`, `adb_tap`, `adb_input_text`, `adb_screenshot`, `adb_ui_dump`, `adb_swipe`
  - Automatic ADB binary discovery (ANDROID_HOME or system PATH)
  - Silent graceful failure if ADB not installed
  - Support for multi-device targeting via device serial parameter
  - UI hierarchy inspection for automation workflows
  - Screenshot capture with workspace-relative paths

- **Workflow System**: Multi-step automation framework
  - 3 new tools: `workflow_execute`, `workflow_save`, `workflow_list`
  - Generic workflow engine (works with ANY tools, not limited to ADB)
  - JSON-based workflow definitions with variable interpolation
  - Step output tracking: `{{step_name_output}}` variables
  - Goal-based steps for LLM-driven automation
  - Variable override support at execution time
  - Example workflows included: UI automation, device health check, browser research

- **Example Workflows**: Pre-built automation templates
  - `ui_automation.json`: Android app login automation with coordinate-based interaction
  - `device_control.json`: Device health monitoring (battery, memory, storage, network)
  - `browser_automation.json`: Web research workflow demonstrating non-ADB use case

- **Builtin Skills Installation**: Automated skill installation from GitHub
  - New command: `pepebot skills install-builtin` downloads and installs all official skills
  - Downloads from `https://github.com/pepebot-space/skills-builtin` as ZIP archive
  - Automatic extraction and installation to workspace
  - Integrated into onboarding wizard (Step 5/5) with Yes as default
  - Graceful error handling with helpful messages

- **Skills Search Integration**: Centralized skill discovery
  - `pepebot skills search` now fetches from `https://github.com/pepebot-space/skills`
  - New registry format with metadata support
  - Displays skill name, description, and install command
  - Shows 4+ community skills: browser-use, claude-code, home-assistant, opencode

- **Environment Variable Configuration**: Native provider env var support
  - Support for both PEPEBOT_* prefixed and native provider variables
  - Auto-detect existing environment variables during onboarding
  - Provider API keys: `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GROQ_API_KEY`, `GEMINI_API_KEY`, `GOOGLE_API_KEY`, `OPENROUTER_API_KEY`, etc.
  - Channel tokens: `TELEGRAM_BOT_TOKEN`, `DISCORD_BOT_TOKEN`, `DISCORD_TOKEN`
  - Onboarding wizard now checks for existing env vars and asks user to confirm usage
  - Masked display of API keys for security (shows first 8 + last 4 chars)
  - Comprehensive `.env.example` with all supported variables and documentation
  - Docker Compose integration with `env_file` support
  - Helper functions: `GetProviderEnvKey()`, `GetProviderEnvBase()`, `GetChannelEnvToken()`

- **Documentation**: Comprehensive documentation system
  - `docs/workflows.md`: Complete workflow system documentation (~500 lines)
    - Tool reference for all 20+ workflow tools
    - Real-world examples (UI automation, device health, browser research)
    - Best practices, advanced patterns, and troubleshooting
  - `docs/install.md`: Complete installation guide (~400 lines)
    - Package managers (Homebrew, Nix, Docker)
    - Manual installation and build from source
    - Service setup (systemd, launchd, rc.d)
    - Platform-specific instructions and troubleshooting
  - `docs/api.md`: REST API and integration reference (~600 lines)
    - Gateway API endpoints (/health, /message, /workflow, /sessions)
    - Tool API for custom tool development
    - Provider API for LLM integrations
    - Channel API for messaging platform integrations
    - Message Bus and Session API documentation
    - Code examples in Python, JavaScript, Go, and cURL
  - `docs/README.md`: Documentation hub with navigation
  - `docs/HOMEBREW_SETUP_GUIDE.md`: Complete Homebrew tap setup guide
  - `install.sh`: Automated installer with service setup
    - Auto-detect OS and architecture (9 architectures supported)
    - Optional systemd (Linux) or launchd (macOS) setup
    - PATH configuration and verification
  - `default.nix`: Nix package definition
  - `pepebot.rb`: Homebrew formula with multi-platform support
  - `CLAUDE.md`: Project architecture guide for AI assistants

### Changed
- **Onboarding Wizard**: Enhanced interactive setup flow
  - Updated from 4 steps to 5 steps (added builtin skills installation)
  - Step 5/5: "Install Builtin Skills" with GitHub repository info
  - Default choice is Yes (user just presses Enter)
  - Skip message shows command to install later
  - Quick Start section conditionally shows install command only if skipped
  - Step 2: Now checks for existing provider environment variables
  - Step 3: Now checks for existing channel token environment variables
  - Interactive confirmation for using detected env vars with masked display
- **Skills Commands**: Streamlined skill management
  - Removed `pepebot skills list-builtin` command (redundant)
  - `install-builtin` now fetches from GitHub instead of local copy
  - Updated help messages to reflect new workflow
- **Skills Registry**: Updated data structure
  - Added `SkillsRegistry` type with version, update time, and skills array
  - Added `Path` field to `AvailableSkill` for subdirectory support
  - Added `Metadata` field for additional skill information
  - Improved JSON parsing for new registry format

### Removed
- **Local Builtin Skills Management**: Removed local skills directory lookup
  - Deleted `skillsListBuiltinCmd()` function
  - Removed local skills copying logic
  - All builtin skills now fetched from GitHub

### Technical Details
- Added `pkg/tools/adb.go`: ADB helper and 7 tool implementations (~800 lines)
- Added `pkg/tools/workflow.go`: Workflow engine and 3 tool implementations (~500 lines)
- Modified `pkg/agent/loop.go`: Tool registration in both `NewAgentLoop()` and `NewAgentLoopWithDefinition()`
- Modified `cmd/pepebot/main.go`:
  - Updated onboarding wizard with Step 5/5 for builtin skills
  - Refactored `skillsInstallBuiltinCmd()` to use GitHub ZIP download
  - Removed `skillsListBuiltinCmd()` and related case statement
  - Updated `skillsHelp()` to remove list-builtin references
  - Enhanced Step 2 (API Key) to check and use existing provider env vars
  - Enhanced Step 3 (Channels) to check and use existing channel token env vars
  - Added masked display for sensitive values (API keys, tokens)
- Modified `pkg/skills/installer.go`:
  - Added `InstallBuiltinSkills()` method with ZIP download and extraction
  - Updated registry URL to `pepebot-space/skills`
  - Added `SkillsRegistry` struct for new JSON format
  - Added `copyDir()` helper function for recursive directory copying
  - Removed git clone dependency
- Modified `pkg/config/config.go`:
  - Split `ProviderConfig` into provider-specific types (8 types)
  - Each provider config now has explicit env tags supporting multiple formats
  - Added support for both `PEPEBOT_*` and native provider env vars
  - Added `GetProviderEnvKey()` to check for existing provider API keys
  - Added `GetProviderEnvBase()` to check for existing provider API base URLs
  - Added `GetChannelEnvToken()` to check for existing channel tokens
- Updated `.env.example`: Comprehensive environment variable documentation
- Updated `docker-compose.yml`: Added `env_file` support and all provider/channel variables
- Workflow tools always registered (no dependencies)
- ADB tools conditionally registered (only if ADB binary found)
- Follows existing pepebot patterns: command execution timeouts, path resolution, error handling
- Variable interpolation supports nested step outputs and dynamic goal generation

### Use Cases
- Mobile app testing and automation
- Android device monitoring and control
- UI automation workflows with visual inspection
- Remote device management via chat interfaces
- Cross-platform automation combining ADB, shell, browser, and file tools
- LLM-driven adaptive workflows with goal-based steps
- One-command installation of official skill library
- Easy discovery and installation of community skills

## [0.3.1] - 2026-02-13

### Added
- **WhatsApp Native Support**: Direct WhatsApp Web integration via whatsmeow
  - Replaced external WebSocket bridge with native Go implementation
  - QR code login displayed directly in terminal
  - Session persistence using SQLite (no re-login required)
  - Pure Go SQLite driver (`modernc.org/sqlite`) - works on all platforms without CGO
  - WAL mode for concurrent access and better performance
  - No external dependencies or bridge setup needed

### Changed
- **WhatsApp Configuration**: Simplified config structure
  - Removed `bridge_url` field (no longer needed)
  - Added `db_path` field for session storage (default: `~/.pepebot/whatsapp.db`)
  - Onboarding wizard updated with simpler WhatsApp setup
- **WhatsApp Channel Implementation**: Complete rewrite
  - Uses `go.mau.fi/whatsmeow` for direct WhatsApp Web protocol
  - QR code rendering in terminal using `github.com/skip2/go-qrcode`
  - Automatic session restoration from SQLite on restart
  - Improved connection stability and error handling

### Fixed
- **Default Agent Registry**: Fixed "default agent not found" error
  - `InitializeFromConfig` now always ensures "default" agent exists
  - Previously only created default agent when registry was completely empty
  - Now creates default agent if missing, regardless of other agents
- **Agent Memory Not Persisting**: Fixed memory writes silently failing
  - Bootstrap loaded MEMORY.md from wrong path (workspace root instead of `memory/` subdirectory)
  - Strengthened system prompt so agent MUST call `write_file` tool for memory saves
  - Agent previously claimed "I'll remember" without actually writing to disk
- **File Tools Path Resolution**: Relative paths now resolve to workspace
  - `read_file`, `write_file`, `list_dir` tools now accept relative paths
  - Relative paths resolve to workspace directory (`~/.pepebot/workspace/`)
  - Absolute paths still work as before

### Removed
- **WebSocket Bridge Dependency**: No longer requires external WhatsApp bridge
  - Removed `gorilla/websocket` dependency (unused after migration)
  - Simplified deployment - one binary, no additional services

### Technical Details
- Modified `pkg/config/config.go`: Updated WhatsAppConfig structure
- Rewritten `pkg/channels/whatsapp.go`: Native whatsmeow implementation
- Updated `pkg/channels/manager.go`: Simplified WhatsApp initialization
- Modified `pkg/agent/registry.go`: Fixed default agent creation logic
- Updated `cmd/pepebot/main.go`: Simplified WhatsApp onboarding
- Added `pkg/channels/whatsapp_stub.go`: MIPS architecture stub (WhatsApp disabled)
- Modified `pkg/agent/context.go`: Fixed MEMORY.md path, strengthened memory instructions
- Modified `pkg/tools/filesystem.go`: Added workspace-aware path resolution
- Modified `pkg/agent/loop.go`: Pass workspace to file tools
- Added dependencies:
  - `go.mau.fi/whatsmeow` - WhatsApp Web client library
  - `modernc.org/sqlite` - Pure Go SQLite driver
  - `github.com/skip2/go-qrcode` - QR code generation for terminal

### Platform Support Notes
- **WhatsApp channel unavailable on MIPS architectures** (mips, mipsle, mips64, mips64le)
  - SQLite dependency (`modernc.org/sqlite`) does not support MIPS
  - MIPS builds will compile but WhatsApp channel will return an error if enabled
  - All other channels (Telegram, Discord, Feishu, MaixCam) work normally on MIPS

### Migration Notes
- Existing users with WhatsApp bridge setup need to re-onboard or manually update config
- Old `bridge_url` field will be ignored (can be manually removed from config.json)
- First run will show QR code - scan once to authenticate
- Session persists in `~/.pepebot/whatsapp.db` for automatic reconnection

## [0.3.0] - 2026-02-13

### Added
- **Multi-Agent System**: Agent registry with management commands
  - Register, remove, enable/disable agents via CLI (`pepebot agent list/register/remove/enable/disable/show`)
  - Per-agent model, temperature, max tokens, and prompt directory configuration
  - Agent definitions stored in workspace registry file
- **Session Key Generation**: Conversation context isolation per channel/chat
  - Each channel generates unique session keys for separate conversation histories
  - Prevents cross-chat context leakage in multi-channel setups
- **Command Handling**: Slash commands for chat channels and CLI interactive mode
  - `/new` — Clear session, start fresh conversation
  - `/stop` — Cancel in-flight LLM processing (channels only)
  - `/help` — List available commands
  - `/status` — Show current agent, model, and session info
- **Concurrent Message Processing**: Gateway now processes messages in goroutines
  - LLM calls run with cancellable contexts for `/stop` support
  - In-flight request tracking per session key via `sync.Map`
- **Verbose Logging Option**: Added `--verbose` flag to `gateway` command
  - Enables DEBUG level logging for detailed diagnostics

### Changed
- **AgentManager.Run()**: Refactored from sequential blocking to concurrent processing
  - Messages are dispatched to goroutines with per-session cancellation
  - Command messages are handled synchronously before dispatch
- **AgentLoop**: Added public accessors `Model()`, `AgentName()`, `ClearSession()`
- **SessionManager**: Added `ClearSession()` method to clear in-memory and on-disk session data

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
- **Documentation**: Updated README.md and docs/android.md
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
  - Comprehensive Android setup documentation (docs/android.md)
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
