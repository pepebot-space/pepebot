# 🐸 Pepebot v0.5.34 - You Can See It Learning

**Release Date:** 2026-10-16

## ⚡ What's New

v0.5.33 taught the bot to keep what it works out. This answers the obvious next question: **how would you know?**

Until now: you wouldn't. It changed its memory and wrote skills, and mentioned it only in a log line on the server.

### It tells you

```
💾 Dicatat (2 hal)
```

A short note in the same conversation, right after the background review keeps something.

```json
{ "memory": { "notify": "verbose" } }    // off | on (default) | verbose
```

`verbose` says what it kept.

### You can ask

```sh
$ pepebot learned

🧠 Ingatan
──────────────────────────────────

Catatan agent (159/2200 karakter)
  • Rilis pepebot: melompati langkah 1-2 (go test & golangci-lint harus 0)
    sudah dua kali membuat rilis gagal di CI.

Profil kamu (31/1375 karakter)
  • Selalu pakai zsh sebagai shell.

📘 Skill yang ditulis sendiri
──────────────────────────────────
  • pepebot-release              2026-10-16 15:16
    Open when preparing or performing a release of pepebot.

  Hapus satu: rm -rf ~/.pepebot/workspace/skills/<nama>
```

Everything the bot taught itself, in one place, with the command to undo any of it.

## 📦 Installation

```bash
curl -fsSL https://raw.githubusercontent.com/pepebot-space/pepebot/main/install.sh | bash
```

## 🔗 Links

- [Changelog](CHANGELOG.md)
- [Agent-written skills](docs/skills-agent.md)
- [Memory Guide](docs/memory.md)
