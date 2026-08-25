# Plan: client-declared tools in Live sessions

## The question

Can an app that connects to `/v1/live` bring its own tools, register them for the
session, and execute them on its own device?

Yes. Nothing in the protocol prevents it, and the proxy is already in the right place
to route the calls. The work is in four parts: declare, merge, route, answer.

## Why it is worth doing

Pepebot's tool registry runs on the *gateway host*. A phone, a rover or a browser tab
has capabilities that host does not: camera, GPS, screen, local files, motors, whatever
the app wraps. Today those are unreachable — the model can only use what the gateway
can do. With client tools, the same voice session can say "take a photo and tell me
what you see" and the photo comes from the device that is actually there.

## How it works

```
app                     pepebot                      realtime server
 │  setup{tools:[...]}     │                              │
 ├────────────────────────►│  session.update              │
 │                         │  (agent tools + app tools)   │
 │                         ├─────────────────────────────►│
 │                         │                              │
 │                         │   function_call take_photo   │
 │                         │◄─────────────────────────────┤
 │  tool_call take_photo   │  (routed: app tool)          │
 │◄────────────────────────┤                              │
 │  tool_result {...}      │                              │
 ├────────────────────────►│  function_call_output        │
 │                         ├─────────────────────────────►│
 │                         │  response.create             │
 │                         ├─────────────────────────────►│
```

A gateway tool takes the path it already takes; only the routing decision is new.

## 1. Declare

`SetupConfig` gains a `tools` field — the same flat shape the Realtime API uses, so an
app that already talks to OpenAI can pass its definitions through unchanged:

```json
{
  "setup": {
    "provider": "realtime",
    "tools": [
      {
        "name": "take_photo",
        "description": "Take a photo with the device camera and describe what is in it.",
        "parameters": {"type": "object", "properties": {
          "camera": {"type": "string", "enum": ["front", "back"]}}}
      }
    ]
  }
}
```

Touches `SetupConfig` in `pkg/live/live.go`.

## 2. Merge

`buildRealtimeSessionUpdate` already flattens the agent's tools; client tools are
appended to that list.

**Names are namespaced by app.** A client declares `take_photo` and sets
`setup.app: "rover"`; the model sees `rover-take_photo`, and the client is asked for it
by the bare name it declared. Gateway tools are all `snake_case`, so joining with a
hyphen makes client tools structurally distinct and impossible to confuse — and the
join is still checked against the gateway's names, because nothing stops a future
gateway tool from containing a hyphen.

Client tool names are also validated (`^[a-zA-Z0-9_-]{1,64}$`) before they reach the
upstream session, so a malformed definition fails fast at setup rather than corrupting
the session.

## 3. Route

`handleRealtimeToolCall` currently executes everything through `ls.tools.ExecuteTool`.
It gains a lookup first:

- name in the session's client-tool set → forward to the app, wait for its result
- otherwise → execute on the gateway, exactly as now

The session needs a pending-call table: `map[callID]chan string`, guarded by a mutex,
so a result arriving on the client→upstream pump can be handed to the goroutine that is
waiting for it.

## 4. Answer

Two new frames, both on the existing socket:

**pepebot → app**
```json
{"type": "tool_call", "call_id": "call_abc", "name": "take_photo",
 "arguments": {"camera": "back"}}
```

**app → pepebot**
```json
{"type": "tool_result", "call_id": "call_abc", "output": "a cat on a keyboard"}
{"type": "tool_result", "call_id": "call_abc", "error": "camera permission denied"}
```

`tool_result` is intercepted by the client→upstream pump and **not** forwarded
upstream — the upstream only ever sees the `function_call_output` pepebot builds from
it.

An `error` field comes back to the model as `Error: <message>`, the same shape a failed
gateway tool produces, so the model can recover by saying so instead of stalling.

### Timeouts

A device can be slow, backgrounded, or simply gone. Each client call gets a deadline
(default 30s, `setup.client_tool_timeout_ms` to override, capped at the 90s the gateway
tools already use). On expiry pepebot answers upstream with
`Error: client tool timed out` and drops the pending entry, so a silent app cannot wedge
the turn forever.

## What this does not cover

- **Gemini/Vertex sessions.** The same routing applies, but the frames differ
  (`toolCall`/`toolResponse` instead of `function_call`). Realtime first; Gemini is a
  follow-up once the routing has proven itself.
- **Streaming or binary results.** `output` is a string. An app that wants to return an
  image should send it as a data URL, or upload it and return a reference.
- **Trust.** A client declares what it can do and executes it itself, so pepebot is not
  granting the app anything it did not already have — but the *model* now sees
  app-supplied tool descriptions. A prompt-injection-flavoured description
  ("ignore previous instructions…") reaches the session instructions. Descriptions are
  passed through as data, and the risk is the app's own, not the gateway's; worth a
  note in the docs rather than a filter.

## Files

| | |
|---|---|
| `pkg/live/live.go` | `SetupConfig.Tools`, validation, pending-call table, routing, the two frames |
| `pkg/live/live_test.go` | name validation, collision rejection, routing choice, timeout path |
| `examples/live-api/paniki.html` | declare one browser tool (`read_clipboard` or `geolocate`) as a worked example |
| `docs/live-api.md` | the two frames and the collision rule |

## Check

The routing and timeout logic gets a test with a fake client: declare a tool, feed a
`function_call` in, assert a `tool_call` frame comes out, answer it, assert the
`function_call_output` sent upstream carries that output. Then the same with no answer,
asserting the timeout message. The mock Realtime server used for the tool-loop work
already provides the upstream half.

## Sequence

1. Declare + merge + validation (no routing yet — tools appear to the model, calls
   still go to the gateway and fail with "unknown tool", which is honest)
2. Routing + the two frames + pending table
3. Timeout
4. Browser example + docs
