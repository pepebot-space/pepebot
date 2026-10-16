# 🐸 Pepebot v0.5.32 - Housekeeping That Pays Rent

**Release Date:** 2026-10-16

## ⚡ What's New

### Pepebot now tells you when your config is wrong

Every check added here is a mistake that really happened and took an afternoon to find, because the complaint always arrived somewhere else:

| What you wrote | What you used to see |
|---|---|
| `provider: maiarouter` + `model: maiarouter/zai/glm-5.3` | `no healthy deployments for this model` |
| `model: custom:r9/myr` with no `providers.custom.r9` | a dial error |
| `context_window` below `max_tokens` | nothing — sessions just compacted far too early, forever |

Now pepebot says so at startup, naming the setting and the fix:

```
WARN config: agents.defaults.model "maiarouter/zai/glm-5.3" repeats the provider name;
     write "zai/glm-5.3", or use the canonical "maiarouter:zai/glm-5.3"
```

These are warnings. A bot that starts and complains beats one that refuses to start.

## 🧹 Cleanup

### The lint job is green for the first time

63 findings down to 0. That matters beyond tidiness: with the job permanently red, a new problem looked exactly like the old noise.

Most of them were dropped errors, and several were real:

- a session that failed to save — the conversation gone at the next restart, with nothing in the log
- cron schedules that failed to persist
- a Telegram reply that never reached the user
- a config file that onboarding never actually wrote

Those now report. The ones that genuinely cannot be acted on — writing to a client that already hung up — are marked as deliberate, with the reason written down once.

Also: five dead functions deleted, and the turn-completion sequence (record, summarize, review, save) that had been copied into three code paths is now one function. Every past fix to it had to be made three times, and the streaming copies were each missed once.

## 📦 Installation

```bash
curl -fsSL https://raw.githubusercontent.com/pepebot-space/pepebot/main/install.sh | bash
```

## 🔗 Links

- [Changelog](CHANGELOG.md)
- [Providers Guide](docs/providers.md)
- [Documentation](docs/README.md)
