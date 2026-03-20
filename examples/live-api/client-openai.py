import asyncio
import base64
import json
import signal
import sys

import pyaudio
import websockets


URL = "ws://localhost:18790/v1/live"

INPUT_RATE = 24000
OUTPUT_RATE = 24000
CHANNELS = 1
SAMPLE_WIDTH = 2
FORMAT = pyaudio.paInt16
INPUT_CHUNK = 2048


def decode_base64_audio(base64_string: str) -> bytes:
    s = (base64_string or "").replace("-", "+").replace("_", "/")
    while len(s) % 4 != 0:
        s += "="
    return base64.b64decode(s)


def try_parse_json(data):
    try:
        if isinstance(data, bytes):
            return json.loads(data.decode("utf-8", errors="ignore"))
        return json.loads(data)
    except Exception:
        return None


async def main():
    print(f"Connecting to Pepebot at {URL}...")

    stop_event = asyncio.Event()
    loop = asyncio.get_running_loop()
    session_ready = False

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
        rate=INPUT_RATE,
        input=True,
        frames_per_buffer=INPUT_CHUNK,
    )
    speaker_stream = p.open(
        format=FORMAT,
        channels=CHANNELS,
        rate=OUTPUT_RATE,
        output=True,
    )

    try:
        async with websockets.connect(URL, max_size=20 * 1024 * 1024) as ws:
            await ws.send(
                json.dumps(
                    {
                        "setup": {
                            "provider": "openai",
                            "model": "gpt-4o-realtime-preview",
                            "agent": "default",
                            "enable_tools": True,
                        }
                    }
                )
            )

            async def send_text(text: str):
                await ws.send(
                    json.dumps(
                        {
                            "type": "conversation.item.create",
                            "item": {
                                "type": "message",
                                "role": "user",
                                "content": [{"type": "input_text", "text": text}],
                            },
                        }
                    )
                )
                await ws.send(json.dumps({"type": "response.create"}))

            async def sender_audio():
                while not stop_event.is_set():
                    if not session_ready:
                        await asyncio.sleep(0.05)
                        continue

                    pcm = await asyncio.to_thread(
                        mic_stream.read,
                        INPUT_CHUNK,
                        False,
                    )
                    b64_data = base64.b64encode(pcm).decode("utf-8")
                    await ws.send(
                        json.dumps(
                            {
                                "type": "input_audio_buffer.append",
                                "audio": b64_data,
                            }
                        )
                    )

            async def receiver():
                nonlocal session_ready
                while not stop_event.is_set():
                    msg = await ws.recv()
                    parsed = try_parse_json(msg)
                    if not isinstance(parsed, dict):
                        continue

                    if parsed.get("status") == "connected":
                        print(
                            f"Proxy connected: {parsed.get('provider')} -> {parsed.get('model')}"
                        )
                        continue

                    evt_type = parsed.get("type")

                    if evt_type == "session.created":
                        await ws.send(
                            json.dumps(
                                {
                                    "type": "session.update",
                                    "session": {
                                        "modalities": ["text", "audio"],
                                        "instructions": "Sapa dengan bahasa indonesia.",
                                        "input_audio_format": "pcm16",
                                        "output_audio_format": "pcm16",
                                        "turn_detection": {"type": "server_vad"},
                                        "voice": "alloy",
                                    },
                                }
                            )
                        )
                        session_ready = True
                        print("OpenAI realtime session ready")
                        continue

                    if evt_type == "response.audio.delta" and parsed.get("delta"):
                        audio = decode_base64_audio(parsed["delta"])
                        if len(audio) >= 2 and len(audio) % 2 == 0:
                            await asyncio.to_thread(speaker_stream.write, audio)
                        continue

                    if evt_type == "response.audio_transcript.delta" and parsed.get(
                        "delta"
                    ):
                        print(f"Bot(transcript): {parsed['delta']}")
                        continue

                    if evt_type == "response.text.delta" and parsed.get("delta"):
                        print(f"Bot(text): {parsed['delta']}")
                        continue

                    if evt_type == "error":
                        err = parsed.get("error", {})
                        if isinstance(err, dict):
                            print(f"Error: {err.get('message')}")
                        else:
                            print(f"Error: {err}")

            async def input_worker():
                print("Type message, '/commit' to force audio turn, '/quit' to exit")
                while not stop_event.is_set():
                    text = (await asyncio.to_thread(input, "You> ")).strip()
                    if not text:
                        continue
                    if text == "/quit":
                        stop_event.set()
                        return
                    if text == "/commit":
                        await ws.send(json.dumps({"type": "input_audio_buffer.commit"}))
                        await ws.send(json.dumps({"type": "response.create"}))
                        continue
                    await send_text(text)

            tasks = [
                asyncio.create_task(sender_audio()),
                asyncio.create_task(receiver()),
                asyncio.create_task(input_worker()),
            ]

            try:
                await stop_event.wait()
            finally:
                for task in tasks:
                    task.cancel()
                await asyncio.gather(*tasks, return_exceptions=True)
    finally:
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
    print("Dependencies: pip install websockets pyaudio")
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        sys.exit(0)
