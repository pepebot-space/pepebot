# 🐸 Pepebot v0.5.19 - The Device Answers Back

**Release Date:** 2026-08-27

## 🎉 What's New

### 🔧 Apps bring their own tools

Until now the agent could only use tools that run on the gateway host. A phone, a rover, a browser tab has things that host will never have — a camera, GPS, a clipboard, a screen, motors.

An app connecting to `/v1/live` can now declare its own tools and execute them itself:

```json
{"setup": {"provider": "realtime", "app": "rover",
           "tools": [{"name": "take_photo", "description": "...", "parameters": {...}}]}}
```

The model sees `rover-take_photo`; the app is asked for it by the bare name and answers with a `tool_result`. Gateway tools are all `snake_case`, so the hyphen makes a collision structurally impossible rather than merely forbidden — and a slow or absent device hits a deadline instead of wedging the turn.

Try it: `node examples/live-api/nodejs/paniki-toolcall.js` declares `device_info`, `read_clipboard` and `notify`. Ask what's on your clipboard and watch your own machine answer.

### 💬 Named sessions, one per device

`setup.session_key` names the conversation, and that name is the memory boundary:

```json
{"setup": {"session_key": "rover-01"}}
{"setup": {"session_key": "hp-ibnu"}}
```

Same key, new connection? It remembers. Different key? A separate conversation that knows nothing of the first. Voice and text on the same key are one conversation.

This also fixes something v0.5.18 got half-right. That release recorded live turns into session history — but nothing ever read them back, so a session started amnesiac even when its key was reused. It does now: the last 20 turns are replayed as context.

### 🔁 A dropped upstream no longer ends the session

If the upstream connection fails mid-conversation, the client stays connected while Pepebot rebuilds the other leg — 3 attempts with backoff, replaying persona, skills, tools and session config, then a `{"status":"reconnected"}` frame. Caught a real outage during testing and behaved.

### 🖼️ Images, honestly documented

`docs/live-vision.md` covers what a Realtime session can and cannot do with images — measured against a live server, not assumed. Four block shapes work, video is frames-as-items (they are consumed, not accumulated), and the agent cannot send an image back over the protocol at all; a client tool that displays it is the answer.

## 🐛 Fixed

- **A gateway crash.** Routing a client tool call added a second writer to the client socket, and gorilla panics on concurrent writes — it took the whole gateway down mid-session on the test server. Found by deploying before merging, not by the test suite: nothing in it opened a real socket with two writers. Now it does.
- **CI can name a broken Android release token.** The release workflow's Android leg failed on eight consecutive releases with only `Resource not accessible by personal access token`. It now says which token setting to change, and `android-dispatch.yml` re-fires a release without rebuilding sixteen binaries.

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
pepebot gateway                                   # /v1/live on 127.0.0.1:18790
cd examples/live-api/nodejs && npm install
SESSION=laptop node paniki-toolcall.js
```

Then ask it what's on your clipboard.

## 🔗 Links

- [Changelog](./CHANGELOG.md)
- [Live API guide](./docs/live-api.md) — including per-device sessions
- [Images and video](./docs/live-vision.md)
- [Custom Realtime endpoints and client tools](./examples/live-api/README-realtime.md)
