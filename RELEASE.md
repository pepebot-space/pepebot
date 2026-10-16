# 🐸 Pepebot v0.5.35 - Hand It Off, Get Told When It's Done

**Release Date:** 2026-10-16

## ⚡ What's New

### Delegation that actually comes back

Ask for something slow and the bot no longer has to sit on the question:

```
You:  analisa 92 skill ini, mana yang tidak pernah dipakai
🐸    Saya kerjakan di latar, id t_8a373014.
      …
🐸    ✅ Tugas t_8a373014 (audit skill) selesai.
      57 dari 92 skill tidak pernah terpakai. Tiga terbesar: …
```

The result is **read by the bot before you see it** — you get the conclusion, not a dump of raw output.

### The work can run anywhere

A task names a route; what sits behind it is yours:

```json
{ "tasks": { "routes": {
    "riset":  { "executor": "exec", "target": "ssh herdr 'claude -p \"$PEPEBOT_TASK_PROMPT\"'" },
    "render": { "executor": "http", "target": "http://100.64.0.9:9000/jobs" }
}}}
```

Sending a job to Claude on another machine is **a line of config**, not a feature pepebot had to learn. Pepebot never knows what is on the other end.

### The hook

A remote worker is handed where to report and a token that works for that one task:

```sh
curl -X POST "$callback" -H "Authorization: Bearer $token" \
     -d '{"status":"done","result":"Temuan: 3 anomali, terbesar di baris 418."}'
```

Send it twice and you still get one answer in the chat.

### A restart no longer swallows your work

Kill pepebot mid-task and it tells you when it comes back:

```
⚠️ Tugas t_a5e75c9f (impor data besar) terhenti karena pepebot restart.
   Mau dilanjutkan? Balas "lanjutkan t_a5e75c9f" atau "batalkan t_a5e75c9f".
```

Your call, not the bot's.

Full details: [docs/tasks.md](docs/tasks.md)

## 🐛 Fixed along the way

Three bugs the tests and the end-to-end run turned up: a worker could overwrite a task that had already finished (and report it twice), the hook rejected valid callbacks coming from a different process, and a one-shot CLI run exited while its own delegated task was still going.

## 🧹 Removed

The old `spawn` sketch: never registered, no tools, results delivered nowhere, ids lost on every restart.

## 📦 Installation

```bash
curl -fsSL https://raw.githubusercontent.com/pepebot-space/pepebot/main/install.sh | bash
```

## 🔗 Links

- [Changelog](CHANGELOG.md)
- [Delegation guide](docs/tasks.md)
- [Agent-written skills](docs/skills-agent.md)
