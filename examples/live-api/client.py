import asyncio
import json
import websockets
import base64
import sys
import pyaudio  # pip install pyaudio
import audioop

# Audio configuration per Vertex AI Live API docs:
#   Input:  16-bit PCM, 16 kHz, mono, little-endian
#   Output: 16-bit PCM, 24 kHz, mono, little-endian
INPUT_RATE = 16000
OUTPUT_RATE = 24000
FORMAT = pyaudio.paInt16
CHANNELS = 1
CHUNK = 4096

# Lightweight input noise suppression (noise gate) to reduce false VAD triggers.
# NOTE: This is not full acoustic echo cancellation (AEC).
ENABLE_NOISE_GATE = True
NOISE_FLOOR_ALPHA = 0.95  # Higher = slower adaptation
NOISE_GATE_MULTIPLIER = 2.2  # Gate threshold = noise_floor * multiplier
NOISE_GATE_MIN_RMS = 250  # Absolute minimum RMS threshold
NOISE_GATE_HANGOVER = 3  # Keep a few chunks after speech to avoid choppy tail

# Pepebot Gateway WebSocket URL
URL = "ws://localhost:18790/v1/live"


class NoiseGate:
    def __init__(
        self,
        alpha=NOISE_FLOOR_ALPHA,
        multiplier=NOISE_GATE_MULTIPLIER,
        min_rms=NOISE_GATE_MIN_RMS,
        hangover=NOISE_GATE_HANGOVER,
    ):
        self.alpha = alpha
        self.multiplier = multiplier
        self.min_rms = min_rms
        self.hangover = hangover
        self.noise_floor = float(min_rms)
        self.hangover_left = 0

    def process(self, pcm_bytes):
        rms = audioop.rms(pcm_bytes, 2) if pcm_bytes else 0

        # Update floor only when likely in non-speech region
        if rms < (self.noise_floor * 1.5):
            self.noise_floor = (self.alpha * self.noise_floor) + (
                (1 - self.alpha) * rms
            )

        threshold = max(self.min_rms, self.noise_floor * self.multiplier)
        is_speech = rms >= threshold

        if is_speech:
            self.hangover_left = self.hangover
            return pcm_bytes, rms, threshold, False

        if self.hangover_left > 0:
            self.hangover_left -= 1
            return pcm_bytes, rms, threshold, False

        return b"\x00" * len(pcm_bytes), rms, threshold, True


def try_parse_json(data):
    """Try to parse data as JSON, whether it's bytes or string."""
    try:
        if isinstance(data, bytes):
            return json.loads(data.decode("utf-8"))
        return json.loads(data)
    except (json.JSONDecodeError, UnicodeDecodeError):
        return None


