#!/usr/bin/env node
/**
 * Type at Pepebot's Live API, hear the answer back as a WAV file.
 *
 *   npm install ws
 *   node paniki-client.js                          # ws://127.0.0.1:18790/v1/live
 *   node paniki-client.js ws://host:18790/v1/live
 *   LANG_CODE=en-US VOICE=nenden node paniki-client.js
 *
 * Deliberately text-in / file-out: `ws` is the only dependency, so this runs
 * anywhere. For live microphone conversation use paniki_client.py, where the audio
 * bindings are far less painful.
 *
 * It connects to Pepebot rather than the Realtime server directly, so the agent's
 * persona, skills and tools are all in play — ask it to list a directory and watch
 * the tool call go by.
 */

const fs = require("fs");
const path = require("path");
const readline = require("readline");
const WebSocket = require("ws");

const GATEWAY = process.argv[2] || process.env.PEPEBOT_LIVE_URL || "ws://127.0.0.1:18790/v1/live";
const PANIKI = (process.env.PANIKI_URL || "http://100.104.36.93:8000").replace(/\/+$/, "");
const LANGUAGE = process.env.LANG_CODE || "id-ID";
const VOICE = process.env.VOICE || "";
const OUT_DIR = process.env.OUT_DIR || ".";
const RATE = 24000; // pcm16 mono, both directions

let audioChunks = [];
let turn = 0;

// Read /v1/models and /v1/voices so nothing here goes stale on a redeploy.
async function discover(pathname, label) {
  try {
    const res = await fetch(`${PANIKI}${pathname}`);
    const body = await res.json();
    const ids = (body.data || []).map((row) => row.id);
    console.log(`-- ${label}: ${ids.slice(0, 6).join(", ")}${ids.length > 6 ? " …" : ""}`);
    return ids;
  } catch (err) {
    console.log(`-- could not read ${pathname} (${err.message}); relying on the gateway's config`);
    return [];
  }
}

// pcm16 mono has no container of its own, so wrap it in a 44-byte WAV header.
function writeWav(file, pcm) {
  const header = Buffer.alloc(44);
  header.write("RIFF", 0);
  header.writeUInt32LE(36 + pcm.length, 4);
  header.write("WAVE", 8);
  header.write("fmt ", 12);
  header.writeUInt32LE(16, 16);          // PCM chunk size
  header.writeUInt16LE(1, 20);           // format: PCM
  header.writeUInt16LE(1, 22);           // channels
  header.writeUInt32LE(RATE, 24);
  header.writeUInt32LE(RATE * 2, 28);    // byte rate
  header.writeUInt16LE(2, 32);           // block align
  header.writeUInt16LE(16, 34);          // bits per sample
  header.write("data", 36);
  header.writeUInt32LE(pcm.length, 40);
  fs.writeFileSync(file, Buffer.concat([header, pcm]));
}

function flushAudio() {
  if (!audioChunks.length) return;
  const pcm = Buffer.concat(audioChunks);
  audioChunks = [];
  const file = path.join(OUT_DIR, `reply-${++turn}.wav`);
  writeWav(file, pcm);
  const seconds = (pcm.length / 2 / RATE).toFixed(1);
  console.log(`\n[audio] ${file} (${seconds}s)`);
}

async function main() {
  const models = await discover("/v1/models", "models");
  await discover("/v1/voices", "voices");

  console.log(`-- connecting to ${GATEWAY}`);
  const ws = new WebSocket(GATEWAY);

  const send = (event) => ws.send(JSON.stringify(event));

  ws.on("open", () => {
    const setup = {
      provider: "realtime",
      agent: process.env.AGENT || "default",
      session_key: process.env.SESSION_KEY || "paniki-node",
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
      console.log(`-- proxy: ${evt.provider} -> ${evt.model}`);
      console.log("-- type a message and press enter (ctrl-c to quit)");
      rl.prompt();
      return;
    }
    if (evt.status === "reconnected") {
      console.log("\n-- upstream reconnected; conversation state was reset");
      return;
    }

    switch (evt.type) {
      case "response.audio.delta":
        audioChunks.push(Buffer.from(evt.delta, "base64"));
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
        flushAudio();
        rl.prompt();
        break;

      case "error":
        console.log(`\n[server error] ${JSON.stringify(evt.error)}`);
        break;
    }
  });

  ws.on("error", (err) => console.error(`-- socket error: ${err.message}`));
  ws.on("close", () => { console.log("-- socket closed"); process.exit(0); });

  const rl = readline.createInterface({ input: process.stdin, output: process.stdout, prompt: "> " });
  rl.on("line", (line) => {
    const text = line.trim();
    if (!text) return rl.prompt();
    if (ws.readyState !== WebSocket.OPEN) return;
    send({
      type: "conversation.item.create",
      item: { type: "message", role: "user", content: [{ type: "input_text", text }] },
    });
    send({ type: "response.create" });
  });
  rl.on("close", () => ws.close());
}

main();
