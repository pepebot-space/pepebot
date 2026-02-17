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

### Endpoints

#### Health Check

**GET** `/health`

Check if the gateway is running.

**Response:**
```json
{
  "status": "ok",
  "version": "0.4.0",
  "uptime": 3600
}
```

**Example:**
```bash
curl http://localhost:18790/health
```

---

#### Send Message

**POST** `/message`

Send a message to the agent and get a response.

**Request Headers:**
```
Content-Type: application/json
```

**Request Body:**
```json
{
  "message": "string (required)",
  "session_key": "string (optional, default: 'gateway:default')",
  "agent": "string (optional, default: 'default')",
  "channel": "string (optional, default: 'gateway')",
  "media": ["string"] (optional, image URLs or file paths)
}
```

**Response:**
```json
{
  "success": true,
  "response": "Agent's response text",
  "session_key": "gateway:default",
  "timestamp": "2026-02-17T12:00:00Z"
}
```

**Error Response:**
```json
{
  "success": false,
  "error": "Error message"
}
```

**Example:**
```bash
curl -X POST http://localhost:18790/message \
  -H "Content-Type: application/json" \
  -d '{
    "message": "What is the weather today?",
    "session_key": "api:user123"
  }'
```

**Example with Media:**
```bash
curl -X POST http://localhost:18790/message \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Analyze this image",
    "media": ["https://example.com/image.jpg"],
    "session_key": "api:user123"
  }'
```

---

#### Execute Workflow

**POST** `/workflow`

Execute a workflow by name.

**Request Body:**
```json
{
  "workflow_name": "string (required)",
  "variables": {
    "key": "value"
  } (optional)
}
```

**Response:**
```json
{
  "success": true,
  "workflow_name": "device_health",
  "result": "Workflow execution result",
  "timestamp": "2026-02-17T12:00:00Z"
}
```

**Example:**
```bash
curl -X POST http://localhost:18790/workflow \
  -H "Content-Type: application/json" \
  -d '{
    "workflow_name": "device_health",
    "variables": {
      "device": "emulator-5554"
    }
  }'
```

---

#### List Sessions

**GET** `/sessions`

List all active sessions.

**Response:**
```json
{
  "success": true,
  "sessions": [
    {
      "key": "gateway:default",
      "message_count": 15,
      "last_activity": "2026-02-17T12:00:00Z"
    },
    {
      "key": "telegram:123456789",
      "message_count": 42,
      "last_activity": "2026-02-17T11:55:00Z"
    }
  ]
}
```

**Example:**
```bash
curl http://localhost:18790/sessions
```

---

#### Clear Session

**DELETE** `/session/:session_key`

Clear a specific session's history.

**Response:**
```json
{
  "success": true,
  "session_key": "gateway:default",
  "message": "Session cleared"
}
```

**Example:**
```bash
curl -X DELETE http://localhost:18790/session/gateway:default
```

---

#### Agent Status

**GET** `/status`

Get agent status and configuration.

**Response:**
```json
{
  "success": true,
  "agent": "default",
  "model": "maia/gemini-3-pro-preview",
  "channels": {
    "telegram": true,
    "discord": false,
    "whatsapp": true
  },
  "tools": [
    "read_file",
    "write_file",
    "web_search",
    "adb_devices"
  ],
  "skills": [
    "weather",
    "github",
    "workflow"
  ]
}
```

**Example:**
```bash
curl http://localhost:18790/status
```

---

#### List Tools

**GET** `/tools`

Get list of available tools.

**Response:**
```json
{
  "success": true,
  "tools": [
    {
      "name": "read_file",
      "description": "Read contents of a file",
      "parameters": {
        "type": "object",
        "properties": {
          "path": {
            "type": "string",
            "description": "File path to read"
          }
        },
        "required": ["path"]
      }
    }
  ]
}
```

**Example:**
```bash
curl http://localhost:18790/tools
```

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

### Python Client

