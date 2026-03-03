import asyncio
import base64
import json
import signal
import sys
from typing import Optional

import audioop
import pyaudio  # pip install pyaudio
import websockets


# Audio configuration per Vertex AI Live API docs:
#   Input:  16-bit PCM, 16 kHz, mono, little-endian
#   Output: 16-bit PCM, 24 kHz, mono, little-endian
INPUT_RATE = 16000
OUTPUT_RATE = 24000
CHANNELS = 1
SAMPLE_WIDTH = 2  # PCM16
FORMAT = pyaudio.paInt16

# Smaller chunks improve responsiveness and smoothness.
INPUT_CHUNK = 2048
OUTPUT_CHUNK = 4096

URL = "ws://localhost:18790/v1/live"


# Lightweight noise gate for mic input (optional)
ENABLE_NOISE_GATE = False
NOISE_FLOOR_ALPHA = 0.95
NOISE_GATE_MULTIPLIER = 2.0
NOISE_GATE_MIN_RMS = 180
NOISE_GATE_HANGOVER = 3

# Vertex/Gemini usually deliver audio via JSON inlineData.
# Raw binary frames may contain non-audio envelopes on some backends.
ENABLE_RAW_BINARY_AUDIO = False
MIN_BINARY_PCM_BYTES = 640


class NoiseGate:
    def __init__(self):
        self.noise_floor = float(NOISE_GATE_MIN_RMS)
        self.hangover_left = 0

    def process(self, pcm_bytes: bytes) -> bytes:
        if not pcm_bytes:
            return pcm_bytes

        rms = audioop.rms(pcm_bytes, SAMPLE_WIDTH)

        if rms < self.noise_floor * 1.5:
            self.noise_floor = (
                NOISE_FLOOR_ALPHA * self.noise_floor + (1 - NOISE_FLOOR_ALPHA) * rms
            )

        threshold = max(NOISE_GATE_MIN_RMS, self.noise_floor * NOISE_GATE_MULTIPLIER)
        is_speech = rms >= threshold

        if is_speech:
            self.hangover_left = NOISE_GATE_HANGOVER
            return pcm_bytes

        if self.hangover_left > 0:
            self.hangover_left -= 1
            return pcm_bytes

        return b"\x00" * len(pcm_bytes)


def try_parse_json(data):
    try:
        if isinstance(data, bytes):
            return json.loads(data.decode("utf-8", errors="ignore"))
        return json.loads(data)
    except Exception:
        return None


def extract_inline_audio(parsed: dict) -> Optional[bytes]:
    server_content = parsed.get("serverContent")
    if not isinstance(server_content, dict):
        return None

    model_turn = server_content.get("modelTurn")
    if not isinstance(model_turn, dict):
        return None

    parts = model_turn.get("parts")
    if not isinstance(parts, list):
        return None

    chunks = []
    for part in parts:
        if not isinstance(part, dict):
            continue
        inline_data = part.get("inlineData")
        if not isinstance(inline_data, dict):
            continue
        b64_audio = inline_data.get("data")
        if not isinstance(b64_audio, str) or not b64_audio:
            continue

        # Support standard base64 and base64url
        normalized = b64_audio.replace("-", "+").replace("_", "/")
        while len(normalized) % 4 != 0:
            normalized += "="
        try:
            chunks.append(base64.b64decode(normalized))
        except Exception:
            continue

    if not chunks:
        return None
    return b"".join(chunks)


