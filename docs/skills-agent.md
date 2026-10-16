# Skills the agent writes

Memory holds short facts that ride in every prompt. Skills hold procedures that
are read only when they are relevant — so the expensive knowledge lives where it
costs nothing until it is needed.

Pepebot can now write its own.

## Two ways a skill gets written

**The agent decides, during the conversation.** The `skill_manage` tool takes
`create`, `patch`, `delete` and `list`. It is narrower than the `write_file` the
agent already has: a validated name, the frontmatter the loader needs, a size
limit, and a `created_by: agent` marker.

**The background review decides, after it.** The same review that curates memory
may propose a procedure. It goes through the same author, so an unattended write
gets the same validation, the same content scan and the same marker as a
deliberate one. In testing, told explicitly not to call any tool, the agent
answered "ok" — and the review afterwards saved the procedure as a skill and the
one durable fact as a memory entry.

## What it will not do

- **Touch a skill it did not write.** Patch and delete check for the
  `created_by: agent` marker first. Improving an installed skill in place would
  be undone the next time that skill updates from its source.
- **Overwrite by name collision.** `create` refuses an existing name; patch is
  the way.
- **Escape the skills directory.** Names are `[a-z0-9-]`, 3-40 characters. A
  name like `../elsewhere` is rejected, not sanitised.
- **Grow without bound.** 32 KB per skill. Every skill's description is paid for
  in the skills summary on every request.
- **Accept content it should not read back as instructions.** Instruction
  overrides, private keys and invisible characters are refused — the same check
  memory entries get.

## Finding and removing them

Every agent-written skill carries its marker and a timestamp:

```yaml
---
name: pepebot-api-key-rotation
description: Open when rotating or replacing the API key for pepebot.
created_by: agent
created_at: 2026-10-16T07:40:16Z
---
```

Ask the bot (`skill_manage` with `list`), or on disk:

```sh
grep -rl "created_by: agent" ~/.pepebot/workspace/skills/
rm -rf ~/.pepebot/workspace/skills/<name>     # removing one is a directory delete
```

## Turning it off

```json
{ "skills": { "agent_managed": false } }
```

Unset means on. That unregisters the tool and stops the review from proposing
skills; existing ones stay where they are and keep working.

## One thing to know about one-shot runs

`pepebot agent -m "..."` used to exit the moment it printed the reply, killing
the review goroutine mid-flight — a one-shot run never learned anything while
the long-running gateway learned fine. The CLI now waits for background work
before exiting (90s cap), so both paths behave the same.
