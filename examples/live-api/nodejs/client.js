const readline = require("readline");
const WebSocket = require("ws");
const mic = require("mic");
const Speaker = require("speaker");

const URL = "ws://localhost:18790/v1/live";
const INPUT_SAMPLE_RATE = 16000;
const OUTPUT_SAMPLE_RATE = 24000;
const INPUT_MIME = "audio/pcm;rate=16000";

let ws;
let micInstance;
let micInputStream;

const speaker = new Speaker({
  channels: 1,
  bitDepth: 16,
  sampleRate: OUTPUT_SAMPLE_RATE,
  signed: true,
});

function decodeBase64Audio(base64String) {
  let s = (base64String || "").replace(/-/g, "+").replace(/_/g, "/");
  while (s.length % 4 !== 0) s += "=";
  return Buffer.from(s, "base64");
}

function tryParseTextFrame(str) {
  try {
    return JSON.parse(str);
  } catch {
    return null;
  }
}

function sendText(text) {
  if (!text || !ws || ws.readyState !== WebSocket.OPEN) return;
  ws.send(
    JSON.stringify({
      clientContent: {
        turns: [{ role: "user", parts: [{ text }] }],
        turnComplete: true,
      },
    })
  );
  console.log(`You: ${text}`);
}

function setupMic() {
  micInstance = mic({
    rate: String(INPUT_SAMPLE_RATE),
    channels: "1",
    bitwidth: "16",
    encoding: "signed-integer",
    endian: "little",
    device: "default",
    fileType: "raw",
  });

  micInputStream = micInstance.getAudioStream();

  micInputStream.on("data", (chunk) => {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    const b64 = chunk.toString("base64");
    ws.send(
      JSON.stringify({
        realtimeInput: {
          mediaChunks: [{
            mimeType: INPUT_MIME,
            data: b64,
          }],
        },
      })
    );
  });

  micInputStream.on("error", (err) => {
    console.error("Mic error:", err.message);
  });

  micInstance.start();
  console.log("Mic started");
}

function stopMic() {
  if (!micInstance) return;
  micInstance.stop();
  micInstance = null;
  micInputStream = null;
  console.log("Mic stopped");
}

function connect() {
  console.log(`Connecting to ${URL}...`);
  ws = new WebSocket(URL);

  ws.on("open", () => {
    console.log("WebSocket open, sending setup");
    ws.send(
      JSON.stringify({
        setup: {
          provider: "vertex",
          model: "gemini-live-2.5-flash-native-audio",
          agent: "default",
          enable_tools: true,
        },
      })
    );
  });

  ws.on("message", (data, isBinary) => {
    if (isBinary) {
      const parsed = tryParseTextFrame(data.toString("utf-8"));
      if (parsed) {
        handleParsedMessage(parsed);
      } else if (data.length >= 2 && data.length % 2 === 0) {
        speaker.write(data);
      }
      return;
    }

    const parsed = tryParseTextFrame(String(data));
    if (parsed) handleParsedMessage(parsed);
  });

  ws.on("error", (err) => {
    console.error("WebSocket error:", err.message);
  });

  ws.on("close", () => {
    console.log("WebSocket closed");
    stopMic();
    process.exit(0);
  });
}

function handleParsedMessage(data) {
  if (data.status === "connected") {
    console.log(`Proxy connected: ${data.provider} -> ${data.model}`);
    return;
  }
  if (data.setupComplete !== undefined) {
    console.log("Live session ready");
    return;
  }
  if (data.error) {
    console.error(`Error: ${data.error}`);
    return;
  }

  const serverContent = data.serverContent;
  if (!serverContent || !serverContent.modelTurn || !Array.isArray(serverContent.modelTurn.parts)) {
    return;
  }

  for (const part of serverContent.modelTurn.parts) {
    if (part.text) console.log(`Bot: ${part.text}`);
    if (part.inlineData && part.inlineData.data) {
      const pcm = decodeBase64Audio(part.inlineData.data);
      if (pcm.length >= 2 && pcm.length % 2 === 0) speaker.write(pcm);
    }
  }
}

function setupInput() {
  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
    prompt: "You> ",
  });

  console.log("Mic auto ON. Command: /quit | or type message");
  rl.prompt();

  rl.on("line", (line) => {
    const text = line.trim();
    if (!text) {
      rl.prompt();
      return;
    }

    if (text === "/quit") {
      rl.close();
      stopMic();
      if (ws && ws.readyState === WebSocket.OPEN) ws.close();
      return;
    }

    sendText(text);
    rl.prompt();
  });
}

process.on("SIGINT", () => {
  stopMic();
  if (ws && ws.readyState === WebSocket.OPEN) ws.close();
  process.exit(0);
});

connect();
setupMic();
setupInput();
