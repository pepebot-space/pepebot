---
name: workflow
description: "Create and manage multi-step automation workflows combining any tools (ADB, shell, web, file) with variable interpolation. Supports standalone CLI execution, cron scheduling, and scripting."
metadata: {"pepebot":{"emoji":"🔄","requires":{},"platform":"all"}}
---

# Workflow Creation Skill

## JSON Structure

```json
{
  "name": "Workflow Name",
  "description": "What it does",
  "variables": {
    "device": "",
    "param": "default_value"
  },
  "steps": [
    {
      "name": "step_id",
      "tool": "tool_name",
      "args": {
        "param": "{{variable}}"
      }
    },
    {
      "name": "analysis",
      "goal": "Instruction for LLM to interpret"
    },
    {
      "name": "use_skill",
      "skill": "skill_name",
      "goal": "Goal that uses the loaded skill content"
    },
    {
      "name": "delegate",
      "agent": "agent_name",
      "goal": "Goal to delegate to another agent"
    }
  ]
}
```

## Rules

1. **Every tool step MUST have `"args": {}`** - even if empty. No exceptions.
2. **Every step needs `"name"`** - use snake_case identifiers.
3. **4 step types**: `tool`+`args`, `goal` only, `skill`+`goal`, `agent`+`goal`.
4. **`tool` cannot combine** with `skill` or `agent`; `skill` and `agent` are mutually exclusive.
5. **`skill` and `agent` require `goal`**.
6. **Use `{{variable}}` for interpolation** in args and goals.
7. **Step outputs** are auto-stored as `{{step_name_output}}`.

**IMPORTANT:** When user says "use skill X" or "with skill X", ALWAYS use a **skill step** (`"skill": "X", "goal": "..."`). Do NOT manually replicate the skill's commands via tool/exec steps. The skill step automatically loads the skill content and combines it with the goal.

## Available Tools

| Tool | Required Args | Description |
|------|--------------|-------------|
| `adb_devices` | *(none)* | List connected devices |
| `adb_shell` | `command`, `device`(opt) | Run shell on device |
| `adb_tap` | `x`, `y`, `device`(opt) | Tap coordinates (numbers) |
| `adb_input_text` | `text`, `device`(opt) | Type text |
| `adb_screenshot` | `filename`, `device`(opt) | Capture screen |
| `adb_ui_dump` | `device`(opt) | Get UI XML hierarchy |
| `adb_swipe` | `x1`,`y1`,`x2`,`y2`, `duration`(opt), `device`(opt) | Swipe gesture |
| `exec` | `command` | Run shell command |
| `read_file` | `path` | Read file |
| `write_file` | `path`, `content` | Write file |
| `web_search` | `query` | Search web |

## Workflow Tools (Agent)

- **`workflow_save`**: Save workflow JSON → `workflow_save(workflow_name, workflow_content)`
- **`workflow_execute`**: Run workflow → `workflow_execute(workflow_name, variables?)`
- **`workflow_list`**: List all workflows → `workflow_list()`

## Workflow CLI (Standalone)

Workflows can be executed directly from the terminal without the agent. This enables cron jobs, shell scripts, CI/CD, and headless automation.

```bash
# List workflows
pepebot workflow list

# Show workflow details (steps, variables, types)
pepebot workflow show <name>

# Run workflow from workspace
pepebot workflow run <name>

# Run with variable overrides (repeatable)
pepebot workflow run <name> --var device=emulator-5554 --var query=hello

# Run directly from any JSON file (bypass workspace)
pepebot workflow run -f /path/to/workflow.json
pepebot workflow run -f /path/to/workflow.json --var key=value

# Validate workflow structure
pepebot workflow validate <name>
pepebot workflow validate -f /path/to/workflow.json

# Delete a workflow
pepebot workflow delete <name>
```

### Key: `-f` flag
The `-f` (or `--file`) flag runs a workflow from any file path, bypassing workspace lookup. This is useful for:
- Testing new workflows before saving to workspace
- Running workflows from external repos or shared directories
- CI/CD pipelines that store workflows alongside code
- One-off automation scripts

### Standalone vs Agent Execution
| Feature | CLI (`pepebot workflow run`) | Agent (`workflow_execute`) |
|---------|----------------------------|--------------------------|
| Tool steps | Executed directly | Executed directly |
| Goal steps | Logged but not interpreted | LLM interprets and acts |
| Skill steps | Content loaded, stored as variable | Content loaded + LLM processes |
| Agent steps | Not available (no gateway) | Delegates to other agents |
| Variable overrides | `--var key=value` | `variables` parameter |
| Use case | Cron, scripts, CI/CD, headless | Chat, interactive, LLM-driven |

**Tip:** For tool-only workflows, CLI execution is faster and doesn't need an LLM. Design workflows with pure tool steps for standalone automation, and add goal/agent steps only when LLM reasoning is needed.

## Patterns

