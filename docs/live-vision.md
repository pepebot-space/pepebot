# Images and video in a Live session

Everything here is measured against the Paniki Realtime server
(`ws://100.104.36.93:8000/v1/realtime`, model `qwen/qwen3.8-27b`) through Pepebot's
`/v1/live` proxy. Other Realtime servers differ; the probe commands at the bottom tell
you what yours accepts.

## Short answer

| | |
|---|---|
| Send an image to the model | **Yes** — four block shapes are read |
| Send a stream of frames ("video") | **Yes, as items.** No streaming event, and none needed: only the newest image is kept, so frames are consumed |
| Show the model two images at once | **Only within one item** — retention keeps just the latest |
| Have the agent send an image back | **No** — the protocol has no image output; use a client tool |
| Pepebot changes needed | **None.** The proxy forwards client frames verbatim |

## Sending an image

The OpenAI Realtime shape:

```json
{"type": "conversation.item.create",
 "item": {"type": "message", "role": "user", "content": [
   {"type": "input_image", "image_url": "data:image/png;base64,iVBORw0KG..."},
   {"type": "input_text", "text": "Ini warna apa?"}
 ]}}
```

Then `{"type":"response.create"}` as usual.

**Four shapes are read**, so an integrator coming from another API does not have to
guess which dialect this one speaks:

| shape | |
|-------|--|
| `input_image` + `image_url` (a `data:` URL, or `{url}`) | ✅ |
| `image_url` + `image_url.url` | ✅ |
| `image` + `source.base64` (Anthropic style) | ✅ |
| `input_image` + `data` (+ `mime_type`) | ✅ |

And the two ways to get it wrong are errors rather than silence:

| | |
|-------|--|
| an image part with nothing readable in it | `unreadable_image` |
| a remote `http(s)` URL | `remote_image` — refused, not fetched |

The remote refusal is deliberate: a server that fetches URLs on a client's behalf is an
SSRF tool. Encode the bytes yourself.

> Earlier versions of this document said two of those four shapes were silently ignored.
> That was true when measured and was fixed server-side; `_item_image()` had been
> returning the same `None` for "no image here" and "an image I could not read", which is
> why one produced a wrong answer instead of an error.

## Video: repeated frames, and they are consumed

There is no streaming event, and there does not need to be one. **Only the newest image
is retained** — a new frame replaces the previous one — so frames sent as items are
consumed rather than accumulated, which is the property a frame channel would have given.

Measured: thirty red frames followed by one blue, then "what colour is the last image?"

```
30 red frames + 1 blue → "Biru"     45s
1 frame                → "Merah"    43s
```

Thirty-one frames cost the same as one. Send a frame with no `response.create` and
nothing is generated — it just replaces the current image. That is what makes a camera
feed workable: push frames as they arrive, ask only when you want an answer.

The server says so itself, so there is no need to guess:

```
GET /v1/realtime → images: {
  "retention": "only the newest image is kept; a new one replaces it",
  "camera_feed": "send frames as items -- they are consumed, not accumulated",
  "video_channel": false,
  "max_bytes": 8388608
}
```

### The tradeoff: one image at a time

Retention is also a limit. The model can only ever see the most recent image, so
"compare these two" does not work:

```
frame (red), frame (green), "name the colour of the first and second image"
→ "Saya hanya bisa melihat satu gambar, warnanya hijau."
```

For a feed that is exactly right. For before-and-after, or two documents side by side,
put both images in **one** `conversation.item.create` content array instead of two items,
or ask about each one as it arrives and let the model keep the answer in the
conversation rather than the image.

### Practical shape

- Downscale first. 640×480 JPEG is plenty for "what am I looking at"; the cap is 8MB per
  image but a 4K PNG buys no extra understanding
- JPEG is accepted, which matters because camera frames are JPEG
- Frames are cheap now, but the *turns* are not: a question per frame is a full
  response each time. Push frames freely, ask sparingly

For a genuine video channel — frames outside the conversation entirely — Vertex/Gemini
Live has one (`realtimeInput` with inline video, used by
`examples/live-api/index-video.html`). The Realtime protocol does not, and for a
latest-frame feed it does not need one.

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
