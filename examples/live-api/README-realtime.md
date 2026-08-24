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
