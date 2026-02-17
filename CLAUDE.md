# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Pepebot is a lightweight personal AI agent written in Go. It supports multiple LLM providers (Anthropic, OpenAI, OpenRouter, Groq, Gemini, etc.), multiple chat channels (Telegram, Discord, WhatsApp, Feishu, MaixCam), and an extensible tool/skill system with Android automation via ADB.

## Build & Development Commands

```bash
make build          # Build for current platform → build/pepebot
make build-all      # Cross-compile for Linux/Windows/Android
make build-android  # Android ARM64 (CGO_ENABLED=0)
make install        # Build + install binary and skills to ~/.local/bin
make fmt            # Format Go code (go fmt ./...)
make deps           # Update and tidy dependencies
make run ARGS="agent -m 'hello'"  # Build and run with arguments
make clean          # Remove build artifacts
```

## Testing

```bash
go test ./...                       # Run all tests
go test -v -race ./pkg/logger/...   # Run single package tests with race detection
```

Test coverage is currently limited (primarily `pkg/logger/`). CI runs: `go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...`

## Architecture

### Core Flow
User message → Channel (Telegram/Discord/etc.) → Message Bus → Agent Loop → LLM + Tool Calls → Response back through channel

### Key Packages (`pkg/`)

- **agent/** - Central agent loop (`loop.go`), context builder (`context.go`), multi-agent registry. The loop iterates LLM calls + tool execution up to `max_tool_iterations` (default 20). Context is built from workspace files (SOUL.md, USER.md, AGENTS.md) plus session history with automatic summarization.
- **tools/** - Registry-based tool system. Each tool implements `Tool` interface (Name, Description, Parameters, Execute). Tools are registered at startup; ADB tools register conditionally based on binary availability.
  - File tools: read_file, write_file, list_dir, edit_file
  - Shell: exec (with timeout)
  - Web: web_search (Brave API), web_fetch
  - ADB: adb_devices, adb_shell, adb_tap, adb_input_text, adb_screenshot, adb_ui_dump, adb_swipe
  - Workflow: workflow_execute, workflow_save, workflow_list
- **channels/** - Chat platform integrations, each implementing channel interface
- **providers/** - LLM provider implementations behind a common interface
- **session/** - In-memory + disk-persisted sessions with token-aware summarization (triggers at 75% context window, keeps summary + last 4 messages)
- **bus/** - Message bus for loose coupling between channels and agent
- **skills/** - Dynamic skill loader; skills are markdown files (SKILL.md) with frontmatter metadata, loaded from `~/.pepebot/workspace/skills/`
- **config/** - JSON config (`~/.pepebot/config.json`) with environment variable overrides (`PEPEBOT_*` pattern)

### Workflow System (`pkg/tools/workflow.go`)
JSON-based multi-step automation with variable interpolation (`{{variable}}`). Two step types: **tool** (execute a registered tool) and **goal** (natural language for LLM). Step outputs auto-available as `{{step_name_output}}`. Workflows stored in `~/.pepebot/workspace/workflows/`.

### Entry Point
`cmd/pepebot/main.go` - CLI with subcommands: `onboard`, `agent`, `gateway`, `status`, `skills`, `cron`

## Configuration

- Config file: `~/.pepebot/config.json`
- Workspace: `~/.pepebot/workspace/` (skills, workflows, context files)
- Environment overrides: `PEPEBOT_AGENTS_DEFAULTS_MODEL`, `PEPEBOT_CHANNELS_TELEGRAM_TOKEN`, etc.

## Module

`github.com/pepebot-space/pepebot` — Go 1.24.0, pure Go with no CGO requirements (uses `modernc.org/sqlite`).

## Documentation

Project documentation is organized in the following locations:

- **`docs/`** - Technical documentation for features and systems
  - `docs/README.md` - Documentation index with navigation guide
  - `docs/install.md` - Complete installation guide for all platforms
  - `docs/workflows.md` - Complete workflow system documentation
  - Add new documentation files here for major features
- **`CHANGELOG.md`** - Version history and release notes
  - **IMPORTANT**: Update CHANGELOG.md when adding or removing features
  - Follow format: `## [Version] - Date` with sections: Added, Changed, Removed, Fixed
  - Include technical details and use cases for significant changes
- **`README.md`** - User-facing quick start and overview
- **`BUILD.md`** - Build instructions and CI/CD documentation

### When to Update Documentation

**Always update CHANGELOG.md when:**
- Adding new features (tools, commands, integrations)
- Removing features or deprecating functionality
- Making breaking changes to APIs or configuration
- Fixing significant bugs
- Changing system behavior

**Add to `docs/` when:**
- Creating new major features that need detailed explanation
- Writing technical guides for complex systems
- Documenting APIs, workflows, or architecture patterns
- Content is too detailed for README but important for developers
