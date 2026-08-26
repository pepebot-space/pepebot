# Images and video in a Live session

Everything here is measured against the Paniki Realtime server
(`ws://100.104.36.93:8000/v1/realtime`, model `qwen/qwen3.8-27b`) through Pepebot's
`/v1/live` proxy. Other Realtime servers differ; the probe commands at the bottom tell
you what yours accepts.

## Short answer

| | |
|---|---|
| Send an image to the model | **Yes** — one shape works, see below |
| Send a stream of frames ("video") | **Yes, as repeated items.** There is no streaming event |
| Have the agent send an image back | **No** — the protocol has no image output; use a client tool |
| Pepebot changes needed | **None.** The proxy forwards client frames verbatim |

## Sending an image

One shape works, and it is the OpenAI Realtime one:

```json
{"type": "conversation.item.create",
 "item": {"type": "message", "role": "user", "content": [
   {"type": "input_image", "image_url": "data:image/png;base64,iVBORw0KG..."},
   {"type": "input_text", "text": "Ini warna apa?"}
 ]}}
```

Then `{"type":"response.create"}` as usual.

**Two other shapes are accepted and silently ignored.** This is the failure mode to
watch for — no error comes back, the model simply answers as if it saw nothing:

| shape | result |
|-------|--------|
| `{"type":"input_image","image_url":"data:..."}` | **"Red"** — correct |
| `{"type":"image","source":{"type":"base64","media_type":...,"data":...}}` | "Unknown" — block dropped |
| `{"type":"input_image","data":"...","media_type":"image/png"}` | "Unknown" — block dropped |

If the model keeps answering vaguely about an image you are certain you sent, check the
block shape before anything else. An Anthropic-style `source` object is the easy
mistake to make.

## Video: repeated frames, not a stream

There is no streaming event. Every plausible name is rejected:

```
input_image_buffer.append      → unknown_type
input_video_buffer.append      → unknown_type
realtime_input / realtimeInput → unknown_type
input_media_buffer.append      → unknown_type
conversation.item.append_image → unknown_type
```

So a video feed is a sequence of image items. The model reads the most recent one:

```
frame (red)   → "what colour is the most recent image?" → Red
frame (green) → "and now?"                              → Green
```

Send a frame with **no** `response.create` and nothing is generated — it just becomes
context the next turn sees. That is what makes a camera feed workable: push frames as
they come, and only ask a question when you want an answer.

### Pace it, or it gets expensive

Every frame stays in the conversation as an item, and the upstream server charges for
the tokens of each one on every subsequent turn. This is nothing like an audio stream,
where frames are consumed and dropped.

Practical shape for a camera:

- **One frame every second or two**, not 30 a second
- Better: **one frame on demand** — a client tool the agent calls when it wants to look
  (see below), rather than a constant feed
- Downscale first. 640×480 JPEG is plenty for "what am I looking at"; a 4K PNG is
  megabytes of base64 for no extra understanding
- If a session runs long, reconnect periodically: the conversation restarts and the
  accumulated frames go with it

For a genuine continuous video stream, Vertex/Gemini Live is the provider that has one
(`realtimeInput` with inline video, which `examples/live-api/index-video.html` uses).
The Realtime protocol does not.

## Through Pepebot

No changes are needed and none were made. Pepebot forwards client→upstream frames
verbatim — the only exception is `tool_result`, which it consumes. So a client sends
image items straight through:

```
proxy: realtime -> qwen/qwen3.8-27b
item diterima upstream
REPLY: Red
```

That was measured through the deployed gateway on `sj`, not a mock.

## Getting an image back out

The agent cannot show you a picture over a live session: the Realtime protocol produces
text and audio, nothing else. Two ways around it, and they are different things:

**A client tool** — the right answer for live. The app declares a tool the agent can
call, and the app displays the image itself:

```js
// in the browser client
{
  name: 'show_image',
  description: 'Display an image to the user on their screen.',
  parameters: { type: 'object', properties: {
    url: { type: 'string', description: 'Image URL or data URL' } }, required: ['url'] },
}
```

The agent calls `web-show_image`, the page renders an `<img>`, and the picture appears
on the device that is actually in front of the user. See
`examples/live-api/README-realtime.md` for how client tools work.

**The `send_image` tool** — not this. It publishes to a *chat channel*
(Telegram/Discord/WhatsApp) through the message bus and needs a `channel` and `chat_id`.
It has nothing to do with the live socket, and on a gateway with no channels enabled it
simply fails.

A nice pairing once both are in place: `adb_screenshot` (a gateway tool — the file lands
on the gateway host) followed by `show_image` (a client tool — it appears on your
screen).

## Probing a different server

Whether a given Realtime server does vision, and in which shape, takes about a minute
to find out. Send one image item per shape and see which one it actually reads:

```python
import base64, json, websockets, asyncio

IMG = base64.b64encode(open("red.png", "rb").read()).decode()

SHAPES = {
    "input_image + image_url": {"type": "input_image", "image_url": f"data:image/png;base64,{IMG}"},
    "image + source":          {"type": "image", "source": {"type": "base64",
                                "media_type": "image/png", "data": IMG}},
}

async def probe(url, block):
    async with websockets.connect(url) as ws:
        await ws.send(json.dumps({"type": "conversation.item.create", "item": {
            "type": "message", "role": "user",
            "content": [block, {"type": "input_text", "text": "What colour? One word."}]}}))
        await ws.send(json.dumps({"type": "response.create"}))
        async for raw in ws:
            e = json.loads(raw)
            if e.get("type") == "error":
                return "ERROR: " + e["error"].get("message", "")
            if e.get("type") == "response.audio_transcript.done":
                return e.get("transcript", "").strip()

for label, block in SHAPES.items():
    print(label, "->", asyncio.run(probe("ws://host:8000/v1/realtime", block)))
```

Use a solid-colour image and ask for the colour: a server that ignores the block answers
plausibly and wrongly, which a "did it error?" check would miss entirely.
