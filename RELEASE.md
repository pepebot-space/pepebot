# 🐸 Pepebot v0.5.33 - It Keeps What It Works Out

**Release Date:** 2026-10-16

## ⚡ What's New

### Your bot can now write its own skills

Memory already remembered short facts — your preferences, your environment. But a *procedure* doesn't fit in one line, and pasting it into memory would make every single request pay for it.

Skills are the other half: read only when they're relevant, so they cost nothing until they're needed. Pepebot can now write them.

```
You:  [walks the bot through a 6-step debugging routine]
🐸    ...and I've saved that as a skill, so next time I'll follow it.

[new session, days later]
You:  Bot Discord diam lagi, gimana?
🐸    1. systemctl is-active — sering "active" padahal mati, jangan berhenti di sini...
```

Two things write them: the bot itself during a conversation, and the background review afterwards. In testing, told explicitly *not* to call any tool, the bot answered "ok" — and the review afterwards filed the procedure as a skill and the one durable fact as a memory entry, on its own.

### What it will not do

This is a **narrower** power than the file-writing the bot already had, not a wider one:

- it cannot touch a skill it did not write — your installed and hand-written skills are off limits
- it cannot overwrite one by picking a colliding name
- it cannot escape the skills folder — odd names are rejected, not cleaned up
- 32 KB per skill, because every skill's description is paid for on every request
- instruction overrides, private keys and invisible characters are refused

Every skill it writes is stamped `created_by: agent` with a timestamp, so finding them is a grep and removing one is a folder delete. Full details: [docs/skills-agent.md](docs/skills-agent.md).

Not for you? `{ "skills": { "agent_managed": false } }`.

## 🐛 What's Fixed

### One-shot runs never learned anything

`pepebot agent -m "..."` exited the instant it printed the reply, killing the memory review mid-thought. The always-on gateway was fine — which is exactly why nobody noticed: memory worked *there* and silently didn't *here*. The CLI now waits for background work before it exits.

## 📦 Installation

```bash
curl -fsSL https://raw.githubusercontent.com/pepebot-space/pepebot/main/install.sh | bash
```

## 🔗 Links

- [Changelog](CHANGELOG.md)
- [Agent-written skills](docs/skills-agent.md)
- [Memory Guide](docs/memory.md)
- [Documentation](docs/README.md)
