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

Open <http://127.0.0.1:8899/index-openai.html>.

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
agent's tool definitions to the upstream server in a `session.update`, then executes
any `function_call` it gets back and returns a `function_call_output`, exactly as it
does for Vertex/Gemini live sessions.

The upstream server has to allow it. The Paniki Realtime server currently does not —
`session.update` with tools is answered with:

```
{"type":"error","error":{"type":"invalid_request_error","code":"unsupported_field",
 "message":"session.tools is fixed by the server."}}
```

The rest of the same `session.update` still applies (the persona in `instructions`
does land), and those two error frames are relayed to your client. Once the server
accepts `session.tools`, tool calling works with no client change.

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
