# Providers and model references

One model, written one way, kept in one place.

## The canonical form: `<provider>:<model_id>`

```json
{ "agents": { "defaults": { "model": "maiarouter:zai/glm-4.5v" } } }
```

Everything left of the **first** colon names the endpoint; everything right of it
is the model id sent upstream, untouched.

```
maiarouter:zai/glm-4.5v   → endpoint maiarouter, model "zai/glm-4.5v"
openai:gpt-4.1            → endpoint openai,     model "gpt-4.1"
custom:ollama/qwen3       → custom endpoint "ollama", model "qwen3"
```

The split is on the colon, never a slash, which is what makes it unambiguous: a
vendor namespace inside the model id (`zai/glm-4.5v`, `anthropic/claude-3.5-sonnet`)
uses slashes and survives intact. Guessing where a provider ended and a model
began is what once sent `maiarouter/zai/glm-5.3-flash` to an API that answered
*"no healthy deployments for this model"*.

### If `provider` is set separately, the model is just the id

```json
{ "agents": { "defaults": { "provider": "maiarouter", "model": "zai/glm-4.5v" } } }
```

Both spellings work. When both are present and disagree, the colon wins — it is
the more specific of the two, and someone who writes it means it.

## Custom endpoints

Any OpenAI-compatible endpoint, named in config rather than added to the code:

```json
{
  "providers": {
    "custom": {
      "9router": { "api_base": "http://localhost:20128/v1", "api_key": "..." },
      "litellm": { "api_base": "https://llm.example.com/v1", "api_key": "..." },
      "ollama":  { "api_base": "http://minipepe:11434/v1",   "api_key": "-", "model": "qwen3" }
    }
  }
}
```

Address them as `custom:<name>/<model>` — `custom:9router/kr/claude-sonnet-4.5`,
`custom:litellm/zai/glm-4.6`. With no model, `custom:ollama` uses that entry's
own `model`.

A custom endpoint's namespace is its own: the model id goes out exactly as
written, with no prefix stripping. A gateway may legitimately want
`kr/claude-sonnet-4.5`, and pepebot has no business second-guessing a name it
does not own.

## Where the model lives

**The agent registry** — `workspace/agents/registry.json` — is the source of
truth. `agents.defaults.model` in `config.json` seeds a fresh install and is the
fallback for agents that name no model of their own.

This used to be a trap: the registry silently won, so editing `config.json`
looked like it did nothing, and a stale registry entry kept going out on the
wire. Now, at startup:

| Situation | What happens |
|---|---|
| Registry has no default agent | Seeded from `config.json`, written in canonical form |
| Both agree | Nothing, except a legacy spelling is rewritten canonically |
| They disagree, terminal present | You are asked which one to use |
| They disagree, no terminal (systemd) | The registry wins, and both values are logged as a warning |

The question is only asked where someone can answer it. A gateway under systemd
must never block on a prompt nobody can see — but it still records the
disagreement, so the mismatch is discoverable instead of silent.

```
⚠️  Two different models are configured:
    agent registry : custom:mr/zai/glm-4.5v   (this is what runs today)
    config.json    : maiarouter:zai/glm-4.5v

Use the config.json model and overwrite the registry? [y/N]:
```

Answering `y` rewrites the registry and persists it, so the question is asked
once, not on every run.
