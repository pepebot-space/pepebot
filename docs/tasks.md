# Delegation: tasks and the completion hook

Some work does not fit in a reply. Pepebot hands it off, answers with an id, and
comes back to the same conversation when it is done.

```
You:  analisa 92 skill ini, mana yang tidak pernah dipakai
🐸    Saya kerjakan di latar, id t_8a373014. Hasilnya saya kabari di sini.
      …
🐸    ✅ Tugas t_8a373014 (audit skill) selesai.
      57 dari 92 skill tidak pernah terpakai. Tiga terbesar: …
```

## Where work can go

A task names a **route**; what sits behind the route is yours to decide.

```json
{
  "tasks": {
    "enabled": true,
    "workers": 0,
    "callback_url": "http://100.81.179.88:18790",
    "routes": {
      "riset":  { "executor": "exec", "target": "ssh herdr 'claude -p \"$PEPEBOT_TASK_PROMPT\"'" },
      "render": { "executor": "http", "target": "http://100.64.0.9:9000/jobs" },
      "sendiri":{ "executor": "internal" }
    }
  }
}
```

| Executor | What it does |
|---|---|
| `internal` | pepebot's own agent loop, with its tools, in a session of its own |
| `exec` | runs a command — the general door to anything, including Claude on another host |
| `http` | posts the job to a worker and waits for the hook |

The prompt reaches an `exec` command as `$PEPEBOT_TASK_PROMPT`, never spliced
into the command line: a task prompt comes from a chat message, and splicing
that into a shell line is how a question becomes a command.

`workers: 0` means the host's thread count. The queue is bounded; a burst of
delegations waits rather than starting an unbounded number of subprocesses.

## The completion hook

An `http` worker is handed everything it needs to report back:

```json
{
  "task_id": "t_ed58c719",
  "prompt": "analisa log produksi, cari anomali",
  "callback": "http://100.81.179.88:18790/v1/tasks/t_ed58c719/result",
  "token": "…"
}
```

When it is done:

```sh
curl -X POST "$callback" \
     -H "Authorization: Bearer $token" \
     -H 'Content-Type: application/json' \
     -d '{"status":"done","result":"Temuan: 3 anomali, terbesar di baris 418."}'
```

The token authenticates **one task**. A leaked one finishes that job and
nothing else. A retried callback is accepted once: a worker that sends twice
does not produce two answers in the chat.

### Pepebot has to be reachable

`callback_url` exists because pepebot cannot guess it. The gateway binds
`127.0.0.1` by default, which is correct and useless to a worker on another
machine. Set it to the address that worker can actually route to — a Tailscale
IP, for instance — and pepebot hands that out with every job. Dispatch fails
loudly when a remote route is configured without it, rather than sending work
that can never report back.

## When pepebot restarts mid-task

Nothing finishes it: an in-process job died with the process, and a remote one
has a callback going nowhere. The task becomes `stale` and the requester is
told, with the choice:

```
⚠️ Tugas t_a5e75c9f (impor data besar) terhenti karena pepebot restart.
   Mau dilanjutkan? Balas "lanjutkan t_a5e75c9f" atau "batalkan t_a5e75c9f".
```

Silence was the old behaviour — the sketch this replaces kept tasks in memory
and lost every one of them on restart without a word.

## Limits

- **Depth 3.** A task may delegate, and so may that task, three levels deep.
- **Results are summarised.** Anything longer than ~4000 characters is written
  to `workspace/task-results/<id>.txt` and the chat gets the gist plus the path.
- **One workspace, one writer.** Readers re-read the task file when another
  process has written it — that is how the CLI can delegate and the gateway can
  serve the hook — but two pepebot processes delegating into one workspace at
  the same moment can still lose a record.

## Looking at them

```sh
curl localhost:18790/v1/tasks            # all of them
curl localhost:18790/v1/tasks/t_8a37     # one
```

In chat: ask the bot. `task` with `action=status` lists what is running,
`cancel` drops one, `resume` re-runs one that a restart interrupted.