async def main():
    print(f"Connecting to Pepebot at {URL}...")

    stop_event = asyncio.Event()
    loop = asyncio.get_running_loop()

    def _handle_stop(*_):
        stop_event.set()

    for sig in (signal.SIGINT, signal.SIGTERM):
        try:
            loop.add_signal_handler(sig, _handle_stop)
        except NotImplementedError:
            pass

    # Fallback signal handler (helps on some platforms/envs)
    signal.signal(signal.SIGINT, lambda *_: stop_event.set())

    p = pyaudio.PyAudio()
    output_queue: asyncio.Queue[bytes] = asyncio.Queue(maxsize=64)

    stream_out = p.open(
        format=FORMAT,
        channels=CHANNELS,
        rate=OUTPUT_RATE,
        output=True,
        frames_per_buffer=OUTPUT_CHUNK,
    )

    stream_in = p.open(
        format=FORMAT,
        channels=CHANNELS,
        rate=INPUT_RATE,
        input=True,
        frames_per_buffer=INPUT_CHUNK,
    )

    noise_gate = NoiseGate()

    async def playback_worker():
        while not stop_event.is_set():
            try:
                pcm = await asyncio.wait_for(output_queue.get(), timeout=0.2)
            except asyncio.TimeoutError:
                continue

            try:
                await asyncio.to_thread(stream_out.write, pcm)
            except Exception as e:
                if not stop_event.is_set():
                    print(f"Playback error: {e}")
                return

    try:
        async with websockets.connect(
            URL,
            max_size=20 * 1024 * 1024,
            ping_interval=20,
            ping_timeout=20,
            close_timeout=5,
        ) as ws:
            print("Connected! Sending setup...")

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

            setup_ok = False
            while not setup_ok and not stop_event.is_set():
                msg = await asyncio.wait_for(ws.recv(), timeout=15)

                parsed = try_parse_json(msg)
                if parsed is None:
                    continue

                if parsed.get("error"):
                    print(f"Error: {parsed['error']}")
                    return

                if parsed.get("status") == "connected":
                    print(
                        f"Proxy connected: {parsed.get('provider')} -> {parsed.get('model')}"
                    )
                    continue

                if "setupComplete" in parsed:
                    setup_ok = True
                    print("Live session ready")

            if not setup_ok:
                return

            print(
                f"Mic live (input={INPUT_RATE}Hz), speaker live (output={OUTPUT_RATE}Hz)"
            )
            print("Speak now... Press Ctrl+C to stop.")

            async def sender():
                while not stop_event.is_set():
                    try:
                        data = await asyncio.to_thread(
                            stream_in.read,
                            INPUT_CHUNK,
                            exception_on_overflow=False,
                        )
                        if ENABLE_NOISE_GATE:
                            data = noise_gate.process(data)

                        b64_data = base64.b64encode(data).decode("utf-8")
                        await ws.send(
                            json.dumps(
                                {
                                    "realtimeInput": {
                                        "mediaChunks": [
                                            {
                                                "mimeType": "audio/pcm;rate=16000",
                                                "data": b64_data,
                                            }
                                        ]
                                    }
                                }
                            )
                        )
                    except asyncio.CancelledError:
                        return
                    except Exception as e:
                        if not stop_event.is_set():
                            print(f"Sender error: {e}")
                        stop_event.set()
                        return

            async def receiver():
                while not stop_event.is_set():
                    try:
                        message = await ws.recv()
                    except asyncio.CancelledError:
                        return
                    except websockets.exceptions.ConnectionClosed as e:
                        if not stop_event.is_set():
                            print(f"Connection closed: {e}")
                        stop_event.set()
                        return
                    except Exception as e:
                        if not stop_event.is_set():
                            print(f"Receiver error: {e}")
                        stop_event.set()
                        return

                    # Raw binary PCM audio (common on realtime endpoints)
                    if isinstance(message, bytes):
                        # Some providers send JSON in binary frames
                        parsed_bin = try_parse_json(message)
                        if isinstance(parsed_bin, dict):
                            audio_inline = extract_inline_audio(parsed_bin)
                            if (
                                audio_inline
                                and len(audio_inline) >= 2
                                and len(audio_inline) % 2 == 0
                            ):
                                if output_queue.full():
                                    try:
                                        output_queue.get_nowait()
                                    except asyncio.QueueEmpty:
                                        pass
                                await output_queue.put(audio_inline)
                            continue

                        if ENABLE_RAW_BINARY_AUDIO:
                            if (
                                len(message) >= MIN_BINARY_PCM_BYTES
                                and len(message) % 2 == 0
                            ):
                                if output_queue.full():
                                    try:
                                        output_queue.get_nowait()
                                    except asyncio.QueueEmpty:
                                        pass
                                await output_queue.put(message)
                        continue

                    parsed = try_parse_json(message)
                    if parsed is None:
                        continue

                    if parsed.get("error"):
                        print(f"Error: {parsed['error']}")
                        continue

                    if parsed.get("serverContent", {}).get("turnComplete"):
                        continue

                    audio_inline = extract_inline_audio(parsed)
                    if audio_inline:
                        if len(audio_inline) >= 2 and len(audio_inline) % 2 == 0:
                            if output_queue.full():
                                try:
                                    output_queue.get_nowait()
                                except asyncio.QueueEmpty:
                                    pass
                            await output_queue.put(audio_inline)

                    # Optional text print
                    model_turn = parsed.get("serverContent", {}).get("modelTurn", {})
                    parts = (
                        model_turn.get("parts", [])
                        if isinstance(model_turn, dict)
                        else []
                    )
                    for part in parts:
                        if isinstance(part, dict) and part.get("text"):
                            print(f"Bot: {part['text']}")

            tasks = [
                asyncio.create_task(playback_worker()),
                asyncio.create_task(sender()),
                asyncio.create_task(receiver()),
            ]

            try:
                await stop_event.wait()
            except KeyboardInterrupt:
                stop_event.set()

            for t in tasks:
                t.cancel()
            await asyncio.gather(*tasks, return_exceptions=True)

    except ConnectionRefusedError:
        print(
            f"Cannot connect to {URL}. Ensure gateway is running and live.enabled=true"
        )
    except asyncio.TimeoutError:
        print("Timeout waiting for setupComplete")
    finally:
        try:
            stream_in.stop_stream()
            stream_in.close()
        except Exception:
            pass
        try:
            stream_out.stop_stream()
            stream_out.close()
        except Exception:
            pass
        p.terminate()


if __name__ == "__main__":
    print("Dependencies: pip install websockets pyaudio")
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        sys.exit(0)
