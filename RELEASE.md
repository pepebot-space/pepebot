# 🐸 Pepebot v0.5.28 - Voice Without The Wait

**Release Date:** 2026-10-03

## ⚡ What's New

### Turn off tools for voice, and get 1.5 seconds back

Tool definitions are sent to the model again on every single turn. In a text chat nobody notices. On a voice call it is the whole experience:

| Live session | Time to first token |
|---|---|
| Without tools | **591 ms** |
| With pepebot's 16 tools attached | **2131 ms** |

Measured end to end against a real voice server one LAN hop away, five samples each.

Until now the only way to switch them off was for every client to say so in its setup message. Now the deployment can decide:

```json
{ "live": { "enable_tools": false } }
```

A client that asks either way still wins, and leaving it unset keeps tools on — nothing changes for anyone who does not set it.

### How much does pepebot's Live proxy cost? Almost nothing

While measuring the above, the proxy itself came out at **585 ms** to first token versus **595 ms** talking straight to the voice server. The relay is in the noise; what costs you is the tool prefill above, and how far away your voice server is.

## 📦 Installation

```bash
curl -fsSL https://raw.githubusercontent.com/pepebot-space/pepebot/main/install.sh | bash
```

## 🚀 Quick Start

```bash
pepebot onboard
pepebot gateway
```

## 🔗 Links

- [Changelog](CHANGELOG.md)
- [Providers Guide](docs/providers.md)
- [Memory Guide](docs/memory.md)
- [Documentation](docs/README.md)
