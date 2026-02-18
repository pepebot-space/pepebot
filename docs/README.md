# Pepebot Documentation

Welcome to the Pepebot documentation directory. Here you'll find comprehensive guides and technical documentation for all aspects of Pepebot.

## 📚 Available Documentation

### [Installation Guide](./install.md)
**Complete installation instructions for all platforms**

Learn how to install Pepebot on Linux, macOS, or FreeBSD using various methods:
- Quick automated installation with `install.sh`
- Package managers (Homebrew, Nix, Docker)
- Manual installation from pre-built binaries
- Building from source
- Setting up systemd or launchd services
- Troubleshooting common installation issues

**Topics covered:**
- Automated installer usage
- Platform-specific instructions (Linux, macOS, FreeBSD)
- Service configuration (systemd, launchd, rc.d)
- Verification and testing
- Uninstallation guide

---

### [Workflow System](./workflows.md)
**Multi-step automation framework documentation**

Complete guide to creating and using workflows for task automation:
- Workflow structure and JSON format
- Variable interpolation and data flow
- Tool steps vs goal steps
- Complete examples and advanced patterns
- Best practices and troubleshooting

**Topics covered:**
- Workflow tools (workflow_execute, workflow_save, workflow_list)
- Variable system with interpolation
- Step types and execution modes
- ADB, file, web, and shell tool reference
- Real-world workflow examples
- Performance optimization
- Debugging strategies

---

### [API Documentation](./api.md)
**REST API and integration interfaces**

Complete API reference for integrating with Pepebot:
- Gateway REST API endpoints
- Tool API for custom tools
- Provider API for LLM integrations
- Channel API for chat platforms
- Message Bus and Session API
- Code examples in Python, JavaScript, Go, and cURL

**Topics covered:**
- OpenAI-compatible Chat Completions API with SSE streaming
- Gateway endpoints (/v1/chat/completions, /v1/models, /v1/agents, /v1/sessions, /health)
- Tool development interface with examples
- Provider integration for custom LLMs
- Channel integration for messaging platforms
- Internal message bus pub/sub system
- Session management API
- Authentication and security
- Configuration via environment variables

---

### [Android Setup Guide](./android.md)
**Android-specific installation and Termux instructions**

Complete guide for running Pepebot on Android devices:
- Termux installation and setup
- Android-specific configuration
- Termux:API integration
- Mobile optimization tips
- Troubleshooting Android issues

**Topics covered:**
- Termux app installation
- Package installation (Go, ADB tools)
- Building Pepebot on Android
- Running as background service
- Device automation setup
- Performance considerations
- Common Android-specific issues

---

## 📖 Additional Resources

### Root Directory Documentation

These important documents are located in the project root:

- **[README.md](../README.md)** - Project overview, quick start, and basic usage
- **[CHANGELOG.md](../CHANGELOG.md)** - Version history and release notes
- **[BUILD.md](../BUILD.md)** - Build instructions and CI/CD documentation
- **[android.md](./android.md)** - Android-specific setup and Termux instructions
- **[CLAUDE.md](../CLAUDE.md)** - Guidelines for AI assistants working with this codebase

### Configuration

- **[config.example.json](../config.example.json)** - Example configuration file with all options

### Skills & Workflows

- **[skills/](../skills/)** - Built-in skills directory
  - Each skill has a `SKILL.md` with documentation and metadata
- **[workspace/workflows/](../workspace/workflows/)** - Example workflows
  - `examples/ui_automation.json` - Android app automation
  - `examples/device_control.json` - Device health monitoring
  - `examples/browser_automation.json` - Web research workflow

## 🔍 Quick Navigation

### Getting Started
1. [Install Pepebot](./install.md#quick-install)
2. [Run Setup Wizard](../README.md#️-configuration)
3. [Configure Channels](../README.md#channel-configuration)
4. [Start Using](../README.md#-usage)

### Advanced Usage
1. [Create Workflows](./workflows.md#workflow-structure)
2. [Install Skills](../README.md#skills-management)
3. [Setup Multi-Agent](../README.md#multi-agent-system)
4. [Android Automation](./android.md)

### Development
1. [Build from Source](./install.md#build-from-source)
2. [Architecture Overview](../CLAUDE.md#architecture)
3. [Contributing Guide](../README.md#-contributing)
4. [Release Process](../BUILD.md#release-process)

## 🆘 Getting Help

### Documentation Issues
If you find any errors or missing information in the documentation:
- [Open an issue](https://github.com/pepebot-space/pepebot/issues/new?labels=documentation)
- [Start a discussion](https://github.com/pepebot-space/pepebot/discussions)

### Installation Problems
See the [Troubleshooting section](./install.md#troubleshooting) in the installation guide.

### Workflow Issues
See the [Troubleshooting section](./workflows.md#troubleshooting) in the workflow documentation.

### General Support
- **Issues**: https://github.com/pepebot-space/pepebot/issues
- **Discussions**: https://github.com/pepebot-space/pepebot/discussions
- **Repository**: https://github.com/pepebot-space/pepebot

## 📝 Contributing to Documentation

We welcome contributions to improve our documentation!

### Documentation Standards

- Use clear, concise language
- Include practical examples
- Add troubleshooting sections for complex topics
- Keep formatting consistent with existing docs
- Test all code examples before submitting

### File Organization

- **`docs/`** - Technical documentation (this directory)
  - Feature-specific guides
  - In-depth technical documentation
  - API references
- **Root `*.md`** - User-facing documentation
  - README.md - Overview and quick start
  - CHANGELOG.md - Version history
  - BUILD.md - Build and deployment
  - android.md - Platform-specific guides

### When to Add New Documentation

Add new documentation to `docs/` when:
- Creating a new major feature that needs detailed explanation
- Writing technical guides for complex systems
- Documenting APIs, workflows, or architecture patterns
- Content is too detailed for README but important for users/developers

Always update `CHANGELOG.md` when adding or removing features.

## 🔄 Documentation Updates

This documentation is maintained alongside the codebase and is updated with each release.

**Current Version**: 0.4.3
**Last Updated**: 2026-02-18

For the latest documentation, visit: https://github.com/pepebot-space/pepebot/tree/main/docs

---

**Happy Reading! 🐸**
