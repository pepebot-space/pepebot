#!/usr/bin/env node
/**
 * Full-duplex terminal client for Pepebot's Live API: speak, hear the answer.
 *
 *   npm install                                   # ws + speaker
 *   node paniki-client.js                         # ws://127.0.0.1:18790/v1/live
 *   node paniki-client.js ws://host:18790/v1/live
 *   LANG_CODE=en-US VOICE=nenden node paniki-client.js
 *   MIC=":1" node paniki-client.js                # pick another input device
 *   NO_MIC=1 node paniki-client.js                # type only
 *
 * You can also type at it at any time; both paths feed the same session.
 *
 * Microphone capture goes through ffmpeg rather than the `mic` package, which
 * shells out to sox/arecord — ffmpeg is one binary that covers macOS, Linux and
 * Windows. Playback uses the `speaker` package.
 *
 * List input devices:
 *   macOS   ffmpeg -f avfoundation -list_devices true -i ""
 *   Linux   arecord -l
 */

const { spawn, spawnSync } = require("child_process");
const fs = require("fs");
const path = require("path");
const readline = require("readline");
const WebSocket = require("ws");

const GATEWAY = process.argv[2] || process.env.PEPEBOT_LIVE_URL || "ws://127.0.0.1:18790/v1/live";
const PANIKI = (process.env.PANIKI_URL || "http://100.104.36.93:8000").replace(/\/+$/, "");
const LANGUAGE = process.env.LANG_CODE || "id-ID";
const VOICE = process.env.VOICE || "";
const OUT_DIR = process.env.OUT_DIR || ".";
// One session key per client by default, so separate clients keep separate histories.
const SESSION = process.env.SESSION || process.env.SESSION_KEY || "js-session";

const RATE = 24000;              // pcm16 mono, both directions
const FRAME_BYTES = 480 * 2;     // 20ms: low latency, snappy barge-in

let ws;
let ffmpeg = null;
let speaker = null;
let Speaker = null;
let micTail = Buffer.alloc(0);
let micPeak = 0;
let micFrames = 0;
let micHeardAnything = false;
let wavChunks = [];
let turn = 0;

try {
  Speaker = require("speaker");
} catch {
  console.log("-- `speaker` not installed; replies will be written as WAV files instead");
}

// ---------------------------------------------------------------- discovery
// Read /v1/models and /v1/voices so nothing here goes stale on a redeploy.
async function discover(pathname, label) {
  try {
    const res = await fetch(`${PANIKI}${pathname}`);
    const ids = ((await res.json()).data || []).map((row) => row.id);
    console.log(`-- ${label}: ${ids.slice(0, 6).join(", ")}${ids.length > 6 ? " …" : ""}`);
    return ids;
  } catch (err) {
    console.log(`-- could not read ${pathname} (${err.message}); relying on the gateway's config`);
    return [];
  }
}

// ---------------------------------------------------------------- playback
function openSpeaker() {
  if (!Speaker) return null;
  return new Speaker({ channels: 1, bitDepth: 16, sampleRate: RATE });
}

// Audio deltas arrive in bursts, and CoreAudio drains faster than the first few
// land — starting playback immediately means a stream of underflow warnings and a
// choppy first word. Hold ~300ms back before opening the device.
const PREBUFFER_BYTES = RATE * 2 * 0.3;
let prebuffer = [];
let prebuffered = 0;

function play(pcm) {
  if (!Speaker) { wavChunks.push(pcm); return; }

  if (speaker) { speaker.write(pcm); return; }

  prebuffer.push(pcm);
  prebuffered += pcm.length;
  if (prebuffered < PREBUFFER_BYTES) return;

  speaker = openSpeaker();
  speaker.write(Buffer.concat(prebuffer));
  prebuffer = [];
  prebuffered = 0;
}

