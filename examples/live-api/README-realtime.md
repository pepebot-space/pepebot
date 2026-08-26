# Trying a custom OpenAI Realtime endpoint

`index-openai.html` drives Pepebot's `/v1/live` proxy using the OpenAI Realtime
event protocol. It works against any upstream that speaks that protocol —
OpenAI itself, or a self-hosted server.

## 1. Point Pepebot at the endpoint

`~/.pepebot/config.json`:

```json
{
  "providers": {
    "realtime": { "api_base": "http://127.0.0.1:8000/v1", "api_key": "" }
  },
  "live": { "enabled": true, "provider": "realtime", "model": "your-model" }
}
```

`api_key` may be empty — no `Authorization` header is sent when it is.
`http://` becomes `ws://`, `https://` becomes `wss://`, and the provider connects
to `<api_base>/realtime?model=<model>`.

Leaving `live.provider` on another provider is fine: the demo sends the provider
per session, so nothing else has to change.

## 2. Run the gateway and serve the page

```bash
pepebot gateway                                   # listens on 127.0.0.1:18790
python3 -m http.server 8899 --bind 127.0.0.1      # from examples/live-api/
```

Open **<http://127.0.0.1:8899/paniki.html>** — the page written for this server.
(`index-openai.html` is the generic OpenAI-Realtime demo; it sends fields Paniki
fixes server-side and will log errors for them.)

`paniki.html` reads the model list from `/v1/models` on load, so it never goes stale
when the server is redeployed with a different model. It sends only fields Paniki
accepts, leaves `instructions` alone so Pepebot's agent persona and tools survive, and
logs tool calls as they happen. "Discover voices" harvests the available voice ids from
the server's own `unknown_voice` error, since there is no voices endpoint.

The page asks for the gateway URL separately (default
`ws://127.0.0.1:18790/v1/live`) because it is usually served from a different
port than the gateway.

## 3. Try it

- **Connect** — expect `Proxy connected: <provider> -> <model>` then
  `Realtime session created`.
- **Send 2s test audio** — streams a synthetic tone as pcm16, no microphone
  needed. Keep the tab in the foreground; browsers throttle timers in background
  tabs, which stalls the paced sender.
- **Start Mic** — real capture at 24kHz, committed on **Stop Mic**.

Audio is pcm16 mono at 24kHz in both directions.

## Agent tools

Tools are on by default (`setup.enable_tools`, defaults to true). Pepebot sends the
agent's tool definitions and persona to the upstream server in a `session.update`,
executes any `function_call` the model makes, and returns a `function_call_output` —
the same behaviour as Vertex/Gemini live sessions.

Verified against the Paniki Realtime server, asking it to list a directory:

```
live: Sent upstream session.update {provider=realtime, tools=24, prompt_chars=162, prompt_source=config}
<< response.output_item.done -> function_call list_dir {"path":"."}
live: Executed live tool call {tool=list_dir, failed=false, chars=42}
<< response.audio_transcript.done -> "Here's the current directory: agents, ok.txt, workflows"
```

The second turn is spoken as well — 130 audio deltas on the follow-up.

### Tuning the upstream session

Anything else the upstream server allows goes in `live.realtime_session` and is merged
into the same `session.update`:

```json
"live": {
  "provider": "realtime",
  "realtime_session": { "voice": "JV-00027", "temperature": 0.2, "max_response_output_tokens": 200 }
}
```

Ask for a voice the server does not have and it says so:
`unknown_voice — No voice 'echo'. Available: JV-00027, JV-00264, JV-00658, ...`.
Pepebot's own `instructions` and `tools` take precedence over the same keys.

## Clients

| | |
|---|---|
| `paniki.html` | Browser: mic, barge-in, tool-call trace, model/voice pickers. Serve it and open it. |
| `nodejs/paniki-client.js` | Terminal, full duplex: mic via ffmpeg, playback via `speaker`. Type at it too. |
| `paniki_client.py` | Terminal, full duplex: mic and playback via `sounddevice`. |
| `nodejs/paniki-toolcall.js` | Terminal, text: declares client tools this machine runs. Only needs `ws`. |
| `paniki_toolcall.py` | Same in Python. Only needs `websockets`. |

