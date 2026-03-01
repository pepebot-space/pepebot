# 🐸 Pepebot v0.5.5 - MCP Registry Management

**Release Date:** 2026-03-01

## 🎉 What's New

### 🔌 Native MCP Registry Tooling

Pepebot can now manage MCP server definitions directly from the agent using a new tool: `manage_mcp`.

- Add/update MCP server definitions
- List all configured MCP servers
- Remove MCP servers
- Support for `stdio`, remote `sse`, and remote `http`

### ⚙️ MCP Runtime Loading

Pepebot now tries to initialize enabled MCP servers on startup and auto-register discovered MCP tools into the normal tool list used by the agent.

- Dynamic `tools/list` discovery from MCP servers
- MCP tool calls proxied into regular agent tool execution
- Startup diagnostics for MCP init/list failures

Example:

```json
{
  "action": "add",
  "name": "my-remote-mcp",
  "transport": "http",
  "url": "https://mcp.example.com",
  "headers": {
    "Authorization": "Bearer ${MCP_TOKEN}"
  }
}
```

### 🧩 Skills Can Auto-Register MCP

Skills can now declare MCP servers in `SKILL.md` frontmatter via `mcp` entries.
When the agent loads context, those MCP definitions are synced automatically into:

`~/.pepebot/workspace/mcp/registry.json`

That means a skill can bootstrap required MCP endpoints without manual setup.

### 🪵 Better Verbose Logs in `agent`

`pepebot agent` now supports `-v` / `--verbose` to show richer DEBUG traces:

- provider request/response diagnostics
- tool calls, tool arguments, and tool output previews
- easier troubleshooting when MCP tool routing is missing

You can now run one-shot messages with positional syntax too:

```bash
pepebot agent "pakai mcp browser untuk cari harga tiket ke japan" --verbose
```

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
Download the appropriate binary for your platform from the [releases page](https://github.com/pepebot-space/pepebot/releases/tag/v0.5.5).

## 🚀 Quick Start

1. Start agent:
   ```bash
   pepebot agent -m "list mcp servers"
   ```
2. Ask agent to add MCP:
   ```bash
   pepebot agent -m "tambahkan MCP remote http bernama docs-mcp ke https://mcp.example.com"
   ```
3. Or use it in workflow steps via `manage_mcp`.

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

For a complete list of changes, see [CHANGELOG.md](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md#055---2026-03-01).
