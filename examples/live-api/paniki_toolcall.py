#!/usr/bin/env python3
"""Client-declared tools: the agent calls, this machine answers.

    pip install websockets
    python3 paniki_toolcall.py
    python3 paniki_toolcall.py ws://100.104.36.93:18790/v1/live
    APP=laptop python3 paniki_toolcall.py

The gateway's own tools run on the gateway host. The three declared here run right
here instead, so the agent can reach things that only exist on this machine:

    device_info      hostname, OS and timezone of this machine
    read_clipboard   whatever you last copied
    notify           a desktop notification on your screen

Ask "what's on my clipboard?" or "remind me on screen that the build is done" and
watch the call come back down the socket to be executed locally.

Text in, text out — audio is a separate concern, and paniki_client.py covers it.
Client tools work the same way in a voice session.
"""

import asyncio
import json
import os
import platform
import socket
import sys
import time

import websockets

sys.stdout.reconfigure(line_buffering=True)

GATEWAY = sys.argv[1] if len(sys.argv) > 1 else os.getenv("PEPEBOT_LIVE_URL", "ws://127.0.0.1:18790/v1/live")
PANIKI = os.getenv("PANIKI_URL", "http://100.104.36.93:8000").rstrip("/")
APP = os.getenv("APP", "laptop")
LANGUAGE = os.getenv("LANG_CODE", "id-ID")
# One session key per client by default, so a Python session and a browser session keep
# separate histories instead of talking over each other. SESSION=name to pick another.
SESSION = os.getenv("SESSION") or os.getenv("SESSION_KEY") or "py-session"


# --------------------------------------------------------------------- the tools
# Each returns a string, or raises. A raised message becomes the tool's error, which
# reaches the model as "Error: ..." — the agent can then say so instead of stalling.


async def shell(*args: str) -> str:
    proc = await asyncio.create_subprocess_exec(
        *args, stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.PIPE
    )
    try:
        out, err = await asyncio.wait_for(proc.communicate(), timeout=8)
    except asyncio.TimeoutError:
        proc.kill()
        raise RuntimeError(f"{args[0]} timed out")
    if proc.returncode != 0:
        detail = (err or b"").decode().strip().splitlines()
        raise RuntimeError(detail[0] if detail else f"{args[0]} failed")
    return out.decode().strip()


async def device_info(_args: dict) -> str:
    return ", ".join([
        f"hostname {socket.gethostname()}",
        f"os {platform.system()} {platform.release()} on {platform.machine()}",
        f"timezone {time.tzname[time.daylight and time.localtime().tm_isdst > 0]}",
        f"local time {time.strftime('%H:%M:%S')}",
        f"python {platform.python_version()}",
    ])


async def read_clipboard(_args: dict) -> str:
    if sys.platform == "darwin":
        text = await shell("pbpaste")
    elif sys.platform.startswith("win"):
        text = await shell("powershell", "-command", "Get-Clipboard")
    else:
        text = await shell("xclip", "-selection", "clipboard", "-o")
    if not text:
        raise RuntimeError("the clipboard is empty")
    return text if len(text) <= 800 else text[:800] + "… (truncated)"


async def notify(args: dict) -> str:
    message = str(args.get("message") or "").strip()
    if not message:
        raise RuntimeError("nothing to show: message was empty")

    if sys.platform == "darwin":
        # Quotes inside the message would end the AppleScript string early.
        safe = message.replace('"', " ").replace("\\", " ")
        await shell("osascript", "-e", f'display notification "{safe}" with title "Pepebot"')
    elif sys.platform.startswith("win"):
        raise RuntimeError("desktop notifications are not wired up on Windows in this example")
    else:
        await shell("notify-send", "Pepebot", message)
    return f"shown on screen: {message}"


TOOLS = {
    "device_info": {
        "run": device_info,
        "description": "Get details about the machine the user is sitting at: hostname, operating system, timezone and local time.",
        "parameters": {"type": "object", "properties": {}},
    },
    "read_clipboard": {
        "run": read_clipboard,
        "description": "Read the text currently on the user's clipboard.",
        "parameters": {"type": "object", "properties": {}},
    },
    "notify": {
        "run": notify,
        "description": "Show a desktop notification on the user's screen. Use it when asked to remind or alert them.",
        "parameters": {
            "type": "object",
            "properties": {"message": {"type": "string", "description": "The text to show"}},
            "required": ["message"],
        },
    },
}


