# 🐸 Pepebot v0.5.30 - Send It Any Document

**Release Date:** 2026-10-05

## 🐛 What's Fixed

### PDFs from chat finally arrive

v0.5.29 made PDFs work — from the command line. Attach one in Discord and it still failed, for a dull reason: chat attachments are signed links like `.../laporan.pdf?ex=68a9&hm=9f3a`, and the code reading the file extension got `.pdf?ex=68a9&hm=9f3a`, which matches no file type at all. Every attachment was labelled "unknown binary", and the model refused it.

Fixed, and the same bug was quietly breaking **images** from chat too — a `.png?ex=…` was mislabelled exactly the same way.

Pepebot now also checks the file's actual contents when a server is unhelpful, so an attachment served as "unknown binary" with no extension in the link is still recognised.

## ⚡ What's New

### Word, Excel and PowerPoint

No model can read a `.docx` — it is a zip file full of XML. Attach one and nothing useful happened, on any provider.

Pepebot now opens them and sends the text:

```
You:  [notulen.docx] Kode dokumen apa yang disebut?
🐸    Kode dokumen: NOTULEN-4412
```

Spreadsheets come through as rows (`Tayangan | 478.307`), presentations slide by slide. Works on every model, including the ones that cannot take documents at all.

Very long documents are cut at 200k characters with a note saying so, rather than silently crowding out the conversation.

### An unsupported file no longer kills the message

Send something nothing can read and you get a short note about that one attachment — the rest of your message still gets answered, instead of the whole request failing.

## 📦 Installation

```bash
curl -fsSL https://raw.githubusercontent.com/pepebot-space/pepebot/main/install.sh | bash
```

## 🔗 Links

- [Changelog](CHANGELOG.md)
- [Providers Guide](docs/providers.md)
- [Documentation](docs/README.md)
