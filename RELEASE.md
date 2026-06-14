# 🐸 Pepebot v0.5.16 - Give Your Live Session a Role

**Release Date:** 2026-06-14

## 🎉 What's New

### 🧠 System prompts for the Live API

Live voice/vision sessions can finally be told *how to behave*. Set a role, persona, or task and the Vertex/Gemini Live model follows it:

- **Config default** — `live.system_prompt` (or `live.system_prompt_file`, env `PEPEBOT_LIVE_SYSTEM_PROMPT`).
- **Per-session override** — a client can pass `system_prompt` in its `setup` message, perfect for task-specific clients (e.g. an autonomous rover that sets its mission per run).
- **Reuse your agent's persona** — flip `live.use_agent_prompt: true` and the Live session inherits the selected agent's `AGENTS.md`/`SOUL.md`/`IDENTITY.md`, so it sounds the same in chat and in voice.

If you set nothing, behavior is unchanged — no surprises.

```json
{ "live": { "system_prompt": "You are LEXA, an autonomous rover. Perceive → act → repeat, avoid obstacles, stop on command. Narrate briefly." } }
```

---

## 🎉 Previous Highlights

### 👁️ Live API sees your camera faster

When you stream your camera to a Live session (Vertex/Gemini), the model now reacts to what it sees much sooner:

- **Faster turn-taking** — the assistant decides you've stopped talking quicker (`END_SENSITIVITY_HIGH` + a shorter `silenceDurationMs: 500`), so it looks at the latest frame and replies with less lag.
- **Lighter frames** — video frames default to `MEDIA_RESOLUTION_LOW` (~280 tokens each), cutting cost and inference latency. Tune it via `live.media_resolution` or `PEPEBOT_LIVE_MEDIA_RESOLUTION`.
- **📸 Snapshot button** — the video example (`examples/live-api/index-video.html`) now has a **Snapshot** button that pushes the current frame as a committed turn, forcing an instant "look at this now" without waiting for you to finish speaking.

### 🛠️ Tool calls no longer freeze the conversation

A slow tool call during a Live session used to block the model's audio and incoming video frames. Tool calls now run off the proxy loop, so audio and video keep flowing smoothly while a tool runs.

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
Download the appropriate binary for your platform from the [releases page](https://github.com/pepebot-space/pepebot/releases).

## 🚀 Quick Start

1. Run setup wizard: `pepebot onboard`
2. Start interactive agent: `pepebot agent`
3. (Optional) Start API gateway: `pepebot gateway`

## 📚 Documentation

- [Workflow Guide](https://github.com/pepebot-space/pepebot/blob/main/docs/workflows.md)
- [Installation Guide](https://github.com/pepebot-space/pepebot/blob/main/docs/install.md)
- [Full Changelog](https://github.com/pepebot-space/pepebot/blob/main/CHANGELOG.md)

## 🔗 Links

- **GitHub**: https://github.com/pepebot-space/pepebot
- **Documentation**: https://github.com/pepebot-space/pepebot/tree/main/docs
- **Issues**: https://github.com/pepebot-space/pepebot/issues
- **Discussions**: https://github.com/pepebot-space/pepebot/discussions
