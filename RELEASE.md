# 🐸 Pepebot v0.5.6 - Google Vertex AI Support

**Release Date:** 2026-03-02

## 🎉 What's New

### ☁️ Google Vertex AI Provider

Pepebot now supports **Google Cloud Vertex AI** natively using service account credentials (no API key needed).

- Authenticate via service account JSON file (`account_services.json`)
- Configure `credentials_file`, `project_id`, and `region`
- Full Chat and Streaming support with Gemini models
- Tool calling (function declarations) fully supported
- Supports both regional and global endpoints

**Quick Setup:**

```json
{
  "agents": {
    "defaults": {
      "model": "vertex/gemini-2.0-flash",
      "provider": "vertex"
    }
  },
  "providers": {
    "vertex": {
      "credentials_file": "~/.config/service-accounts.json",
      "project_id": "your-gcp-project-id",
      "region": "global"
    }
  }
}
```

Or via environment variables:

```bash
export PEPEBOT_PROVIDERS_VERTEX_CREDENTIALS_FILE=~/.config/service-accounts.json
export PEPEBOT_PROVIDERS_VERTEX_PROJECT_ID=your-gcp-project-id
export PEPEBOT_PROVIDERS_VERTEX_REGION=global
export PEPEBOT_AGENTS_DEFAULTS_MODEL=vertex/gemini-2.0-flash
```

### 🎯 Explicit Provider Option

New optional `provider` field in agent config — override automatic provider detection from model name prefix.

```json
{
  "agents": {
    "defaults": {
      "model": "gemini-2.0-flash",
      "provider": "vertex"
    }
  }
}
```

Supported values: `vertex`, `maiarouter`, `openrouter`, `anthropic`, `openai`, `gemini`, `zhipu`, `groq`, `vllm`

Environment variable: `PEPEBOT_AGENTS_DEFAULTS_PROVIDER`

### 🤖 Better Multi-Agent Controls (`manage_agent`)

`manage_agent` is now more complete for agent lifecycle and delegation:

- `remove` action to delete an agent from registry (optional file cleanup with `remove_files`)
- `call` action to directly invoke another named agent and get its response
- `assign_skill` action to persist skill assignments into per-agent memory (`<agent_dir>/memory/MEMORY.md`)
- `register` now supports optional `provider`, so each agent can use different provider/model settings

Prompt behavior has also been tightened so normal requests like "panggil agent" are less likely to be misinterpreted as workflow creation.

---

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
Download the appropriate binary for your platform from the [releases page](https://github.com/pepebot-space/pepebot/releases/tag/v0.5.6).

## 🚀 Quick Start

1. Create a service account at [Google Cloud Console](https://console.cloud.google.com/iam-admin/serviceaccounts)
2. Download the JSON key file
3. Configure Pepebot:
   ```bash
   pepebot onboard
   # Or manually set environment variables
   ```
4. Start chatting:
   ```bash
   pepebot agent -m "Hello from Vertex AI!"
   ```

## 📚 Documentation

- [Workflow Documentation](https://github.com/pepebot-space/pepebot/blob/main/docs/workflows.md)
- [Installation Guide](https://github.com/pepebot-space/pepebot/blob/main/docs/install.md)
- [Full Changelog](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md)

## 🔗 Links

- **GitHub**: https://github.com/pepebot-space/pepebot
- **Documentation**: https://github.com/pepebot-space/pepebot/tree/main/docs
- **Issues**: https://github.com/pepebot-space/pepebot/issues
- **Discussions**: https://github.com/pepebot-space/pepebot/discussions

## 📝 Full Changelog

For a complete list of changes, see [CHANGELOG.md](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md#056---2026-03-02).
