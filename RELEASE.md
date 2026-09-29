# 🐸 Pepebot v0.5.27 - Talks To Gateways That Bend The Rules

**Release Date:** 2026-09-29

## 🐛 What's Fixed

### A reply that ends with `data: [DONE]` no longer breaks everything

Some gateways — 9Router among them — answer a plain, non-streaming request with `Content-Type: text/event-stream` and glue `data: [DONE]` onto the end of the JSON. Strictly speaking that is not a JSON document, and pepebot's parser refused all of it: the answer was sitting right there and every single request failed anyway.

Pepebot now reads the JSON object and ignores whatever framing follows it. Point it at a gateway like that and it just works.

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
- [Providers Guide](docs/providers.md)
- [Memory Guide](docs/memory.md)
- [Documentation](docs/README.md)
