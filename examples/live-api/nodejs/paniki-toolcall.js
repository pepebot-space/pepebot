#!/usr/bin/env node
/**
 * Client-declared tools: the agent calls, this machine answers.
 *
 *   npm install ws
 *   node paniki-toolcall.js
 *   node paniki-toolcall.js ws://100.104.36.93:18790/v1/live
 *   APP=laptop node paniki-toolcall.js
 *
 * The gateway's own tools run on the gateway host. The three declared here run right
 * here instead, so the agent can reach things that only exist on this machine:
 *
 *   device_info      hostname, OS and timezone of this machine
 *   read_clipboard   whatever you last copied
 *   notify           a desktop notification on your screen
 *
 * Ask "what's on my clipboard?" or "send me a notification saying hello" and watch
 * the call come back down the socket to be executed locally.
 *
 * Text in, text out — audio is a separate concern, and paniki-client.js covers it.
 * Client tools work the same way in a voice session.
 */

const os = require("os");
const readline = require("readline");
const { execFile } = require("child_process");
const WebSocket = require("ws");

const GATEWAY = process.argv[2] || process.env.PEPEBOT_LIVE_URL || "ws://127.0.0.1:18790/v1/live";
const PANIKI = (process.env.PANIKI_URL || "http://100.104.36.93:8000").replace(/\/+$/, "");
const APP = process.env.APP || "laptop";
const LANGUAGE = process.env.LANG_CODE || "id-ID";
// One session key per client by default, so a Node session and a browser session keep
// separate histories instead of talking over each other. SESSION=name to pick another.
const SESSION = process.env.SESSION || process.env.SESSION_KEY || "js-session";

// ------------------------------------------------------------------ the tools
// Each returns a string, or throws. A thrown message becomes the tool's error, which
// reaches the model as "Error: ..." — the agent can then say so instead of stalling.

function run(cmd, args) {
  return new Promise((resolve, reject) => {
    execFile(cmd, args, { timeout: 8000 }, (err, stdout, stderr) => {
      if (err) return reject(new Error((stderr || err.message).trim().split("\n")[0]));
      resolve(stdout.trim());
    });
  });
}

