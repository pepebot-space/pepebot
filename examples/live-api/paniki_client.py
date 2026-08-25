#!/usr/bin/env python3
"""Talk to Pepebot's Live API with your microphone.

    python paniki_client.py                          # ws://127.0.0.1:18790/v1/live
    python paniki_client.py ws://host:18790/v1/live
    MIC="usb audio" python paniki_client.py          # pick an input device
    LANG=en-US VOICE=nenden python paniki_client.py

Speak; the agent answers and you can cut it off mid-sentence. Ctrl-C to quit.

This connects to Pepebot, not to the Realtime server directly, so the agent's
persona, skills and tools are all in play — ask it to read a file and it will.

Requires: websockets, sounddevice, numpy
"""

import asyncio
import base64
import json
import os
import queue
import sys
import threading
import urllib.request

import numpy as np
import sounddevice as sd
import websockets

sys.stdout.reconfigure(line_buffering=True)

GATEWAY = os.getenv("PEPEBOT_LIVE_URL", "ws://127.0.0.1:18790/v1/live")
PANIKI = os.getenv("PANIKI_URL", "http://100.104.36.93:8000")
LANGUAGE = os.getenv("LANG_CODE", "id-ID")
VOICE = os.getenv("VOICE", "")
# One session key per client by default, so separate clients keep separate histories.
SESSION = os.getenv("SESSION") or os.getenv("SESSION_KEY") or "py-session"

RATE = 24000        # the Realtime API speaks pcm16 mono at this rate
BLOCK = 480         # 20ms: low latency, and small enough for any frame-based VAD


class Playback:
    """Speaker buffer that can be dropped on the floor when the user cuts in."""

    def __init__(self):
        self._buffer = bytearray()
        self._lock = threading.Lock()
        self.stream = sd.RawOutputStream(
            samplerate=RATE, channels=1, dtype="int16",
            blocksize=BLOCK, callback=self._fill,
        )

    def _fill(self, outdata, frames, _time, _status):
        want = frames * 2
        with self._lock:
            chunk = bytes(self._buffer[:want])
            del self._buffer[:want]
        outdata[: len(chunk)] = chunk
        if len(chunk) < want:
            outdata[len(chunk):] = b"\x00" * (want - len(chunk))

    def add(self, pcm: bytes):
        with self._lock:
            self._buffer.extend(pcm)

    def clear(self):
        with self._lock:
            self._buffer.clear()


def discover(path: str, key: str) -> list:
    """Read /v1/models or /v1/voices so nothing here goes stale on a redeploy."""
    try:
        with urllib.request.urlopen(f"{PANIKI.rstrip('/')}{path}", timeout=8) as res:
            body = json.load(res)
        items = [row["id"] for row in body.get("data", [])]
        print(f"-- {key}: {', '.join(items[:6])}{' …' if len(items) > 6 else ''}")
        return items
    except Exception as err:
        print(f"-- could not read {path} ({err}); relying on the gateway's config")
        return []


def check_mic(device) -> None:
    """Refuse to sit there silently when the input delivers nothing.

    macOS answers a denied microphone with zeroed buffers rather than an error, so a
    permission problem is indistinguishable from a quiet room unless you look.
    """
    name = sd.query_devices(device, kind="input")["name"]
    blocks = []
    with sd.InputStream(samplerate=RATE, channels=1, dtype="int16", device=device,
                        callback=lambda indata, *_: blocks.append(indata.copy())):
        sd.sleep(600)
    peak = int(np.abs(np.concatenate(blocks)).max()) if blocks else 0
    print(f"-- mic: {name} (peak {peak})")
    if peak == 0:
        print(
            "\n!! That input is delivering pure silence, so nothing will be heard.\n"
            "   - Run this from a terminal app that holds the microphone grant.\n"
            "   - System Settings > Privacy & Security > Microphone: toggle it off and on.\n"
            "   - Wrong device? List them with:\n"
            "       python -c \"import sounddevice;print(sounddevice.query_devices())\"",
            file=sys.stderr,
        )
        sys.exit(1)


async def send_mic(ws, mic: queue.Queue):
    """Ship microphone blocks as they arrive off the audio thread."""
    sent, peak = 0, 0
    while True:
        pcm = await asyncio.to_thread(mic.get)
        peak = max(peak, int(np.abs(np.frombuffer(pcm, dtype=np.int16)).max()))
        sent += 1
        if sent % 50 == 0:  # roughly once a second
            bar = "#" * min(20, peak * 20 // 32768)
            print(f"\r  mic {peak:>5} |{bar:<20}|", end="", flush=True)
            peak = 0
        await ws.send(json.dumps({
            "type": "input_audio_buffer.append",
            "audio": base64.b64encode(pcm).decode(),
        }))


async def receive(ws, playback: Playback):
    async for message in ws:
        event = json.loads(message)

        # Pepebot's own proxy frames carry a status rather than a type.
        if event.get("status") == "connected":
            print(f"\n-- proxy: {event['provider']} -> {event['model']}")
            continue
        if event.get("status") == "reconnected":
            print("\n-- upstream reconnected; conversation state was reset")
            playback.clear()
            continue

        kind = event.get("type")

        if kind == "response.audio.delta":
            playback.add(base64.b64decode(event["delta"]))
        elif kind == "input_audio_buffer.speech_started":
            playback.clear()          # barge-in: drop what the bot had not said yet
            print("\n[listening]", end="", flush=True)
        elif kind == "conversation.item.input_audio_transcription.completed":
            print(f"\nyou: {event['transcript'].strip()}")
        elif kind == "response.audio_transcript.delta":
            print(event["delta"], end="", flush=True)
        elif kind == "response.output_item.done":
            item = event.get("item") or {}
            if item.get("type") == "function_call":
                print(f"\n[tool] {item.get('name')}({item.get('arguments', '')})"
                      " — Pepebot is running it")
        elif kind == "response.done":
            if (event.get("response") or {}).get("status") != "completed":
                playback.clear()
            print()
        elif kind == "error":
            print(f"\n[server error] {event.get('error')}")


async def main():
    url = sys.argv[1] if len(sys.argv) > 1 else GATEWAY
    device = os.getenv("MIC") or None
    if device and device.isdigit():
        device = int(device)

    models = discover("/v1/models", "models")
    discover("/v1/voices", "voices")
    check_mic(device)

    mic: queue.Queue = queue.Queue()

    def on_mic(indata, _frames, _time, _status):
        mic.put(bytes(indata))

    print(f'-- session "{SESSION}"')
    print(f"-- connecting to {url}")
    async with websockets.connect(url, max_size=None) as ws:
        setup = {
            "provider": "realtime",
            "agent": os.getenv("AGENT", "default"),
            "session_key": SESSION,
            "enable_tools": True,
            "language": LANGUAGE,
        }
        if models:
            setup["model"] = models[0]
        await ws.send(json.dumps({"setup": setup}))

        # Voice is normally set once in live.realtime_session; this is the per-session
        # override. Everything else (persona, skills, tools) Pepebot already sent.
        if VOICE:
            await ws.send(json.dumps({"type": "session.update", "session": {"voice": VOICE}}))

        playback = Playback()
        recorder = sd.RawInputStream(samplerate=RATE, channels=1, dtype="int16",
                                     blocksize=BLOCK, device=device, callback=on_mic)

        print("-- speak now (ctrl-c to quit)")
        with recorder, playback.stream:
            sender = asyncio.create_task(send_mic(ws, mic))
            try:
                await receive(ws, playback)
            finally:
                sender.cancel()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\nbye")
