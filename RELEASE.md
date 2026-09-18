# 🐸 Pepebot v0.5.24 - Your Bot Can See Again

**Release Date:** 2026-09-18

## 🐛 What's Fixed

### Send it a photo and it actually looks at it

Images sent through Discord, Telegram or WhatsApp were handed to the model as a link, which meant the *model's* servers had to go and download it. Measured against a live endpoint, that never worked: `zai/glm-4.5v` refuses every remote image URL — even a plain public one — with `图片输入格式/解析错误`, while the exact same image sent inline is described correctly.

So the bot wasn't blind. It was being handed a URL it couldn't open.

Pepebot now downloads images and sends the real bytes, the same way it already does for PDFs and documents since v0.5.21. Chat attachments are signed links that expire anyway, so nobody's provider could reliably fetch them.

Files that can't be fetched, or are over 20 MB, still degrade to a short text note instead of breaking the whole message.

### CI is green again

`go test ./...` also runs `go vet`, and seven stray `fmt.Println("…\n")` calls were failing its checks — so the test job had been red on every release for a while, no matter how the tests did. Fixed, with byte-identical output.

### Attach a file straight from the terminal

```bash
pepebot agent -m "Tulisan apa di gambar ini?" --media ./foto.png
pepebot agent -m "Ringkas laporan ini" --media https://example.com/laporan.pdf
```

Repeatable, and it takes the same path a Discord or Telegram attachment does — which is how the image fix above was verified end to end instead of by eye.

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
