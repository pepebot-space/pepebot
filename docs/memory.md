# Memory

Pepebot carries a small set of curated notes into every session, and reviews each
conversation in the background to decide whether anything new belongs there.

## The two stores

| File | Holds | Default limit |
|------|-------|---------------|
| `workspace/memory/MEMORY.md` | the agent's own notes — projects, environment, conventions | 2,200 chars (~800 tokens) |
| `workspace/USER.md` | the user profile — preferences, communication style | 1,375 chars (~500 tokens) |

Both are rendered into the system prompt with a capacity header, so the agent can
see how much room is left:

```
MEMORY [67% — 1,474/2,200 chars]
──────────────────────────────────────────────
Server produksi: myrbot, alamat Tailscale 100.81.179.88
§
Database produksi: PostgreSQL 16, port 5433
```

Entries are separated by `§`. A file written before this system existed has no
delimiters and is read as a single entry, so nothing is lost on upgrade.

### The limit is the feature

There is no auto-compaction. When a write would exceed the limit the call fails
with `MEMORY is full (2,340/2,200 chars). Consolidate or remove an entry, then
retry`, and the agent has to merge two entries or drop a stale one itself.

Silently dropping the oldest entry to make room would throw away something the
user asked to be remembered, at the moment nobody is watching. An error keeps the
notes curated rather than merely long — which is the whole point, because every
character here is paid for on every request. Before this existed, one deployment's
`MEMORY.md` had grown to 14.5k characters (~3.6k tokens) on every single call.

A store that is *already* over its limit — every workspace that predates this
system — can still be shrunk: a write that reduces the size is allowed through
even while it is still over, so the agent can dig out instead of being frozen.

## The `memory` tool

```
memory(action="add",     target="memory", content="...")
memory(action="replace", target="user",   old_text="zsh", content="...")
memory(action="remove",  target="memory", old_text="PostgreSQL")
```

`old_text` matches by substring, so an entry can be named by a fragment. Exact
duplicates are reported and not stored twice.

Entries are scanned before they are accepted. Content carrying instruction
overrides (`ignore previous instructions`), credential material (`BEGIN OPENSSH
PRIVATE KEY`), or invisible/bidi characters is refused — memory is written from
conversation and read back as the agent's own notes forever after, so an injected
instruction would persist across every future session.

## Background review

After every `review_interval` turns, pepebot replays the recent conversation to
the model and asks what, if anything, should be remembered. It runs in a
goroutine, never blocks the reply, and returns plain JSON — it has no tools, so it
cannot touch the filesystem or the network. Writes go through the same stores as
the tool, so limits, duplicate checks and scanning all still apply.

This is what makes memory work with models that are unreliable at tool calling:
in testing, a model answered "noted!" without calling `memory` at all, and the
review saved the fact anyway.

## Configuration

```json
{
  "memory": {
    "enabled": true,
    "char_limit": 2200,
    "user_char_limit": 1375,
    "review": true,
    "review_interval": 5
  }
}
```

| Key | Meaning |
|-----|---------|
| `enabled` | `false` unregisters the tool and stops rendering the stores |
| `char_limit` / `user_char_limit` | size ceilings, in characters |
| `review` | `false` keeps the tool but stops the automatic review |
| `review_interval` | turns between reviews; each one costs an auxiliary LLM call |

Environment overrides follow the usual pattern: `PEPEBOT_MEMORY_ENABLED`,
`PEPEBOT_MEMORY_CHAR_LIMIT`, `PEPEBOT_MEMORY_USER_CHAR_LIMIT`,
`PEPEBOT_MEMORY_REVIEW`, `PEPEBOT_MEMORY_REVIEW_INTERVAL`.

On a busy deployment the review is the part worth switching off first — it is one
extra model call per interval, per session.