// End of turn: flush what is still held back, then close the device. Leaving it open
// makes CoreAudio call back into an empty buffer forever, which floods the terminal
// with underflow warnings. end() plays out what is queued before closing.
function closePlayback() {
  if (!Speaker) return;
  if (prebuffer.length) {
    if (!speaker) speaker = openSpeaker();
    speaker.write(Buffer.concat(prebuffer));
    prebuffer = [];
    prebuffered = 0;
  }
  if (speaker) {
    speaker.end();
    speaker = null;
  }
}

// Barge-in: `speaker` has no flush, so drop the stream and start a new one. Anything
// already handed to Core Audio still plays, which is a few tens of milliseconds.
function stopPlayback() {
  prebuffer = [];
  prebuffered = 0;
  if (speaker) {
    speaker.removeAllListeners("error");
    speaker.on("error", () => {});
    try { speaker.destroy(); } catch {}
    speaker = null;
  }
  wavChunks = [];
}

function flushWav() {
  if (Speaker || !wavChunks.length) return;
  const pcm = Buffer.concat(wavChunks);
  wavChunks = [];
  const header = Buffer.alloc(44);
  header.write("RIFF", 0);
  header.writeUInt32LE(36 + pcm.length, 4);
  header.write("WAVE", 8);
  header.write("fmt ", 12);
  header.writeUInt32LE(16, 16);
  header.writeUInt16LE(1, 20);
  header.writeUInt16LE(1, 22);
  header.writeUInt32LE(RATE, 24);
  header.writeUInt32LE(RATE * 2, 28);
  header.writeUInt16LE(2, 32);
  header.writeUInt16LE(16, 34);
  header.write("data", 36);
  header.writeUInt32LE(pcm.length, 40);
  const file = path.join(OUT_DIR, `reply-${++turn}.wav`);
  fs.writeFileSync(file, Buffer.concat([header, pcm]));
  console.log(`[audio] ${file} (${(pcm.length / 2 / RATE).toFixed(1)}s)`);
}

// ---------------------------------------------------------------- microphone
function ffmpegInput() {
  if (process.env.MIC_ARGS) return process.env.MIC_ARGS.split(" ");
  const device = process.env.MIC || (process.platform === "darwin" ? ":0" : "default");
  if (process.platform === "darwin") return ["-f", "avfoundation", "-i", device];
  if (process.platform === "win32") return ["-f", "dshow", "-i", `audio=${device}`];
  return ["-f", "alsa", "-i", device];
}

function startMic() {
  if (process.env.NO_MIC) return;
  if (spawnSync("ffmpeg", ["-version"], { stdio: "ignore" }).status !== 0) {
    console.log("-- ffmpeg not found; running in text-only mode (brew install ffmpeg)");
    return;
  }

  ffmpeg = spawn("ffmpeg", [
    "-hide_banner", "-loglevel", "error",
    ...ffmpegInput(),
    "-ac", "1", "-ar", String(RATE), "-f", "s16le", "-",
  ]);

  ffmpeg.stdout.on("data", (chunk) => {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;

    // ffmpeg hands out arbitrary block sizes; the wire wants exact 20ms frames.
    let buf = micTail.length ? Buffer.concat([micTail, chunk]) : chunk;
    let at = 0;
    for (; at + FRAME_BYTES <= buf.length; at += FRAME_BYTES) {
      const frame = buf.subarray(at, at + FRAME_BYTES);
      for (let i = 0; i < frame.length; i += 2) {
        micPeak = Math.max(micPeak, Math.abs(frame.readInt16LE(i)));
      }
      ws.send(JSON.stringify({
        type: "input_audio_buffer.append",
        audio: frame.toString("base64"),
      }));

      // A level meter, because a denied microphone and a quiet room look identical.
      if (++micFrames % 50 === 0) {
        if (micPeak > 0) micHeardAnything = true;
        const bar = "#".repeat(Math.min(20, Math.floor((micPeak * 20) / 32768)));
        process.stdout.write(`\r  mic ${String(micPeak).padStart(5)} |${bar.padEnd(20)}|`);
        if (micFrames === 250 && !micHeardAnything) {
          process.stdout.write(
            "\n!! Five seconds of pure silence — the input is delivering zeros.\n" +
            "   macOS answers a denied microphone that way instead of erroring.\n" +
            "   Run this from a terminal app that holds the mic grant, or check\n" +
            "   System Settings > Privacy & Security > Microphone.\n"
          );
        }
        micPeak = 0;
      }
    }
    micTail = buf.subarray(at);
  });

  ffmpeg.stderr.on("data", (d) => {
    const msg = d.toString().trim();
    if (msg) console.log(`\n-- ffmpeg: ${msg.split("\n")[0]}`);
  });
  ffmpeg.on("error", (err) => console.log(`-- mic failed: ${err.message}`));

  console.log("-- mic on (ffmpeg); speak, or type a message");
}

