# 🐸 Pepebot v0.4.3 - OpenAI-Compatible Gateway API

**Release Date:** 2026-02-18

## 🎉 What's New

### 🌐 OpenAI-Compatible HTTP API with SSE Streaming

Pepebot gateway now includes a full HTTP API server that speaks the OpenAI Chat Completions protocol. Connect any OpenAI-compatible client, dashboard, or tool — it just works.

**Chat with streaming:**
```bash
curl -N -X POST http://localhost:18790/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Agent: default" \
  -d '{"model":"maia/gemini-3-pro-preview","messages":[{"role":"user","content":"Hello!"}],"stream":true}'
```

**SSE response format (OpenAI-compatible):**
```
data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"}}]}
data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"delta":{"content":"!"}}]}
data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"delta":{},"finish_reason":"stop"}]}
data: [DONE]
```

### 📡 API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/chat/completions` | Chat with agent (streaming + non-streaming) |
| `GET` | `/v1/models` | List available models |
| `GET` | `/v1/agents` | List registered agents |
| `GET` | `/v1/sessions` | List active web sessions |
| `POST` | `/v1/sessions/{key}/new` | Start new chat session |
| `POST` | `/v1/sessions/{key}/stop` | Stop in-flight processing |
| `DELETE` | `/v1/sessions/{key}` | Delete a session |
| `GET` | `/health` | Health check |

### 🔑 Key Features

- **OpenAI protocol**: Works with any OpenAI-compatible client SDK
- **SSE streaming**: Real-time token-by-token responses
- **Multi-agent**: Select agent via `X-Agent` header
- **Session management**: Create, list, clear, and stop sessions via API
- **CORS enabled**: Ready for browser dashboards
- **Tool calls internal**: Agent handles tools server-side, API returns clean content
- **No new dependencies**: Pure `net/http` stdlib

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
Download the appropriate binary for your platform from the [releases page](https://github.com/pepebot-space/pepebot/releases/tag/v0.4.3).

## 🚀 Quick Start

1. **Start the gateway:**
   ```bash
   pepebot gateway -v
   ```

2. **Check health:**
   ```bash
   curl http://localhost:18790/health
   ```

3. **Chat with the agent:**
   ```bash
   curl -X POST http://localhost:18790/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{"model":"maia/gemini-3-pro-preview","messages":[{"role":"user","content":"Hello!"}],"stream":false}'
   ```

4. **List agents:**
   ```bash
   curl http://localhost:18790/v1/agents
   ```

## 📚 Documentation

- [Installation Guide](https://github.com/pepebot-space/pepebot/blob/main/docs/install.md)
- [API Reference](https://github.com/pepebot-space/pepebot/blob/main/docs/api.md)
- [Workflow Documentation](https://github.com/pepebot-space/pepebot/blob/main/docs/workflows.md)
- [Full Changelog](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md)

## 🔗 Links

- **GitHub**: https://github.com/pepebot-space/pepebot
- **Documentation**: https://github.com/pepebot-space/pepebot/tree/main/docs
- **Issues**: https://github.com/pepebot-space/pepebot/issues
- **Discussions**: https://github.com/pepebot-space/pepebot/discussions

## 📝 Full Changelog

For a complete list of changes, see [CHANGELOG.md](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md#043---2026-02-18).

---

**Note:** When upgrading from v0.4.2, all existing configurations and data are preserved. No migration needed. The HTTP API server starts automatically alongside existing chat channels when running `pepebot gateway`.
