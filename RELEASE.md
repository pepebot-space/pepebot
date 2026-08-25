# 🐸 Pepebot v0.5.18 - Your Agent, Out Loud

**Release Date:** 2026-08-25

## 🎉 What's New

### 🎙️ Voice sessions are real agent sessions now

The Live API used to be a voice pipe with a personality. On the OpenAI Realtime protocol it was barely even that: tool definitions and the system prompt were only ever injected into a provider setup frame, and Realtime providers do not have one — so both were silently dropped. Voice could talk. It could not *do* anything.

That is fixed, and then some. A Realtime voice session now carries:

- **Tools** — the agent's full tool registry, executed locally and fed back. Ask it to read a file and it reads the file.
- **Skills** — the same skills block a text conversation gets. Attached independently of the persona, so setting a system prompt no longer drops them.
- **Memory** — voice turns are written to the agent's session history. Talk to it, then message it on Telegram with the same session key, and it remembers.
- **A reply language** — `live.language` (default `id-ID`) or per session via `setup.language`.

### 🔌 Custom OpenAI Realtime endpoints

New `providers.realtime` config block. Point `api_base` at any server speaking the OpenAI Realtime protocol — self-hosted included — and `api_key` may be empty, because most of them run without auth. It is Live-API-only, so an OpenAI key you use for chat is untouched.

```json
{
  "providers": { "realtime": { "api_base": "http://127.0.0.1:8000/v1", "api_key": "" } },
  "live": { "enabled": true, "provider": "realtime", "model": "your-model" }
}
```

### 🔊 Replies that sound like speech

Pepebot's `session.update` replaces the upstream server's own instructions — including whatever "this will be spoken" guidance they carried. Which is why answers were coming back as markdown tables and emoji, read out loud symbol by symbol. Every Realtime session now carries an explicit speech directive, so a list is spoken as a sentence instead of a bullet list.

### 🔁 Upstream reconnect

If the upstream connection drops mid-session, the client stays connected while Pepebot rebuilds the other leg — 3 attempts, 1s/2s/4s backoff, replaying the persona, skills, tools and session config, then telling the client with a `{"status":"reconnected"}` frame.

### 🖥️ Three clients to try it with

- `examples/live-api/paniki.html` — browser: mic, barge-in, tool-call trace, model and voice pickers read live from the server
- `examples/live-api/nodejs/paniki-client.js` — terminal, full duplex, mic via ffmpeg
- `examples/live-api/paniki_client.py` — terminal, full duplex, mic via sounddevice

All three read `/v1/models` and `/v1/voices` at startup, so nothing goes stale when the server is redeployed.

## 📦 Installation

```bash
curl -fsSL https://raw.githubusercontent.com/pepebot-space/pepebot/main/install.sh | bash
```

Or with Homebrew:

```bash
brew tap pepebot-space/tap https://github.com/pepebot-space/homebrew-tap
brew install pepebot
```

## 🚀 Quick Start

```bash
pepebot gateway                                  # /v1/live on 127.0.0.1:18790
cd examples/live-api/nodejs && npm install && node paniki-client.js
```

Then ask it to do something — list a directory, read a file — and listen to it actually do it.

## 🔗 Links

- [Changelog](./CHANGELOG.md)
- [Live API guide](./docs/live-api.md)
- [Custom Realtime endpoints](./examples/live-api/README-realtime.md)