# --------------------------------------------------------------------- session
def discover(path: str, label: str) -> list:
    import urllib.request

    try:
        with urllib.request.urlopen(f"{PANIKI}{path}", timeout=8) as res:
            ids = [row["id"] for row in json.load(res).get("data", [])]
        print(f"-- {label}: {', '.join(ids[:4])}{' …' if len(ids) > 4 else ''}")
        return ids
    except Exception as err:
        print(f"-- could not read {path} ({err}); relying on the gateway's config")
        return []


async def handle_tool_call(ws, event: dict) -> None:
    name = event.get("name", "")
    args = event.get("arguments") or {}
    print(f"\n[tool_call] {name}({json.dumps(args)})")

    tool = TOOLS.get(name)
    if tool is None:
        await ws.send(json.dumps({
            "type": "tool_result", "call_id": event["call_id"],
            "error": f"no tool named {name} on this client",
        }))
        return

    try:
        output = await tool["run"](args)
        print(f"[tool_result] {output.splitlines()[0][:120]}")
        await ws.send(json.dumps({
            "type": "tool_result", "call_id": event["call_id"], "output": output,
        }))
    except Exception as err:
        print(f"[tool_error] {err}")
        await ws.send(json.dumps({
            "type": "tool_result", "call_id": event["call_id"], "error": str(err),
        }))


async def reader(ws, ready: asyncio.Event) -> None:
    async for raw in ws:
        event = json.loads(raw)

        if event.get("status") == "connected":
            print(f"-- proxy: {event['provider']} -> {event['model']}")
            print('-- try: "what is on my clipboard?" or "notify me that the build is done"')
            ready.set()
            continue

        # A call for one of our tools. Answered on its own task so a slow tool does
        # not block the events still arriving behind it.
        if event.get("type") == "tool_call":
            asyncio.create_task(handle_tool_call(ws, event))
            continue

        kind = event.get("type")
        if kind == "response.output_item.done":
            item = event.get("item") or {}
            if item.get("type") == "function_call" and not item.get("name", "").startswith(APP + "-"):
                print(f"\n[gateway tool] {item.get('name')}({item.get('arguments', '')})")
        elif kind == "response.audio_transcript.done":
            print(f"\nbot: {(event.get('transcript') or '').strip()}")
            print("> ", end="", flush=True)
        elif kind == "error":
            print(f"\n[server error] {event.get('error')}")


async def main() -> None:
    models = discover("/v1/models", "models")

    print(f"-- connecting to {GATEWAY}")
    async with websockets.connect(GATEWAY, max_size=None) as ws:
        setup = {
            "provider": "realtime",
            "agent": os.getenv("AGENT", "default"),
            "session_key": SESSION,
            "enable_tools": True,
            "language": LANGUAGE,
            # Ours to run. Pepebot namespaces them as "<APP>-<name>" so they cannot
            # shadow one of the agent's own, and asks us for them by the bare name.
            "app": APP,
            "tools": [
                {"name": name, "description": t["description"], "parameters": t["parameters"]}
                for name, t in TOOLS.items()
            ],
            "client_tool_timeout_ms": 20000,
        }
        if models:
            setup["model"] = models[0]
        await ws.send(json.dumps({"setup": setup}))
        print(f'-- session "{SESSION}" | declared as {APP}: ' + ", ".join(TOOLS))

        ready = asyncio.Event()
        pump = asyncio.create_task(reader(ws, ready))
        await ready.wait()

        loop = asyncio.get_running_loop()
        while not pump.done():
            line = (await loop.run_in_executor(None, sys.stdin.readline)).strip()
            if not line:
                if pump.done():
                    break
                continue
            await ws.send(json.dumps({
                "type": "conversation.item.create",
                "item": {"type": "message", "role": "user",
                         "content": [{"type": "input_text", "text": line}]},
            }))
            await ws.send(json.dumps({"type": "response.create"}))

        await pump


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\nbye")