async def main():
    print(f"Connecting to Pepebot at {URL}...")

    try:
        async with websockets.connect(URL, max_size=10 * 1024 * 1024) as ws:
            print("Connected! Sending setup message...")

            # 1. Send Pepebot setup message (select provider + model)
            await ws.send(
                json.dumps(
                    {
                        "setup": {
                            "provider": "vertex",
                            "model": "gemini-live-2.5-flash-native-audio",
                        }
                    }
                )
            )

            # Wait for Pepebot confirmation
            resp = await ws.recv()
            parsed = try_parse_json(resp)
            print(f"Pepebot: {json.dumps(parsed) if parsed else resp}")
            if parsed and parsed.get("error"):
                print(f"❌ Error: {parsed['error']}")
                return

            # 2. Wait for Vertex AI setupComplete
            #    NOTE: Vertex may send this as a BINARY WebSocket frame!
            print("Waiting for Vertex AI to be ready...")
            setup_done = False
            while not setup_done:
                msg = await asyncio.wait_for(ws.recv(), timeout=15.0)
                parsed = try_parse_json(msg)
                if parsed:
                    print(f"  Vertex: {json.dumps(parsed)[:200]}")
                    if "setupComplete" in parsed:
                        print("✅ Vertex AI session ready!")
                        setup_done = True
                    elif "error" in parsed:
                        print(f"❌ Vertex error: {parsed}")
                        return
                else:
                    print(f"  [binary frame: {len(msg)} bytes - skipping]")

            # 3. Initialize audio streams
            p = pyaudio.PyAudio()

            stream_out = p.open(
                format=FORMAT,
                channels=CHANNELS,
                rate=OUTPUT_RATE,
                output=True,
                frames_per_buffer=CHUNK,
            )
            stream_in = p.open(
                format=FORMAT,
                channels=CHANNELS,
                rate=INPUT_RATE,
                input=True,
                frames_per_buffer=CHUNK,
            )

            print(
                f"\n🎤 Microphone is live (input={INPUT_RATE}Hz, output={OUTPUT_RATE}Hz)"
            )
            print("   Speak now! Press Ctrl+C to stop.\n")

            noise_gate = NoiseGate()

            # 4. Sender: Microphone → Pepebot → Vertex AI
            async def send_audio():
                try:
                    while True:
                        data = stream_in.read(CHUNK, exception_on_overflow=False)

                        if ENABLE_NOISE_GATE:
                            data, rms, threshold, muted = noise_gate.process(data)
                            if muted and rms > 0 and rms < threshold:
                                # Keep logs minimal: print only occasionally when near threshold
                                pass

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
                        await asyncio.sleep(0.01)
                except asyncio.CancelledError:
                    pass
                except Exception as e:
                    if "close" not in str(e).lower():
                        print(f"Sender error: {e}")

            # 5. Receiver: Vertex AI → Pepebot → Speakers
            async def receive_messages():
                try:
                    async for message in ws:
                        parsed = try_parse_json(message)

                        if parsed:
                            # JSON message
                            if "serverContent" in parsed:
                                content = parsed["serverContent"]

                                if content.get("turnComplete"):
                                    print("  [turn complete]")
                                    continue

                                if "modelTurn" in content:
                                    for part in content["modelTurn"].get("parts", []):
                                        if "text" in part:
                                            print(f"  🤖 {part['text']}")
                                        if "inlineData" in part:
                                            b64_audio = part["inlineData"].get(
                                                "data", ""
                                            )
                                            if b64_audio:
                                                pcm_data = base64.b64decode(b64_audio)
                                                stream_out.write(pcm_data)

                            elif "error" in parsed:
                                print(f"\n❌ Error: {parsed['error']}")
                            else:
                                keys = list(parsed.keys())
                                if keys:
                                    print(f"  [msg: {keys}]")

                        elif isinstance(message, bytes):
                            # Raw binary PCM audio data
                            stream_out.write(message)

                except websockets.exceptions.ConnectionClosed as e:
                    print(f"\nConnection closed: {e}")
                except asyncio.CancelledError:
                    pass
                except Exception as e:
                    if "close" not in str(e).lower():
                        print(f"\nReceiver error: {e}")

            sender = asyncio.create_task(send_audio())
            receiver = asyncio.create_task(receive_messages())

            try:
                done, pending = await asyncio.wait(
                    [sender, receiver], return_when=asyncio.FIRST_COMPLETED
                )
                for task in pending:
                    task.cancel()
            except asyncio.CancelledError:
                pass
            finally:
                sender.cancel()
                receiver.cancel()
                stream_in.close()
                stream_out.close()
                p.terminate()

    except ConnectionRefusedError:
        print(f"\n❌ Cannot connect to Pepebot at {URL}")
        print("Ensure Pepebot is running with live.enabled=true")
    except asyncio.TimeoutError:
        print("\n❌ Timeout waiting for Vertex AI setupComplete")
        print("Check your Vertex AI credentials and model name")
    except KeyboardInterrupt:
        print("\nStopping...")
        sys.exit(0)


if __name__ == "__main__":
    print("Dependencies: pip install websockets pyaudio")
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\nStopped.")
