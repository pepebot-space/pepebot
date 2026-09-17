# 🐸 Pepebot v0.5.21 - Skills That Fit, Attachments That Arrive

**Release Date:** 2026-09-17

## ⚡ What's New

### Your skills no longer eat the whole context window

Pepebot used to paste every `SKILL.md` into the prompt, in full, on every single message. With a real skills folder that is fatal: 89 skills came to roughly **335,000 tokens** before the user had typed a word, so a 128k model refused outright — and the requests that did fit were paying for all 89 skills to use at most one.

Pepebot now sends the **catalogue, not the contents**: each skill's name, description and where it lives. When a task needs a skill, the agent opens that one file and follows it.

Same 89 skills, same machine: **~7k tokens instead of ~335k — a 97.9% cut**, and nothing had to be disabled to make it fit. If you parked skills to get under a limit, you can put them all back.

## 🐛 What's Fixed

### Skill frontmatter finally does something

The `---` block at the top of every `SKILL.md` was being read with a JSON parser, through a regex that could not match more than one line. In other words: never. Descriptions came out blank, `requires:` gated nothing, and MCP servers declared by a skill were quietly ignored.

It now parses as the YAML it always was. Your descriptions show up, a skill that needs a missing tool is correctly marked unavailable, and skill-provided MCP servers actually register.

### Documents reach the model as real bytes

Attach a file on Discord and the bot used to answer with an error:

```
Error processing message: LLM call failed: API error:
messages[0].content[0].file must contain at least one of file_id, file_url, or file_data
```

The attachment was handed over as a bare CDN link, in a field that only accepts inline data — and those signed, expiring links were never fetchable by the provider anyway. Pepebot now downloads the file and sends the actual bytes.

A file that can't be fetched, or is over 20 MB, degrades to a plain-text note instead of killing the whole message. Images are unchanged and still travel as links, so ordinary requests stay small.

> Whether a document is *understood* is still up to your model — some vision models accept images but reject PDFs. Pepebot's side of the handoff is now correct either way.

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
- [Documentation](docs/README.md)
- [Installation Guide](docs/install.md)
