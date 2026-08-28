# 🐸 Pepebot v0.5.20 - Just the Model, Please

**Release Date:** 2026-08-29

## 🐛 What's Fixed

### The provider name no longer travels inside the model id

With `provider: "maiarouter"` set and a model of `maiarouter/zai/glm-5.3-flash`, the whole string went to the API — and the upstream refused it:

```
litellm.BadRequestError: You passed in model=maiarouter/zai/glm-5.3-flash.
There are no healthy deployments for this model.
```

Choosing the endpoint is the config's job. The provider key is now stripped from the front of the model before the request is built, on both the streaming and non-streaming paths.

Only that key is removed, never a vendor namespace — OpenRouter genuinely wants `anthropic/claude-3.5-sonnet` and MAIA genuinely wants `maia/gemini-2.5-flash`, so `maia/` survives even when `maia` is the configured alias.

Worth knowing if you hit this: the composed value usually lives in the **agent registry** entry (`workspace/agents/registry.json`), not in `config.json` — which is why the symptom outlives fixing the config. This release makes it harmless either way.

### The debug log now tells the truth

`pepebot agent -v` reported the model as configured, so it agreed with `config.json` while the wire carried something else. It now prints what is actually sent — which is how you would have caught the above in a minute instead of an afternoon.

## 📦 Installation

```bash
curl -fsSL https://raw.githubusercontent.com/pepebot-space/pepebot/main/install.sh | bash
```

Or with Homebrew:

```bash
brew tap pepebot-space/tap https://github.com/pepebot-space/homebrew-tap
brew install pepebot
```

## 🔎 Checking your own setup

```bash
pepebot agent -v -m "hi" 2>&1 | grep "HTTP chat request"
```

The `model=` in that line is exactly what leaves the machine.

## 🔗 Links

- [Changelog](./CHANGELOG.md)
- [README](./README.md)
