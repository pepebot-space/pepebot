import asyncio
import base64
import json
import signal
import sys
from typing import Optional

import pyaudio
import websockets


URL = "ws://localhost:18790/v1/live"
INPUT_SAMPLE_RATE = 16000
OUTPUT_SAMPLE_RATE = 24000
INPUT_MIME = "audio/pcm;rate=16000"

CHANNELS = 1
SAMPLE_WIDTH = 2
FORMAT = pyaudio.paInt16
INPUT_CHUNK = 2048


def decode_base64_audio(base64_string: str) -> bytes:
    s = (base64_string or "").replace("-", "+").replace("_", "/")
    while len(s) % 4 != 0:
        s += "="
    return base64.b64decode(s)


def try_parse_json(data) -> Optional[dict]:
    try:
        if isinstance(data, bytes):
            return json.loads(data.decode("utf-8", errors="ignore"))
        return json.loads(data)
    except Exception:
        return None


async def main() -> None:
    stop_event = asyncio.Event()
    loop = asyncio.get_running_loop()

    def _stop(*_):
        stop_event.set()

    for sig in (signal.SIGINT, signal.SIGTERM):
        try:
            loop.add_signal_handler(sig, _stop)
        except NotImplementedError:
            pass

    signal.signal(signal.SIGINT, lambda *_: stop_event.set())

    p = pyaudio.PyAudio()
    mic_stream = p.open(
        format=FORMAT,
        channels=CHANNELS,
        rate=INPUT_SAMPLE_RATE,
        input=True,
        frames_per_buffer=INPUT_CHUNK,
    )
    speaker_stream = p.open(
        format=FORMAT,
        channels=CHANNELS,
        rate=OUTPUT_SAMPLE_RATE,
        output=True,
    )

    print(f"Connecting to {URL}...")
    async with websockets.connect(URL, max_size=20 * 1024 * 1024) as ws:
        await ws.send(
            json.dumps(
                {
                    "setup": {
                        "provider": "vertex",
                        "model": "gemini-live-2.5-flash-native-audio",
                        "agent": "default",
                        "enable_tools": True,
                    }
                }
            )
        )

        async def sender() -> None:
            while not stop_event.is_set():
                pcm = await asyncio.to_thread(
                    mic_stream.read,
                    INPUT_CHUNK,
                    False,
                )
                b64_data = base64.b64encode(pcm).decode("utf-8")
                await ws.send(
                    json.dumps(
                        {
                            "realtimeInput": {
                                "mediaChunks": [
                                    {"mimeType": INPUT_MIME, "data": b64_data}
                                ]
                            }
                        }
                    )
                )

        async def receiver() -> None:
            while not stop_event.is_set():
                msg = await ws.recv()

                if isinstance(msg, bytes):
                    parsed_bin = try_parse_json(msg)
                    if isinstance(parsed_bin, dict):
                        msg = parsed_bin
                    else:
                        if len(msg) >= 2 and len(msg) % 2 == 0:
                            await asyncio.to_thread(speaker_stream.write, msg)
                        continue
                else:
                    msg = try_parse_json(msg)

                if not isinstance(msg, dict):
                    continue

                if msg.get("status") == "connected":
                    print(
                        f"Proxy connected: {msg.get('provider')} -> {msg.get('model')}"
                    )
                    continue

                if "setupComplete" in msg:
                    print("Live session ready")
                    continue

                if msg.get("error"):
                    print(f"Error: {msg['error']}")
                    continue

                server_content = msg.get("serverContent")
                if not isinstance(server_content, dict):
                    continue

                model_turn = server_content.get("modelTurn")
                if isinstance(model_turn, dict):
                    parts = model_turn.get("parts", [])
                    for part in parts:
                        if isinstance(part, dict) and part.get("text"):
                            print(f"Bot: {part['text']}")
                        inline = (
                            part.get("inlineData") if isinstance(part, dict) else None
                        )
                        if isinstance(inline, dict) and inline.get("data"):
                            audio = decode_base64_audio(inline["data"])
                            if len(audio) >= 2 and len(audio) % 2 == 0:
                                await asyncio.to_thread(speaker_stream.write, audio)

        async def input_worker() -> None:
            print("Mic auto ON. Command: /quit | or type message")
            while not stop_event.is_set():
                text = (await asyncio.to_thread(input, "You> ")).strip()
                if not text:
                    continue
                if text == "/quit":
                    stop_event.set()
                    return

                await ws.send(
                    json.dumps(
                        {
                            "clientContent": {
                                "turns": [{"role": "user", "parts": [{"text": text}]}],
                                "turnComplete": True,
                            }
                        }
                    )
                )

        tasks = [
            asyncio.create_task(sender()),
            asyncio.create_task(receiver()),
            asyncio.create_task(input_worker()),
        ]

        try:
            await stop_event.wait()
        finally:
            for task in tasks:
                task.cancel()
            await asyncio.gather(*tasks, return_exceptions=True)

    try:
        mic_stream.stop_stream()
        mic_stream.close()
    except Exception:
        pass
    try:
        speaker_stream.stop_stream()
        speaker_stream.close()
    except Exception:
        pass
    p.terminate()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        sys.exit(0)
