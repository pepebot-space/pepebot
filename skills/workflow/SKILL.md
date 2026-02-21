---
name: workflow
description: "Create and manage multi-step automation workflows combining any tools (ADB, shell, web, file) with variable interpolation."
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

### UI Tap with Coordinates
```json
{
  "name": "Tap Button",
  "description": "Tap specific coordinates on screen",
  "variables": {"device": "", "tap_x": "540", "tap_y": "1200"},
  "steps": [
    {"name": "screenshot_before", "tool": "adb_screenshot", "args": {"filename": "before_tap.png", "device": "{{device}}"}},
    {"name": "tap", "tool": "adb_tap", "args": {"x": "{{tap_x}}", "y": "{{tap_y}}", "device": "{{device}}"}},
    {"name": "wait", "tool": "adb_shell", "args": {"command": "sleep 2", "device": "{{device}}"}},
    {"name": "screenshot_after", "tool": "adb_screenshot", "args": {"filename": "after_tap.png", "device": "{{device}}"}}
  ]
}
```

### Skill-Enhanced Analysis
```json
{
  "name": "Smart Analysis",
  "description": "Use a skill to analyze device data",
  "variables": {"device": ""},
  "steps": [
    {"name": "collect", "tool": "adb_shell", "args": {"command": "dumpsys battery", "device": "{{device}}"}},
    {"name": "analyze", "skill": "workflow", "goal": "Analyze {{collect_output}} using workflow best practices"}
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

## Workflow Tools

- **`workflow_save`**: Save workflow JSON → `workflow_save(workflow_name, workflow_content)`
- **`workflow_execute`**: Run workflow → `workflow_execute(workflow_name, variables?)`
- **`workflow_list`**: List all workflows → `workflow_list()`

## Variable System

1. **Default variables**: Defined in `"variables"` field
2. **Runtime overrides**: Passed via `workflow_execute` variables parameter
3. **Step outputs**: Auto-created as `{{step_name_output}}`
4. **Goal results**: Auto-created as `{{step_name_goal}}`

## Notes

- Coordinate values (x, y, x1, y1, etc.) are auto-converted from strings to numbers
- Tool names and required parameters are validated on save
- `adb_ui_dump` may not work on all devices - use coordinates as fallback