```python
import requests

class PepebotClient:
    def __init__(self, base_url="http://localhost:18790"):
        self.base_url = base_url

    def send_message(self, message, session_key="api:python"):
        response = requests.post(
            f"{self.base_url}/message",
            json={
                "message": message,
                "session_key": session_key
            }
        )
        return response.json()

    def execute_workflow(self, workflow_name, variables=None):
        response = requests.post(
            f"{self.base_url}/workflow",
            json={
                "workflow_name": workflow_name,
                "variables": variables or {}
            }
        )
        return response.json()

    def get_status(self):
        response = requests.get(f"{self.base_url}/status")
        return response.json()

# Usage
client = PepebotClient()

# Send message
result = client.send_message("What is 2+2?")
print(result["response"])

# Execute workflow
workflow_result = client.execute_workflow(
    "device_health",
    {"device": "emulator-5554"}
)
print(workflow_result)
```

### JavaScript Client

```javascript
class PepebotClient {
    constructor(baseUrl = 'http://localhost:18790') {
        this.baseUrl = baseUrl;
    }

    async sendMessage(message, sessionKey = 'api:js') {
        const response = await fetch(`${this.baseUrl}/message`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ message, session_key: sessionKey })
        });
        return response.json();
    }

    async executeWorkflow(workflowName, variables = {}) {
        const response = await fetch(`${this.baseUrl}/workflow`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                workflow_name: workflowName,
                variables
            })
        });
        return response.json();
    }

    async getStatus() {
        const response = await fetch(`${this.baseUrl}/status`);
        return response.json();
    }
}

// Usage
const client = new PepebotClient();

// Send message
const result = await client.sendMessage('Hello!');
console.log(result.response);

// Execute workflow
const workflowResult = await client.executeWorkflow('device_health', {
    device: 'emulator-5554'
});
console.log(workflowResult);
```

### cURL Examples

**Send message:**
```bash
curl -X POST http://localhost:18790/message \
  -H "Content-Type: application/json" \
  -d '{"message":"Hello Pepebot!"}'
```

**Execute workflow:**
```bash
curl -X POST http://localhost:18790/workflow \
  -H "Content-Type: application/json" \
  -d '{
    "workflow_name":"device_health",
    "variables":{"device":"emulator-5554"}
  }'
```

**Get status:**
```bash
curl http://localhost:18790/status | jq
```

**List sessions:**
```bash
curl http://localhost:18790/sessions | jq
```

**Clear session:**
```bash
curl -X DELETE http://localhost:18790/session/gateway:default
```

### Go Client

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type PepebotClient struct {
    BaseURL string
    Client  *http.Client
}

func NewPepebotClient(baseURL string) *PepebotClient {
    return &PepebotClient{
        BaseURL: baseURL,
        Client:  &http.Client{},
    }
}

func (c *PepebotClient) SendMessage(message, sessionKey string) (map[string]interface{}, error) {
    payload := map[string]interface{}{
        "message":     message,
        "session_key": sessionKey,
    }

    data, _ := json.Marshal(payload)
    resp, err := c.Client.Post(
        c.BaseURL+"/message",
        "application/json",
        bytes.NewBuffer(data),
    )
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    return result, nil
}

func main() {
    client := NewPepebotClient("http://localhost:18790")

    result, err := client.SendMessage("Hello!", "api:go")
    if err != nil {
        panic(err)
    }

    fmt.Println(result["response"])
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

## WebSocket Support

WebSocket support is planned for future releases. Current implementation uses HTTP long-polling.

**Roadmap:**
- WebSocket for real-time streaming
- Server-sent events (SSE) for updates
- GraphQL API (optional)

---

## Error Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 400 | Bad Request (invalid JSON, missing required fields) |
| 404 | Not Found (invalid endpoint, session not found) |
| 500 | Internal Server Error (agent failure, provider error) |
| 503 | Service Unavailable (agent busy, provider timeout) |

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

**API Version:** 0.4.0
**Last Updated:** 2026-02-17
**Status:** Stable