function stopMic() {
  if (ffmpeg) { ffmpeg.kill("SIGKILL"); ffmpeg = null; }
}

// ---------------------------------------------------------------- session
async function main() {
  const models = await discover("/v1/models", "models");
  await discover("/v1/voices", "voices");

  console.log(`-- connecting to ${GATEWAY}`);
  ws = new WebSocket(GATEWAY);
  const send = (event) => ws.send(JSON.stringify(event));

  ws.on("open", () => {
    const setup = {
      provider: "realtime",
      agent: process.env.AGENT || "default",
      session_key: SESSION,
      enable_tools: true,
      language: LANGUAGE,
    };
    if (models.length) setup.model = models[0];
    send({ setup });

    // Persona, skills and tools are already on the wire from Pepebot; voice is the
    // one thing worth overriding per session.
    if (VOICE) send({ type: "session.update", session: { voice: VOICE } });
  });

  ws.on("message", (raw) => {
    let evt;
    try { evt = JSON.parse(raw.toString()); } catch { return; }

    if (evt.status === "connected") {
      console.log(`-- proxy: ${evt.provider} -> ${evt.model} | session "${SESSION}"`);
      startMic();
      rl.prompt();
      return;
    }
    if (evt.status === "reconnected") {
      console.log("\n-- upstream reconnected; conversation state was reset");
      stopPlayback();
      return;
    }

    switch (evt.type) {
      case "response.audio.delta":
        play(Buffer.from(evt.delta, "base64"));
        break;

      case "input_audio_buffer.speech_started":
        stopPlayback();
        process.stdout.write("\n[listening] ");
        break;

      case "conversation.item.input_audio_transcription.completed":
        console.log(`\nyou: ${(evt.transcript || "").trim()}`);
        break;

      case "response.audio_transcript.delta":
        process.stdout.write(evt.delta || "");
        break;

      case "response.audio_transcript.done":
        console.log(`\nbot: ${(evt.transcript || "").trim()}`);
        break;

      case "response.output_item.done":
        if (evt.item && evt.item.type === "function_call") {
          console.log(`\n[tool] ${evt.item.name}(${evt.item.arguments || ""}) — Pepebot is running it`);
        }
        break;

      case "response.done":
        closePlayback();
        flushWav();
        rl.prompt();
        break;

      case "error":
        console.log(`\n[server error] ${JSON.stringify(evt.error)}`);
        break;
    }
  });

  ws.on("error", (err) => console.error(`-- socket error: ${err.message}`));
  ws.on("close", () => { stopMic(); console.log("\n-- socket closed"); process.exit(0); });

  const rl = readline.createInterface({ input: process.stdin, output: process.stdout, prompt: "> " });
  rl.on("line", (line) => {
    const text = line.trim();
    if (!text || !ws || ws.readyState !== WebSocket.OPEN) return rl.prompt();
    send({
      type: "conversation.item.create",
      item: { type: "message", role: "user", content: [{ type: "input_text", text }] },
    });
    send({ type: "response.create" });
  });
  rl.on("close", () => { stopMic(); ws.close(); });
}

process.on("SIGINT", () => { stopMic(); process.exit(0); });

main();
