# 🐸 Pepebot v0.5.25 - No More "No Response To Give"

**Release Date:** 2026-09-21

## 🐛 What's Fixed

### That apology message, explained and fixed

If your bot kept answering with:

```
I've completed processing but have no response to give.
```

…it wasn't confused. It was cut off mid-thought.

GLM models think out loud before answering, and that thinking is paid for out of the same `max_tokens` budget as the answer. Measured on a live stream, one ordinary reply was **3,246 characters of reasoning followed by 800 characters of answer** — four to one. When the budget ran out during the thinking part, the answer never got written, and pepebot had nothing to show you.

Three fixes, smallest first:

**1. If the answer is missing, you get the model's thinking instead.** A long, truncated answer is more useful than an apology. Reasoning is held back while a real answer is still coming, so normal replies look exactly as before.

**2. You can turn the thinking off.** In `~/.pepebot/config.json`:

```json
{
  "agents": {
    "defaults": {
      "extra_body": { "thinking": { "type": "disabled" } }
    }
  }
}
```

Four ways of asking for this were tested against a live endpoint; this is the only one that survives the proxy in between. With it, a 200-token budget that previously produced *nothing* produces a full answer.

**3. Your conversations stop being compacted for no reason.** `max_tokens` was doing double duty as both the reply budget and the context window, so a bot on a 128k model was summarizing its history as if it had 8k. There is now a separate setting:

```json
{ "agents": { "defaults": { "max_tokens": 8192, "context_window": 128000 } } }
```

Older configs keep working — if `context_window` is absent, `max_tokens` is used as before.

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
- [Memory Guide](docs/memory.md)
- [Documentation](docs/README.md)
- [Installation Guide](docs/install.md)
