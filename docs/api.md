# Pepebot API Documentation

Complete API reference for Pepebot Gateway and integrations.

## Table of Contents

- [Overview](#overview)
- [Gateway API](#gateway-api)
- [Message Bus API](#message-bus-api)
- [Tool API](#tool-api)
- [Provider API](#provider-api)
- [Channel API](#channel-api)
- [Session API](#session-api)
- [Configuration](#configuration)
- [Examples](#examples)

---

## Overview

Pepebot provides multiple APIs for integration and extension:

- **Gateway API** - HTTP REST API for external integrations
- **Message Bus** - Internal pub/sub for components
- **Tool API** - Interface for creating custom tools
- **Provider API** - Interface for adding LLM providers
- **Channel API** - Interface for chat platform integrations

### Architecture

```
┌─────────────────────────────────────────┐
│           External Clients              │
└──────────────┬──────────────────────────┘
               │ HTTP/WebSocket
               ▼
┌─────────────────────────────────────────┐
│         Gateway API (18790)             │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│          Message Bus                     │
├──────────┬──────────┬───────────────────┤
│ Channels │  Agent   │   Tools/Providers │
└──────────┴──────────┴───────────────────┘
```

---

## Gateway API

HTTP REST API for sending messages and managing the bot.

### Base URL

```
http://localhost:18790
```

Configure in `~/.pepebot/config.json`:
```json
{
  "gateway": {
    "host": "0.0.0.0",
    "port": 18790
  }
}
```

### Endpoints Overview

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/chat/completions` | Chat with agent (OpenAI-compatible, SSE streaming) |
| `GET` | `/v1/models` | List available models |
| `GET` | `/v1/agents` | List registered agents |
| `GET` | `/v1/sessions` | List active web sessions |
| `GET` | `/v1/sessions/{key}` | Get session history |
| `POST` | `/v1/sessions/{key}/new` | Clear & start new session |
| `POST` | `/v1/sessions/{key}/stop` | Stop in-flight processing |
| `DELETE` | `/v1/sessions/{key}` | Delete a session |
| `GET` | `/v1/skills` | List installed skills |
| `GET` | `/v1/workflows` | List available workflows |
| `GET` | `/v1/workflows/{name}` | Get workflow definition |
| `GET` | `/v1/config` | Get configuration (masked keys) |
| `PUT` | `/v1/config` | Update configuration |
| `GET` | `/health` | Health check |

---

#### Health Check

**GET** `/health`

Check if the gateway is running.

**Response:**
```json
{
  "status": "ok"
}
```

**Example:**
```bash
curl http://localhost:18790/health
```

---

#### Chat Completions (OpenAI-Compatible)

**POST** `/v1/chat/completions`

Send messages and get a response from the agent. Supports both streaming (SSE) and non-streaming modes. Compatible with OpenAI client SDKs.

**Request Headers:**
```
Content-Type: application/json
X-Agent: default          (optional, selects agent, default: "default")
X-Session-Key: web:default (optional, session routing, default: "web:<agent>")
```

**Request Body:**
```json
{
  "model": "maia/gemini-3-pro-preview",
  "messages": [
    {"role": "user", "content": "Hello!"}
  ],
  "stream": true,
  "temperature": 0.7,
  "max_tokens": 8192
}
```

**Non-Streaming Response** (`stream: false`):
```json
{
  "id": "chatcmpl-1708278123456789",
  "object": "chat.completion",
  "created": 1708278123,
  "model": "maia/gemini-3-pro-preview",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ]
}
```

**Streaming Response** (`stream: true`):
```
data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant"}}]}

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"}}]}

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"!"}}]}

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

**Error Response:**
```json
{
  "error": {
    "message": "invalid request body: ...",
    "type": "invalid_request_error",
    "code": "Bad Request"
  }
}
```

**Example (non-streaming):**
```bash
curl -X POST http://localhost:18790/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "maia/gemini-3-pro-preview",
    "messages": [{"role": "user", "content": "What is 2+2?"}],
    "stream": false
  }'
```

**Example (streaming):**
```bash
curl -N -X POST http://localhost:18790/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Agent: default" \
  -d '{
    "model": "maia/gemini-3-pro-preview",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

**Example (specific agent):**
```bash
curl -X POST http://localhost:18790/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Agent: coder" \
  -H "X-Session-Key: web:coder" \
  -d '{
    "model": "maia/claude-3-5-sonnet",
    "messages": [{"role": "user", "content": "Write a Python hello world"}],
    "stream": true
  }'
```

> **Note:** Tool calls are handled server-side by the agent loop. The API only returns the final assistant content — tool execution is invisible to the client.

---

#### List Models

**GET** `/v1/models`

List available models from enabled agents.

**Response:**
```json
{
  "object": "list",
  "data": [
    {
      "id": "maia/gemini-3-pro-preview",
      "object": "model",
      "created": 1708278123,
      "owned_by": "pepebot"
    },
    {
      "id": "maia/claude-3-5-sonnet",
      "object": "model",
      "created": 1708278123,
      "owned_by": "pepebot"
    }
  ]
}
```

**Example:**
```bash
curl http://localhost:18790/v1/models
```

---

#### List Agents

**GET** `/v1/agents`

Returns raw `registry.json` content from `~/.pepebot/workspace/agents/registry.json`.

**Response:**
```json
{
  "version": "1.0",
  "agents": {
    "default": {
      "enabled": true,
      "model": "maia/gemini-3-pro-preview",
      "provider": "",
      "description": "Default general-purpose agent",
      "temperature": 0.7,
      "max_tokens": 8192
    },
    "coder": {
      "enabled": true,
      "model": "maia/claude-3-5-sonnet",
      "description": "Coding specialist"
    }
  }
}
```

**Example:**
```bash
curl http://localhost:18790/v1/agents
```

---

#### List Sessions

**GET** `/v1/sessions`

List active web sessions (filtered by `web:` prefix).

**Response:**
```json
{
  "sessions": [
    {
      "key": "web:default",
      "created": "2026-02-18T12:00:00Z",
      "updated": "2026-02-18T12:05:00Z",
      "message_count": 5
    },
    {
      "key": "web:coder",
      "created": "2026-02-18T11:00:00Z",
      "updated": "2026-02-18T11:30:00Z",
      "message_count": 12
    }
  ]
}
```

**Example:**
```bash
curl http://localhost:18790/v1/sessions
```

---

#### New Session

**POST** `/v1/sessions/{key}/new`

Clear and create a new chat session.

**Response:**
```json
{
  "status": "ok",
  "session_key": "web:default",
  "message": "Session cleared. Starting fresh conversation."
}
```

**Example:**
```bash
curl -X POST http://localhost:18790/v1/sessions/web:default/new
```

---

#### Stop Session Processing

**POST** `/v1/sessions/{key}/stop`

Stop in-flight LLM processing for a session.

**Response:**
```json
{
  "status": "ok",
  "message": "Stopping current processing..."
}
```

**Example:**
```bash
curl -X POST http://localhost:18790/v1/sessions/web:default/stop
```

---

#### Delete Session

**DELETE** `/v1/sessions/{key}`

Delete a specific session and its history.

**Response:**
```json
{
  "status": "ok",
  "session_key": "web:default",
  "message": "Session deleted."
}
```

**Example:**
```bash
curl -X DELETE http://localhost:18790/v1/sessions/web:default
```

---

#### List Skills

**GET** `/v1/skills`

List all installed skills from workspace and builtin directories.

**Response:**
```json
{
  "skills": [
    {
      "name": "weather",
      "source": "workspace",
      "description": "Get weather information for any location",
      "available": true
    },
    {
      "name": "github",
      "source": "workspace",
      "description": "GitHub integration and management",
      "available": true
    },
    {
      "name": "tmux",
      "source": "builtin",
      "description": "Terminal multiplexer management",
      "available": false,
      "missing": "CLI: tmux"
    }
  ]
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Skill directory name |
| `source` | string | `"workspace"` or `"builtin"` |
| `description` | string | From SKILL.md frontmatter |
| `available` | boolean | Whether requirements are met |
| `missing` | string | Missing requirements (if unavailable) |

**Example:**
```bash
curl http://localhost:18790/v1/skills
```

---

#### List Workflows

**GET** `/v1/workflows`

List all available workflow definitions from `~/.pepebot/workspace/workflows/`.

**Response:**
```json
{
  "workflows": [
    {
      "name": "deploy-app",
      "description": "Build and deploy application to production",
      "step_count": 5,
      "variables": {
        "app_name": "myapp",
        "env": "production"
      }
    }
  ]
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Workflow name (from JSON or filename) |
| `description` | string | Workflow description |
| `step_count` | int | Number of steps in the workflow |
| `variables` | object | Default variable key-value pairs |

**Example:**
```bash
curl http://localhost:18790/v1/workflows
```

---

#### Get Workflow Definition

**GET** `/v1/workflows/{name}`

Get the full workflow definition including all steps and variables.

**Response:**
```json
{
  "name": "deploy-app",
  "description": "Build and deploy application to production",
  "variables": {
    "app_name": "myapp",
    "env": "production"
  },
  "steps": [
    {
      "name": "Build",
      "tool": "shell",
      "args": {
        "command": "npm run build"
      }
    },
    {
      "name": "Deploy",
      "goal": "Deploy {{app_name}} to {{env}}"
    }
  ]
}
```

**Error (404):**
```json
{
  "error": {
    "message": "workflow not found",
    "type": "not_found",
    "code": "Not Found"
  }
}
```

**Example:**
```bash
curl http://localhost:18790/v1/workflows/deploy-app
```

---

#### Get Configuration

**GET** `/v1/config`

Returns the current `~/.pepebot/config.json` with sensitive fields (API keys, tokens, secrets) masked.

**Response:**
```json
{
  "agents": {
    "defaults": {
      "workspace": "~/.pepebot/workspace",
      "model": "maia/gemini-3-pro-preview",
      "max_tokens": 8192,
      "temperature": 0.7,
      "max_tool_iterations": 20
    }
  },
  "gateway": {
    "host": "0.0.0.0",
    "port": 18790
  },
  "providers": {
    "maiarouter": {
      "api_key": "sk-i****-SOw",
      "api_base": ""
    }
  },
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "8504****4Dqw",
      "allow_from": []
    }
  },
  "tools": {
    "web": {
      "search": {
        "api_key": "",
        "max_results": 5
      }
    }
  }
}
```

> **Note:** Fields matching `api_key`, `token`, or `secret` are automatically masked as `xxxx****xxxx` to prevent accidental exposure.

**Example:**
```bash
curl http://localhost:18790/v1/config
```

---

#### Update Configuration

**PUT** `/v1/config`

Save a new configuration to `~/.pepebot/config.json`. Masked values (containing `****`) are automatically restored from the current config, so unchanged secrets are preserved.

**Request Body:** Full config JSON object (same structure as GET response).

**Response:**
```json
{
  "status": "ok",
  "message": "Configuration saved. Restart gateway to apply changes."
}
```

**Error Response:**
```json
{
  "error": {
    "message": "invalid JSON: ...",
    "type": "invalid_request_error",
    "code": "Bad Request"
  }
}
```

**Example:**
```bash
curl -X PUT http://localhost:18790/v1/config \
  -H "Content-Type: application/json" \
  -d '{
    "agents": {"defaults": {"model": "maia/claude-4-sonnet", "temperature": 0.5}},
    "gateway": {"host": "0.0.0.0", "port": 18790}
  }'
```

> **Important:** Changes take effect after restarting the gateway.

---

### Authentication

Currently, the Gateway API does not require authentication. For production use, consider:

1. **Reverse Proxy**: Use nginx/caddy with auth
2. **Network Isolation**: Bind to localhost only
3. **Firewall**: Restrict access by IP
4. **API Key**: Implement custom auth middleware

**Example nginx config:**
```nginx
location /pepebot/ {
    auth_basic "Pepebot API";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://localhost:18790/;
}
```

---

## Message Bus API

Internal pub/sub system for component communication.

### Interface

```go
type MessageBus interface {
    // Subscribe to messages
    Subscribe(channelName string) <-chan InboundMessage

    // Publish a message
    Publish(msg InboundMessage)

    // Close the bus
    Close()
}

type InboundMessage struct {
    Channel    string
    ChatID     string
    UserID     string
    Username   string
    Content    string
    Media      []string
    SessionKey string
}

type OutboundMessage struct {
    Channel string
    ChatID  string
    Content string
    Media   []string
}
```

### Usage

```go
package main

import (
    "github.com/pepebot-space/pepebot/pkg/bus"
)

func main() {
    // Create bus
    msgBus := bus.NewMessageBus()
    defer msgBus.Close()

    // Subscribe
    inbound := msgBus.Subscribe("my-channel")

    // Publish
    msgBus.Publish(bus.InboundMessage{
        Channel:    "my-channel",
        ChatID:     "user123",
        Content:    "Hello!",
        SessionKey: "my-channel:user123",
    })

    // Receive
    msg := <-inbound
    // Process message
}
```

---

## Tool API

Interface for creating custom tools that the agent can use.

### Tool Interface

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() interface{}
    Execute(args map[string]interface{}) (string, error)
}
```

### Creating a Custom Tool

**Example: Custom Calculator Tool**

```go
package tools

import (
    "fmt"
    "github.com/pepebot-space/pepebot/pkg/tools"
)

type CalculatorTool struct{}

func (t *CalculatorTool) Name() string {
    return "calculator"
}

func (t *CalculatorTool) Description() string {
    return "Perform basic math calculations"
}

func (t *CalculatorTool) Parameters() interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "expression": map[string]interface{}{
                "type":        "string",
                "description": "Math expression to evaluate (e.g., '2 + 2')",
            },
        },
        "required": []string{"expression"},
    }
}

func (t *CalculatorTool) Execute(args map[string]interface{}) (string, error) {
    expr, ok := args["expression"].(string)
    if !ok {
        return "", fmt.Errorf("expression must be a string")
    }

    // Simple evaluation (use a real parser in production)
    result, err := evaluateExpression(expr)
    if err != nil {
        return "", fmt.Errorf("failed to evaluate: %w", err)
    }

    return fmt.Sprintf("Result: %v", result), nil
}

func evaluateExpression(expr string) (float64, error) {
    // Implementation here
    return 0, nil
}
```

### Registering Custom Tools

**In `pkg/agent/loop.go`:**

```go
func NewAgentLoop(cfg *config.Config, provider providers.Provider) *AgentLoop {
    // ... existing code ...

    // Register custom tool
    registry.Register(&tools.CalculatorTool{})

    // ... rest of initialization ...
}
```

### Built-in Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `read_file` | Read file contents | `path` |
| `write_file` | Write to file | `path`, `content` |
| `list_dir` | List directory | `path` |
| `edit_file` | Edit file | `path`, `old_content`, `new_content` |
| `exec` | Execute shell command | `command` |
| `web_search` | Search the web | `query` |
| `web_fetch` | Fetch URL content | `url` |
| `adb_devices` | List Android devices | - |
| `adb_shell` | Execute ADB shell | `command`, `device` |
| `adb_tap` | Tap screen | `x`, `y`, `device` |
| `adb_input_text` | Input text | `text`, `device` |
| `adb_screenshot` | Capture screenshot | `filename`, `device` |
| `adb_ui_dump` | Get UI hierarchy | `device` |
| `adb_swipe` | Swipe gesture | `x1`, `y1`, `x2`, `y2`, `device` |
| `workflow_execute` | Execute workflow | `workflow_name`, `variables` |
| `workflow_save` | Save workflow | `workflow_name`, `workflow_content` |
| `workflow_list` | List workflows | - |

---

## Provider API

Interface for integrating LLM providers.

### Provider Interface

```go
type Provider interface {
    // Generate a response
    GenerateResponse(ctx context.Context, messages []Message, tools []Tool) (Response, error)

    // Get provider name
    Name() string

    // Validate configuration
    ValidateConfig() error
}

type Message struct {
    Role    string      // "system", "user", "assistant"
    Content interface{} // string or []ContentBlock
}

type ContentBlock struct {
    Type     string   // "text", "image"
    Text     string   // For text blocks
    ImageURL ImageURL // For image blocks
}

type Response struct {
    Content   string
    ToolCalls []ToolCall
    StopReason string
}

type ToolCall struct {
    ID         string
    Name       string
    Parameters map[string]interface{}
}
```

### Creating a Custom Provider

**Example: Custom OpenAI-compatible Provider**

```go
package providers

import (
    "context"
    "github.com/pepebot-space/pepebot/pkg/providers"
)

type CustomProvider struct {
    apiKey  string
    baseURL string
    model   string
}

func NewCustomProvider(apiKey, baseURL, model string) *CustomProvider {
    return &CustomProvider{
        apiKey:  apiKey,
        baseURL: baseURL,
        model:   model,
    }
}

func (p *CustomProvider) Name() string {
    return "custom"
}

func (p *CustomProvider) ValidateConfig() error {
    if p.apiKey == "" {
        return fmt.Errorf("API key is required")
    }
    return nil
}

func (p *CustomProvider) GenerateResponse(
    ctx context.Context,
    messages []providers.Message,
    tools []providers.Tool,
) (providers.Response, error) {
    // Implementation:
    // 1. Convert messages to provider format
    // 2. Make API call
    // 3. Parse response
    // 4. Return Response struct

    return providers.Response{
        Content: "Response from custom provider",
    }, nil
}
```

### Built-in Providers

| Provider | Models | Features |
|----------|--------|----------|
| **MAIA Router** | 200+ models | Multi-provider, Indonesian-friendly |
| **Anthropic** | Claude 3.5, 3 Opus/Sonnet | Vision, tools, long context |
| **OpenAI** | GPT-4, GPT-3.5 | Vision, tools, function calling |
| **OpenRouter** | 100+ models | Unified API for many providers |
| **Groq** | Llama 3, Mixtral | Fast inference, open models |
| **Gemini** | Gemini 2.0, 1.5 | Vision, tools, long context |
| **Zhipu** | GLM-4 | Chinese language optimized |
| **vLLM** | Local models | Self-hosted, any model |

---

## Channel API

Interface for integrating chat platforms.

### Channel Interface

```go
type Channel interface {
    // Start the channel
    Start(ctx context.Context) error

    // Stop the channel
    Stop(ctx context.Context) error

    // Get channel name
    Name() string

    // Send message
    Send(msg OutboundMessage) error
}
```

### Creating a Custom Channel

**Example: Custom HTTP Webhook Channel**

```go
package channels

import (
    "context"
    "encoding/json"
    "net/http"
    "github.com/pepebot-space/pepebot/pkg/bus"
)

type WebhookChannel struct {
    port int
    bus  *bus.MessageBus
}

func NewWebhookChannel(port int, msgBus *bus.MessageBus) *WebhookChannel {
    return &WebhookChannel{
        port: port,
        bus:  msgBus,
    }
}

func (c *WebhookChannel) Name() string {
    return "webhook"
}

func (c *WebhookChannel) Start(ctx context.Context) error {
    http.HandleFunc("/webhook", c.handleWebhook)

    go func() {
        http.ListenAndServe(fmt.Sprintf(":%d", c.port), nil)
    }()

    return nil
}

func (c *WebhookChannel) Stop(ctx context.Context) error {
    return nil
}

func (c *WebhookChannel) Send(msg bus.OutboundMessage) error {
    // Send response back via webhook or store for polling
    return nil
}

func (c *WebhookChannel) handleWebhook(w http.ResponseWriter, r *http.Request) {
    var payload struct {
        UserID  string `json:"user_id"`
        Message string `json:"message"`
    }

    json.NewDecoder(r.Body).Decode(&payload)

    // Publish to bus
    c.bus.Publish(bus.InboundMessage{
        Channel:    "webhook",
        UserID:     payload.UserID,
        Content:    payload.Message,
        SessionKey: fmt.Sprintf("webhook:%s", payload.UserID),
    })

    w.WriteHeader(http.StatusOK)
}
```

### Built-in Channels

| Channel | Protocol | Features |
|---------|----------|----------|
| **Telegram** | Bot API | Text, images, voice, buttons |
| **Discord** | WebSocket | Text, images, embeds, threads |
| **WhatsApp** | Web Protocol | Text, images, QR login |
| **Feishu** | Webhook | Text, cards, interactive |
| **MaixCam** | Custom | Device-specific integration |

---

## Session API

Manage conversation sessions and history.

### Session Manager Interface

```go
type SessionManager interface {
    // Get session history
    GetHistory(sessionKey string) []Message

    // Add message to session
    AddMessage(sessionKey string, msg Message)

    // Clear session
    ClearSession(sessionKey string)

    // Get all sessions
    ListSessions() []SessionInfo
}

type SessionInfo struct {
    Key          string
    MessageCount int
    LastActivity time.Time
}
```

### Usage Example

```go
package main

import (
    "github.com/pepebot-space/pepebot/pkg/session"
)

func main() {
    sessionMgr := session.NewSessionManager("~/.pepebot/sessions")

    // Get history
    history := sessionMgr.GetHistory("telegram:123456")

    // Add message
    sessionMgr.AddMessage("telegram:123456", session.Message{
        Role:    "user",
        Content: "Hello!",
    })

    // Clear session
    sessionMgr.ClearSession("telegram:123456")

    // List all sessions
    sessions := sessionMgr.ListSessions()
}
```

### Session Storage

Sessions are stored in: `~/.pepebot/sessions/`

Format: JSON files per session key

**Example: `telegram-123456.json`**
```json
{
  "key": "telegram:123456",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Hello!"
    },
    {
      "role": "assistant",
      "content": "Hi! How can I help you?"
    }
  ],
  "created_at": "2026-02-17T10:00:00Z",
  "updated_at": "2026-02-17T12:00:00Z"
}
```

---

## Configuration

### Configuration File

Location: `~/.pepebot/config.json`

### Environment Variables

All config values can be overridden with environment variables using the pattern:
```
PEPEBOT_<SECTION>_<KEY>=value
```

**Examples:**
```bash
# Agent configuration
export PEPEBOT_AGENTS_DEFAULTS_MODEL="maia/gemini-3-pro-preview"
export PEPEBOT_AGENTS_DEFAULTS_MAX_TOKENS=8192

# Provider API keys
export PEPEBOT_PROVIDERS_ANTHROPIC_APIKEY="sk-ant-xxx"
export PEPEBOT_PROVIDERS_OPENAI_APIKEY="sk-xxx"

# Channel configuration
export PEPEBOT_CHANNELS_TELEGRAM_ENABLED=true
export PEPEBOT_CHANNELS_TELEGRAM_TOKEN="123456:ABC-DEF"

# Gateway configuration
export PEPEBOT_GATEWAY_HOST="0.0.0.0"
export PEPEBOT_GATEWAY_PORT=18790
```

---

## Examples

### Python Client (OpenAI SDK)

```python
from openai import OpenAI

# Use Pepebot as an OpenAI-compatible API
client = OpenAI(
    base_url="http://localhost:18790/v1",
    api_key="not-needed",  # Pepebot uses its own configured API keys
)

# Non-streaming
response = client.chat.completions.create(
    model="maia/gemini-3-pro-preview",
    messages=[{"role": "user", "content": "What is 2+2?"}],
)
print(response.choices[0].message.content)

# Streaming
stream = client.chat.completions.create(
    model="maia/gemini-3-pro-preview",
    messages=[{"role": "user", "content": "Tell me a story"}],
    stream=True,
    extra_headers={"X-Agent": "default"},
)
for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="", flush=True)
print()
```

### JavaScript Client (SSE Streaming)

```javascript
// Non-streaming
async function chat(message) {
    const response = await fetch('http://localhost:18790/v1/chat/completions', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'X-Agent': 'default',
        },
        body: JSON.stringify({
            model: 'maia/gemini-3-pro-preview',
            messages: [{ role: 'user', content: message }],
            stream: false,
        }),
    });
    const data = await response.json();
    return data.choices[0].message.content;
}

// Streaming with SSE
async function chatStream(message, onChunk) {
    const response = await fetch('http://localhost:18790/v1/chat/completions', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'X-Agent': 'default',
        },
        body: JSON.stringify({
            model: 'maia/gemini-3-pro-preview',
            messages: [{ role: 'user', content: message }],
            stream: true,
        }),
    });

    const reader = response.body.getReader();
    const decoder = new TextDecoder();

    while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        const lines = decoder.decode(value).split('\n');
        for (const line of lines) {
            if (line.startsWith('data: ') && line !== 'data: [DONE]') {
                const chunk = JSON.parse(line.slice(6));
                const content = chunk.choices[0]?.delta?.content;
                if (content) onChunk(content);
            }
        }
    }
}

// Usage
const reply = await chat('Hello!');
console.log(reply);

await chatStream('Tell me a story', (text) => process.stdout.write(text));
```

### cURL Examples

**Chat (non-streaming):**
```bash
curl -X POST http://localhost:18790/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"maia/gemini-3-pro-preview","messages":[{"role":"user","content":"Hello!"}],"stream":false}'
```

**Chat (streaming):**
```bash
curl -N -X POST http://localhost:18790/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-Agent: default" \
  -d '{"model":"maia/gemini-3-pro-preview","messages":[{"role":"user","content":"Hello!"}],"stream":true}'
```

**List models:**
```bash
curl http://localhost:18790/v1/models | jq
```

**List agents:**
```bash
curl http://localhost:18790/v1/agents | jq
```

**List sessions:**
```bash
curl http://localhost:18790/v1/sessions | jq
```

**New session:**
```bash
curl -X POST http://localhost:18790/v1/sessions/web:default/new
```

**Stop processing:**
```bash
curl -X POST http://localhost:18790/v1/sessions/web:default/stop
```

**Delete session:**
```bash
curl -X DELETE http://localhost:18790/v1/sessions/web:default
```

### Go Client

```go
package main

import (
    "bufio"
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
)

func main() {
    payload := map[string]interface{}{
        "model":    "maia/gemini-3-pro-preview",
        "messages": []map[string]string{{"role": "user", "content": "Hello!"}},
        "stream":   true,
    }
    data, _ := json.Marshal(payload)

    req, _ := http.NewRequest("POST", "http://localhost:18790/v1/chat/completions", bytes.NewReader(data))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Agent", "default")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        line := scanner.Text()
        if strings.HasPrefix(line, "data: ") && line != "data: [DONE]" {
            var chunk map[string]interface{}
            json.Unmarshal([]byte(line[6:]), &chunk)
            choices := chunk["choices"].([]interface{})
            if len(choices) > 0 {
                delta := choices[0].(map[string]interface{})["delta"].(map[string]interface{})
                if content, ok := delta["content"].(string); ok {
                    fmt.Print(content)
                }
            }
        }
    }
    fmt.Println()
}
```

---

## Rate Limiting

Gateway does not implement rate limiting by default. For production:

1. **Use reverse proxy** (nginx, caddy) with rate limiting
2. **Implement middleware** in gateway
3. **Use API gateway** (Kong, Tyk)

**Example nginx rate limiting:**
```nginx
limit_req_zone $binary_remote_addr zone=pepebot:10m rate=10r/m;

location /pepebot/ {
    limit_req zone=pepebot burst=5;
    proxy_pass http://localhost:18790/;
}
```

---

## Error Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 400 | Bad Request (invalid JSON, missing required fields) |
| 404 | Not Found (invalid endpoint, session not found) |
| 405 | Method Not Allowed (wrong HTTP method) |
| 500 | Internal Server Error (agent failure, provider error) |

---

## Security Considerations

1. **Network Binding**: Bind to localhost in production
2. **Authentication**: Implement API key or OAuth
3. **Encryption**: Use TLS/HTTPS with reverse proxy
4. **Input Validation**: Sanitize all inputs
5. **Rate Limiting**: Prevent abuse
6. **Secrets**: Never expose API keys in responses

---

## Additional Resources

- [Workflow API](./workflows.md#tool-reference)
- [Configuration Guide](../README.md#configuration)
- [Examples Repository](https://github.com/pepebot-space/examples)

---

**API Version:** 0.4.3
**Last Updated:** 2026-02-18
**Status:** Stable
