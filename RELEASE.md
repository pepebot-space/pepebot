# 🐸 Pepebot v0.5.31 - Tell Me Why

**Release Date:** 2026-10-09

## 🐛 What's Fixed

### "I've completed processing but have no response to give."

That sentence told you nothing, and it was hiding two very different events.

When a safety classifier declines a request, the API answers **200 OK with no content** and a reason attached. When an answer is cut off before the model writes a word, same thing — empty, with a different reason. Pepebot read the (missing) content, dropped the reason on the floor, and shrugged.

Now you get the actual reason — the request was declined, or the token budget ran out — and the log records it either way.

### The cron service no longer dies on an empty file

```
Error starting cron service: failed to load store: unexpected end of JSON input
```

A crash between creating `jobs.json` and writing to it leaves a zero-byte file. That is a schedule with nothing in it, not a corrupt one — but pepebot treated it as corrupt and failed on every start. A genuinely malformed file is still reported.

### Smaller

Provider log lines name the endpoint that produced them instead of always saying "OpenCode Go", and the debug line now includes the finish reason.

## 📦 Installation

```bash
curl -fsSL https://raw.githubusercontent.com/pepebot-space/pepebot/main/install.sh | bash
```

## 🔗 Links

- [Changelog](CHANGELOG.md)
- [Providers Guide](docs/providers.md)
- [Documentation](docs/README.md)