const tools = {
  device_info: {
    description: "Get details about the machine the user is sitting at: hostname, operating system, timezone and local time.",
    parameters: { type: "object", properties: {} },
    run: async () => [
      `hostname ${os.hostname()}`,
      `os ${os.type()} ${os.release()} on ${os.arch()}`,
      `timezone ${Intl.DateTimeFormat().resolvedOptions().timeZone}`,
      `local time ${new Date().toLocaleTimeString([], { hour12: false })}`,
      `uptime ${Math.round(os.uptime() / 3600)} hours`,
    ].join(", "),
  },

  read_clipboard: {
    description: "Read the text currently on the user's clipboard.",
    parameters: { type: "object", properties: {} },
    run: async () => {
      const text =
        process.platform === "darwin" ? await run("pbpaste", []) :
        process.platform === "win32" ? await run("powershell", ["-command", "Get-Clipboard"]) :
        await run("xclip", ["-selection", "clipboard", "-o"]);
      if (!text) throw new Error("the clipboard is empty");
      return text.length > 800 ? text.slice(0, 800) + "… (truncated)" : text;
    },
  },

  notify: {
    description: "Show a desktop notification on the user's screen. Use it when asked to remind or alert them.",
    parameters: {
      type: "object",
      properties: { message: { type: "string", description: "The text to show" } },
      required: ["message"],
    },
    run: async ({ message }) => {
      if (!message) throw new Error("nothing to show: message was empty");
      if (process.platform === "darwin") {
        // Quotes inside the message would end the AppleScript string early.
        const safe = String(message).replace(/["\\]/g, " ");
        await run("osascript", ["-e", `display notification "${safe}" with title "Pepebot"`]);
      } else if (process.platform === "win32") {
        throw new Error("desktop notifications are not wired up on Windows in this example");
      } else {
        await run("notify-send", ["Pepebot", String(message)]);
      }
      return `shown on screen: ${message}`;
    },
  },
};

// ------------------------------------------------------------------ session
async function discover(pathname, label) {
  try {
    const res = await fetch(`${PANIKI}${pathname}`);
    const ids = ((await res.json()).data || []).map((row) => row.id);
    console.log(`-- ${label}: ${ids.slice(0, 4).join(", ")}${ids.length > 4 ? " …" : ""}`);
    return ids;
  } catch (err) {
    console.log(`-- could not read ${pathname} (${err.message}); relying on the gateway's config`);
    return [];
  }
}

async function main() {
  const models = await discover("/v1/models", "models");

  console.log(`-- connecting to ${GATEWAY}`);
  const ws = new WebSocket(GATEWAY);
  const send = (event) => ws.send(JSON.stringify(event));

  ws.on("open", () => {
    const setup = {
      provider: "realtime",
      agent: process.env.AGENT || "default",
      session_key: SESSION,
      enable_tools: true,
      language: LANGUAGE,
      // Ours to run. Pepebot namespaces them as "<APP>-<name>" so they cannot shadow
      // one of the agent's own, and asks us for them by the bare name.
      app: APP,
      tools: Object.entries(tools).map(([name, t]) => ({
        name,
        description: t.description,
        parameters: t.parameters,
      })),
      client_tool_timeout_ms: 20000,
    };
    if (models.length) setup.model = models[0];
    send({ setup });
    console.log(`-- session "${SESSION}" | declared as ${APP}: ${Object.keys(tools).join(", ")}`);
  });

  ws.on("message", async (raw) => {
    let evt;
    try { evt = JSON.parse(raw.toString()); } catch { return; }

    if (evt.status === "connected") {
      console.log(`-- proxy: ${evt.provider} -> ${evt.model}`);
      console.log('-- try: "what is on my clipboard?" or "notify me that the build is done"');
      rl.prompt();
      return;
    }

    // A call for one of our tools. Answer it, whatever happens.
    if (evt.type === "tool_call") {
      const tool = tools[evt.name];
      console.log(`\n[tool_call] ${evt.name}(${JSON.stringify(evt.arguments || {})})`);
      if (!tool) {
        send({ type: "tool_result", call_id: evt.call_id, error: `no tool named ${evt.name} on this client` });
        return;
      }
      try {
        const output = await tool.run(evt.arguments || {});
        console.log(`[tool_result] ${output.split("\n")[0].slice(0, 120)}`);
        send({ type: "tool_result", call_id: evt.call_id, output });
      } catch (err) {
        console.log(`[tool_error] ${err.message}`);
        send({ type: "tool_result", call_id: evt.call_id, error: err.message });
      }
      return;
    }

    switch (evt.type) {
      case "response.output_item.done":
        if (evt.item && evt.item.type === "function_call" && !evt.item.name.startsWith(APP + "-")) {
          console.log(`\n[gateway tool] ${evt.item.name}(${evt.item.arguments || ""})`);
        }
        break;
      case "response.audio_transcript.done":
        console.log(`\nbot: ${(evt.transcript || "").trim()}`);
        rl.prompt();
        break;
      case "error":
        console.log(`\n[server error] ${JSON.stringify(evt.error)}`);
        break;
    }
  });

  ws.on("error", (err) => console.error(`-- socket error: ${err.message}`));
  ws.on("close", () => { console.log("\n-- socket closed"); process.exit(0); });

  const rl = readline.createInterface({ input: process.stdin, output: process.stdout, prompt: "> " });
  rl.on("line", (line) => {
    const text = line.trim();
    if (!text || ws.readyState !== WebSocket.OPEN) return rl.prompt();
    send({
      type: "conversation.item.create",
      item: { type: "message", role: "user", content: [{ type: "input_text", text }] },
    });
    send({ type: "response.create" });
  });
  rl.on("close", () => ws.close());
}

main();