```bash
cd nodejs && npm install && node paniki-client.js     # needs ffmpeg on PATH
NO_MIC=1 node paniki-client.js                        # type only, replies saved as WAV
MIC=":1" node paniki-client.js                        # another input device

pip install websockets sounddevice numpy && python3 paniki_client.py
LANG_CODE=en-US VOICE=nenden python3 paniki_client.py
```

The Node client shells out to `ffmpeg` for capture rather than using the `mic`
package, which needs sox or arecord installed separately. Without ffmpeg it falls
back to text-only; without `speaker` it writes each reply to `reply-N.wav`.

List input devices: `ffmpeg -f avfoundation -list_devices true -i ""` on macOS,
`arecord -l` on Linux.

All three connect to Pepebot's `/v1/live`, not to the Realtime server directly, so the
agent's persona, skills and tools are in play. Each reads `/v1/models` and `/v1/voices`
from the server at startup, so nothing goes stale on a redeploy.

**If the level meter sits at zero**, the microphone is delivering silence. On macOS a
denied microphone returns zeroed buffers rather than an error, so it looks exactly like
a quiet room — both clients say so instead of waiting forever. Run them from a terminal
app that holds the grant, and check System Settings > Privacy & Security > Microphone.

## What the session carries

Every Realtime session's `instructions` are assembled by Pepebot in this order:

1. the persona — `setup.system_prompt`, else `live.system_prompt(_file)`, else the agent's `AGENTS.md`/`SOUL.md`/`IDENTITY.md`
2. the agent's skills (summary plus definitions) — attached independently, so setting a
   system prompt does not drop them
3. a speech directive: the reply is spoken, so no markdown, lists, tables or emoji
4. the reply language from `setup.language` or `live.language` (e.g. `id-ID`)

## Chat sessions

`setup.session_key` names the conversation. Turns are appended to the agent's session
history under that name, **and replayed back** into a new session with the same name —
so a conversation survives a reconnect, and a voice session and a text session on the
same key are one conversation.

Different names are different conversations, with separate memory:

```bash
SESSION=work node paniki-toolcall.js       # one conversation
SESSION=rumah node paniki-toolcall.js      # another, knows nothing of the first
```

The browser demo exposes it as a **Session** field, shown in the header, remembered per
browser — so two tabs with different names hold two separate conversations.

The last 20 turns are replayed; the upstream server charges for every token of what is
handed back, so this is a window, not the whole archive.

If the upstream connection drops, Pepebot rebuilds it (3 attempts, 1s/2s/4s backoff) and
replays all of the above, then sends the client `{"status":"reconnected"}`. Upstream
conversation state does not survive that — only the session configuration does.

## Images and video

A client can send images to the model with no Pepebot change — the proxy forwards client
frames verbatim. One block shape works (`input_image` + a data URL in `image_url`); the
Anthropic-style `source` object is accepted and silently ignored, which reads as the
model being vague rather than as an error.

There is no streaming event, so a video feed is a sequence of image items — and each one
stays in the conversation and is billed again every turn, so pace it.

Full detail, including how to probe a different server: **[docs/live-vision.md](../../docs/live-vision.md)**.

## Frame size

Small frames are still the better default — **20ms (480 samples, 960 bytes)** keeps
latency low and barge-in responsive — but they are no longer a hard requirement of
the Paniki Realtime server: appends of 100ms and 170ms now complete a turn, because
the server repacketizes internally.

Some other Realtime servers run a frame-based VAD that only accepts small frames and
stall silently on anything larger — no `speech_stopped`, no transcription, no error.
`index-openai.html` therefore repacketizes every audio path — mic and synthetic —
into 20ms frames via `sendAudioSamples`, which also solves the browser's
`ScriptProcessorNode` handing out 4096-sample (~170ms) buffers. Reuse that approach
in your own client rather than forwarding raw capture buffers.

Pacing matters more than size: stream frames in real time. Dumping a whole utterance
at once does not trigger the VAD.