### Device Health Check
```json
{
  "name": "Device Health",
  "description": "Check battery, memory, storage",
  "variables": {"device": ""},
  "steps": [
    {"name": "battery", "tool": "adb_shell", "args": {"command": "dumpsys battery", "device": "{{device}}"}},
    {"name": "memory", "tool": "adb_shell", "args": {"command": "free -h", "device": "{{device}}"}},
    {"name": "storage", "tool": "adb_shell", "args": {"command": "df -h", "device": "{{device}}"}},
    {"name": "report", "goal": "Analyze battery, memory, storage data and generate health report"}
  ]
}
```

### App Launch & Screenshot
```json
{
  "name": "App Launcher",
  "description": "Launch app and capture screen",
  "variables": {"device": "", "package": "com.android.settings"},
  "steps": [
    {"name": "launch", "tool": "adb_shell", "args": {"command": "am start -n {{package}}/.Settings", "device": "{{device}}"}},
    {"name": "wait", "tool": "adb_shell", "args": {"command": "sleep 3", "device": "{{device}}"}},
    {"name": "capture", "tool": "adb_screenshot", "args": {"filename": "app_screen.png", "device": "{{device}}"}}
  ]
}
```

### Nested Workflow (Workflow Calling Workflow)
```json
{
  "name": "Full Test Suite",
  "description": "Run login, tests, and logout workflows in sequence",
  "variables": {"device": ""},
  "steps": [
    {"name": "login", "tool": "workflow_execute", "args": {"workflow_name": "login_flow"}},
    {"name": "test", "tool": "workflow_execute", "args": {"workflow_name": "feature_tests"}},
    {"name": "logout", "tool": "workflow_execute", "args": {"workflow_name": "logout_flow"}}
  ]
}
```

### Agent Delegation
```json
{
  "name": "Multi-Agent Task",
  "description": "Delegate research to another agent",
  "variables": {"topic": "AI trends"},
  "steps": [
    {"name": "research", "agent": "researcher", "goal": "Research {{topic}} and provide a summary"},
    {"name": "save", "tool": "write_file", "args": {"path": "research.txt", "content": "{{research_output}}"}}
  ]
}
```

## Standalone Automation Recipes

### Cron: Scheduled Device Health Check
```bash
# Run every hour — pure tool steps, no LLM needed
0 * * * * pepebot workflow run device_health --var device=emulator-5554

# Run daily report, save output to file
0 8 * * * pepebot workflow run daily_report --var device=abc123 > /var/log/pepebot/daily_$(date +\%F).log 2>&1
```

### Shell Script: Multi-Device Batch
```bash
#!/bin/bash
# Run a workflow across multiple devices
for device in emulator-5554 emulator-5556 abc123; do
  echo "=== Running on $device ==="
  pepebot workflow run device_health --var device="$device"
done
```

### Shell Script: Test Before Deploy
```bash
#!/bin/bash
# Validate and run before deploying
pepebot workflow validate smoke_test || exit 1
pepebot workflow run smoke_test --var device="$DEVICE" --var build="$BUILD_NUMBER"
```

### CI/CD: Run Workflow From Repo
```bash
# Run a workflow checked into the repo (not from workspace)
pepebot workflow run -f ./ci/workflows/integration_test.json --var env=staging
```

### Combine CLI + Script Output
```bash
#!/bin/bash
# Collect data with workflow, post-process with other tools
pepebot workflow run collect_metrics --var device=emulator-5554 > /tmp/metrics.txt
# Parse and send to monitoring
grep "battery" /tmp/metrics.txt | curl -X POST -d @- https://monitoring.example.com/api/metrics
```

### Systemd Timer (Alternative to Cron)
```ini
# /etc/systemd/system/pepebot-health.timer
[Timer]
OnCalendar=*:0/30
# Every 30 minutes

# /etc/systemd/system/pepebot-health.service
[Service]
ExecStart=/usr/local/bin/pepebot workflow run device_health --var device=emulator-5554
```

## Variable System

1. **Default variables**: Defined in `"variables"` field
2. **Runtime overrides**: Passed via `workflow_execute` variables parameter or `--var` flag in CLI
3. **Step outputs**: Auto-created as `{{step_name_output}}`
4. **Goal results**: Auto-created as `{{step_name_goal}}`

## Design Tips for Powerful Workflows

- **Tool-only workflows** run standalone without LLM — ideal for cron and scripts
- **Goal/agent steps** require the agent loop — use `pepebot agent -m "run workflow X"` for those
- **Nest workflows** using `workflow_execute` as a tool step to build modular test suites
- **Use `-f` flag** to iterate on workflow JSON files before saving to workspace
- **Validate before deploying** with `pepebot workflow validate` to catch errors early
- **Log output** by redirecting CLI output to files for audit trails
- **Combine `--var` with environment variables** in scripts: `--var device="$DEVICE_SERIAL"`

## Notes

- Coordinate values (x, y, x1, y1, etc.) are auto-converted from strings to numbers
- Tool names and required parameters are validated on save
- `adb_ui_dump` may not work on all devices - use coordinates as fallback
- Workflow files are stored in `~/.pepebot/workspace/workflows/` as JSON
