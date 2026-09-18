# 🐸 Pepebot v0.5.22 - Memory That Stays Small

**Release Date:** 2026-09-18

## ⚡ What's New

### Your bot remembers you — without the notes taking over the prompt

Pepebot has always kept long-term notes in `MEMORY.md`. The problem was that nothing stopped them growing: on one live deployment that file had reached **14,559 characters — about 3,600 tokens paid on every single message**, most of it irrelevant to whatever was just asked.

Memory is now **bounded and curated**. Two stores, each with a ceiling:

| Store | Holds | Limit |
|---|---|---|
| `MEMORY.md` | projects, environment, conventions | 2,200 chars |
| `USER.md` | your preferences and style | 1,375 chars |

The agent sees how full they are right in its prompt (`MEMORY [67% — 1,474/2,200 chars]`) and edits single entries through a new `memory` tool instead of rewriting whole files.

**When a store is full, the write fails.** That is deliberate. Pepebot will not quietly delete the oldest thing you asked it to remember to make room — it consolidates two entries into one, or drops something stale, and tells you it did.

Already have a huge `MEMORY.md`? Nothing is lost. It keeps working, keeps being read, and the agent is allowed to shrink it even while it is still over the line.

### It learns without being told to

After every few turns, pepebot quietly reviews the conversation in the background and decides whether anything is worth keeping — a preference you stated, a correction you had to repeat twice, how your machine is set up. It never blocks your reply, and it has no tools, so it can only write memory.

This also covers a very real failure: in testing, the model replied "noted!" and never called the memory tool at all. The background review saved the fact anyway.

### Nothing dangerous gets into memory

Memory is written from conversation and read back as the agent's own notes forever after — so anything hidden in there would come back every session. Entries carrying instruction overrides, private keys, or invisible characters are refused at the door.

## 🔧 Configuration

```json
{
  "memory": {
    "enabled": true,
    "char_limit": 2200,
    "user_char_limit": 1375,
    "review": true,
    "review_interval": 5
  }
}
```

Set `review: false` to keep the memory tool but stop the automatic reviews — it is one extra model call per interval.

Full details: [docs/memory.md](docs/memory.md)

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
