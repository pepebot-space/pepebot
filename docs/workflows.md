# Pepebot Workflows Documentation

## Table of Contents
- [Overview](#overview)
- [Core Concepts](#core-concepts)
- [Workflow Tools](#workflow-tools)
- [Workflow Structure](#workflow-structure)
- [Variable System](#variable-system)
- [Step Types](#step-types)
- [Complete Examples](#complete-examples)
- [Advanced Patterns](#advanced-patterns)
- [Tool Reference](#tool-reference)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

---

## Overview

Workflows are multi-step automation sequences that enable complex task orchestration in Pepebot. They combine multiple tools, handle data flow between steps, and support both deterministic tool execution and LLM-driven goal-based automation.

### Key Features

- **Universal Tool Support**: Works with ALL registered tools (ADB, filesystem, shell, web, etc.)
- **Smart Variable Interpolation**: Dynamic data flow between steps using `{{variable}}` syntax
- **Dual Execution Modes**: Tool steps (deterministic) and goal steps (LLM-driven)
- **Output Tracking**: Automatic capture and reuse of step outputs
- **Runtime Flexibility**: Override variables during execution
- **Pure JSON**: Simple, readable, version-controllable workflow definitions

### Use Cases

- **Mobile Testing**: Automated app testing and UI validation on Android devices
- **Device Management**: Health monitoring, diagnostics, and remote device control
- **Data Collection**: Multi-source data gathering with automated analysis
- **Complex Automation**: Chain multiple operations with conditional logic via LLM
- **Research Tasks**: Web search → analysis → report generation pipelines
- **Cross-Platform Workflows**: Combine web, filesystem, shell, and device automation

---

## Core Concepts

### 1. Workflow Definition
A JSON file describing the automation sequence with variables and steps.

### 2. Variables
Named values that can be:
- **Default values** in the workflow definition
- **Overridden** at execution time
- **Generated** from step outputs
- **Interpolated** using `{{variable_name}}` syntax

### 3. Steps
Individual actions executed sequentially:
- **Tool Steps**: Call a specific tool with arguments
- **Goal Steps**: Describe desired outcome in natural language for LLM
- **Skill Steps**: Load a skill's content and combine with a goal
- **Agent Steps**: Delegate a goal to another registered agent

### 4. Step Outputs
Results from each step automatically become available as variables:
- Tool step output: `{{step_name_output}}`
- Goal step result: `{{step_name_goal}}`

---

## Workflow Tools

Pepebot provides three tools for workflow management:

### workflow_execute

Execute a saved workflow from the workspace.

**Parameters:**
```json
{
  "workflow_name": "string (required)",
  "variables": {
    "key": "value"  // optional overrides
  }
}
```

**Example:**
```json
{
  "workflow_name": "device_health",
  "variables": {
    "device": "abc123",
    "alert_threshold": 20
  }
}
```

### workflow_save

Create and save a new workflow definition.

**Parameters:**
```json
{
  "workflow_name": "string (required)",
  "workflow_content": "string (required, JSON as string)"
}
```

**Example:**
```json
{
  "workflow_name": "quick_check",
  "workflow_content": "{\"name\":\"Quick Check\",\"description\":\"Fast device check\",\"variables\":{\"device\":\"auto\"},\"steps\":[{\"name\":\"battery\",\"tool\":\"adb_shell\",\"args\":{\"command\":\"dumpsys battery\",\"device\":\"{{device}}\"}}]}"
}
```

### workflow_list

List all available workflows in the workspace.

**Parameters:** None

**Example Usage:**
```
User: "What workflows do we have?"
Agent: [Calls workflow_list tool]
```

### adb_record_workflow

Record user interactions on an Android device and generate a workflow JSON file. See [ADB Activity Recorder](#adb-activity-recorder) for full documentation.

**Parameters:**
```json
{
  "workflow_name": "string (required)",
  "description": "string (optional)",
  "device": "string (optional)",
  "max_duration": "number (optional, default: 300)"
}
```

**Example:**
```json
{
  "workflow_name": "my_recording",
  "description": "Login flow recording"
}
```

---

## Workflow CLI (Standalone Execution)

Workflows can be run directly from the terminal without the agent. This is ideal for cron jobs, shell scripts, CI/CD pipelines, and headless automation.

### Commands

```bash
# List all workflows in workspace
pepebot workflow list

# Show full details (description, variables, steps with types)
pepebot workflow show <name>

# Run workflow from workspace
pepebot workflow run <name>

# Run with variable overrides (repeatable flag)
pepebot workflow run <name> --var device=emulator-5554 --var query=hello

# Run directly from any JSON file (bypass workspace lookup)
pepebot workflow run -f /path/to/workflow.json
pepebot workflow run -f /path/to/workflow.json --var key=value

# Validate workflow structure and tool parameters
pepebot workflow validate <name>
pepebot workflow validate -f /path/to/workflow.json

# Delete a workflow from workspace
pepebot workflow delete <name>
```

### The `-f` Flag

The `-f` (or `--file`) flag tells `pepebot workflow run` (and `validate`) to load from an arbitrary file path instead of the workspace `~/.pepebot/workspace/workflows/` directory. Use cases:

- **Iterate on new workflows** before saving them to workspace
- **Run workflows from external repos** or shared directories
- **CI/CD pipelines** that store workflow JSON alongside application code
- **One-off automation** scripts that generate workflow files dynamically

### CLI vs Agent Execution

| Feature | CLI (`pepebot workflow run`) | Agent (`workflow_execute` tool) |
|---------|----------------------------|-------------------------------|
| Tool steps | Executed directly | Executed directly |
| Goal steps | Logged, not interpreted | LLM interprets and acts |
| Skill steps | Content loaded as variable | Content loaded + LLM processes |
| Agent steps | Not available (no gateway) | Delegates to other agents |
| Variable overrides | `--var key=value` flags | `variables` JSON parameter |
| Speed | Fast (no LLM overhead) | Slower (LLM calls per goal step) |
| Best for | Cron, scripts, CI/CD, headless | Chat, interactive, LLM-driven tasks |

### Cron Scheduling

Schedule tool-only workflows to run automatically:

```bash
# Device health check every hour
0 * * * * /usr/local/bin/pepebot workflow run device_health --var device=emulator-5554

# Daily report with log file
0 8 * * * /usr/local/bin/pepebot workflow run daily_report > /var/log/pepebot/daily_$(date +\%F).log 2>&1
```

Systemd timer alternative:
```ini
# /etc/systemd/system/pepebot-health.timer
[Timer]
OnCalendar=*:0/30

# /etc/systemd/system/pepebot-health.service
[Service]
ExecStart=/usr/local/bin/pepebot workflow run device_health --var device=emulator-5554
```

### Shell Scripting

```bash
#!/bin/bash
# Run across multiple devices
for device in emulator-5554 emulator-5556 192.168.1.100:5555; do
  echo "=== $device ==="
  pepebot workflow run device_health --var device="$device"
done
```

```bash
#!/bin/bash
# Validate before running
pepebot workflow validate smoke_test || { echo "Invalid workflow!"; exit 1; }
pepebot workflow run smoke_test --var device="$DEVICE" --var build="$BUILD"
```

### CI/CD Integration

```bash
# Run workflow JSON from the repo (not from workspace)
pepebot workflow run -f ./ci/workflows/integration_test.json --var env=staging --var build=$CI_COMMIT_SHA
```

### Design Tips for Standalone Workflows

- **Use only tool steps** for workflows meant to run via cron/scripts (no LLM needed)
- **Add goal/agent steps** only when LLM reasoning is required
- **Nest workflows** using `workflow_execute` as a tool step for modular automation
- **Validate first** with `pepebot workflow validate` before deploying to cron
- **Log output** by redirecting stdout to files for audit trails
- **Use `--var` with shell variables** for dynamic configuration: `--var device="$DEVICE_SERIAL"`

---

## Workflow Structure

### Basic Template

```json
{
  "name": "Workflow Display Name",
  "description": "What this workflow accomplishes",
  "variables": {
    "var1": "default_value",
    "var2": "another_value"
  },
  "steps": [
    {
      "name": "step_identifier",
      "tool": "tool_name",
      "args": {
        "param": "{{variable}}"
      }
    }
  ]
}
```

### Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Human-readable workflow name |
| `description` | string | Yes | Clear explanation of purpose |
| `variables` | object | No | Default variable values |
| `steps` | array | Yes | Ordered list of steps to execute |

### Step Structure

#### Tool Step
```json
{
  "name": "unique_step_name",
  "tool": "registered_tool_name",
  "args": {
    "arg1": "value or {{variable}}",
    "arg2": 123
  }
}
```

#### Goal Step
```json
{
  "name": "unique_step_name",
  "goal": "Natural language description of what to accomplish"
}
```

#### Skill Step
```json
{
  "name": "unique_step_name",
  "skill": "skill_name",
  "goal": "Goal that uses the loaded skill content"
}
```

#### Agent Step
```json
{
  "name": "unique_step_name",
  "agent": "agent_name",
  "goal": "Goal to delegate to another agent"
}
```

---

## Variable System

### Variable Sources

1. **Default Variables** (in workflow definition)
2. **Override Variables** (passed to workflow_execute)
3. **Step Outputs** (automatically captured)
4. **Goal Results** (from LLM goal steps)

### Interpolation Syntax

Variables are replaced using `{{variable_name}}` anywhere in the workflow:

```json
{
  "variables": {
    "device": "emulator-5554",
    "username": "test@example.com"
  },
  "steps": [
    {
      "name": "check_device",
      "tool": "adb_shell",
      "args": {
        "command": "getprop ro.build.version.release",
        "device": "{{device}}"
      }
    },
    {
      "name": "log_result",
      "tool": "write_file",
      "args": {
        "path": "device_info.txt",
        "content": "Device {{device}} is running Android {{check_device_output}}"
      }
    }
  ]
}
```

### Variable Precedence

1. **Override variables** (highest priority)
2. **Step outputs** (generated during execution)
3. **Default variables** (from workflow definition)

### Type Coercion

The workflow engine automatically converts string variables to appropriate types:

```json
{
  "variables": {
    "x": "540",      // String in definition
    "y": "960"
  },
  "steps": [
    {
      "name": "tap",
      "tool": "adb_tap",
      "args": {
        "x": "{{x}}",  // Automatically converted to float64
        "y": "{{y}}",  // Automatically converted to float64
        "device": "{{device}}"
      }
    }
  ]
}
```

---

## Step Types

### Tool Steps

Execute a registered tool with specific arguments.

**Characteristics:**
- Deterministic behavior
- Direct tool invocation
- Predictable output format
- Fast execution

**Example:**
```json
{
  "name": "capture_screen",
  "tool": "adb_screenshot",
  "args": {
    "filename": "screen_{{timestamp}}.png",
    "device": "{{device}}"
  }
}
```

**Output Access:**
```json
{
  "name": "next_step",
  "tool": "write_file",
  "args": {
    "path": "log.txt",
    "content": "Screenshot saved: {{capture_screen_output}}"
  }
}
```

### Goal Steps

Describe desired outcome in natural language for the LLM to interpret and act upon.

**Characteristics:**
- Flexible, adaptive behavior
- LLM interprets and executes
- Can use any available tools
- Handles complex logic

**Example:**
```json
{
  "name": "analyze_ui",
  "goal": "Find the login button in the UI dump from step 'get_ui' and extract its center coordinates. Store them as 'button_x' and 'button_y' variables."
}
```

**Output Access:**
```json
{
  "name": "tap_button",
  "tool": "adb_tap",
  "args": {
    "x": "{{button_x}}",
    "y": "{{button_y}}",
    "device": "{{device}}"
  }
}
```

### Skill Steps

Load skill content and combine with a goal. The skill's content is loaded and stored along with the goal as `{{step_name_output}}`.

**Characteristics:**
- Loads skill definition content at execution time
- Combines skill context with a goal instruction
- Requires both `skill` and `goal` fields
- Cannot be combined with `tool` or `agent`

**Example:**
```json
{
  "name": "analyze_with_skill",
  "skill": "workflow",
  "goal": "Using the workflow skill knowledge, analyze the output from step 'collect_data' and suggest improvements."
}
```

### Agent Steps

Delegate a goal to another registered agent. The agent processes the goal independently with an ephemeral session and returns a response as `{{step_name_output}}`.

**Characteristics:**
- Delegates to a different agent (different model, prompt, capabilities)
- Uses ephemeral session key per workflow execution
- Requires both `agent` and `goal` fields
- Cannot be combined with `tool` or `skill`
- Not available in standalone mode (CLI without gateway)

**Example:**
```json
{
  "name": "research_topic",
  "agent": "researcher",
  "goal": "Research the latest trends in {{topic}} and provide a concise summary with sources."
}
```

**Output Access:**
```json
{
  "name": "save_research",
  "tool": "write_file",
  "args": {
    "path": "research_results.txt",
    "content": "{{research_topic_output}}"
  }
}
```

### Choosing Between Step Types

| Use Tool Step When... | Use Goal Step When... | Use Skill Step When... | Use Agent Step When... |
|----------------------|----------------------|----------------------|----------------------|
| You know exact tool and parameters | Logic requires analysis or decision | Need specialized knowledge from a skill | Need a different agent's capabilities |
| Need deterministic behavior | Need adaptive behavior | Want to augment a goal with skill context | Want model/prompt specialization |
| Performance is critical | Flexibility is critical | Skill provides domain-specific instructions | Task suits a different agent's expertise |
| Simple, direct operation | Complex, multi-condition operation | Combining skill + LLM reasoning | Cross-agent collaboration |

---

## Complete Examples

### Example 1: Android App Login Automation

**Workflow:** `ui_automation.json`

```json
{
  "name": "Login Automation",
  "description": "Automate app login with username/password",
  "variables": {
    "device": "emulator-5554",
    "username": "demo@example.com",
    "password": "demo123"
  },
  "steps": [
    {
      "name": "get_ui",
      "tool": "adb_ui_dump",
      "args": {
        "device": "{{device}}"
      }
    },
    {
      "name": "analyze_ui",
      "goal": "Find username field coordinates in UI dump from step 'get_ui'. Store the coordinates as variables 'username_x' and 'username_y'."
    },
    {
      "name": "tap_username",
      "tool": "adb_tap",
      "args": {
        "x": "{{username_x}}",
        "y": "{{username_y}}",
        "device": "{{device}}"
      }
    },
    {
      "name": "input_username",
      "tool": "adb_input_text",
      "args": {
        "text": "{{username}}",
        "device": "{{device}}"
      }
    },
    {
      "name": "find_password_field",
      "goal": "Find password field coordinates in the UI. Store as 'password_x' and 'password_y'."
    },
    {
      "name": "tap_password",
      "tool": "adb_tap",
      "args": {
        "x": "{{password_x}}",
        "y": "{{password_y}}",
        "device": "{{device}}"
      }
    },
    {
      "name": "input_password",
      "tool": "adb_input_text",
      "args": {
        "text": "{{password}}",
        "device": "{{device}}"
      }
    },
    {
      "name": "verify",
      "tool": "adb_screenshot",
      "args": {
        "filename": "login_result.png",
        "device": "{{device}}"
      }
    }
  ]
}
```

**Usage:**
```
User: "Login to the app with user john@example.com password secret123"
Agent: [Calls workflow_execute with overrides]
{
  "workflow_name": "ui_automation",
  "variables": {
    "username": "john@example.com",
    "password": "secret123"
  }
}
```

### Example 2: Device Health Monitoring

**Workflow:** `device_control.json`

```json
{
  "name": "Device Health Check",
  "description": "Check battery, memory, storage, and network status of Android device",
  "variables": {
    "device": "emulator-5554"
  },
  "steps": [
    {
      "name": "check_devices",
      "tool": "adb_devices",
      "args": {}
    },
    {
      "name": "battery",
      "tool": "adb_shell",
      "args": {
        "command": "dumpsys battery",
        "device": "{{device}}"
      }
    },
    {
      "name": "memory",
      "tool": "adb_shell",
      "args": {
        "command": "free -h",
        "device": "{{device}}"
      }
    },
    {
      "name": "storage",
      "tool": "adb_shell",
      "args": {
        "command": "df -h",
        "device": "{{device}}"
      }
    },
    {
      "name": "network",
      "tool": "adb_shell",
      "args": {
        "command": "ip addr show",
        "device": "{{device}}"
      }
    },
    {
      "name": "report",
      "goal": "Analyze the health data from previous steps (battery, memory, storage, network) and generate a concise summary report with key metrics and any warnings."
    }
  ]
}
```

**Usage:**
```
User: "Check health of device abc123"
Agent: [Calls workflow_execute]
{
  "workflow_name": "device_control",
  "variables": {
    "device": "abc123"
  }
}
```

### Example 3: Web Research Pipeline

**Workflow:** `browser_automation.json`

```json
{
  "name": "Browser Research Workflow",
  "description": "Example workflow demonstrating non-ADB automation using web search and file operations",
  "variables": {
    "search_topic": "Android ADB debugging tips",
    "output_file": "research_results.txt"
  },
  "steps": [
    {
      "name": "search_web",
      "tool": "web_search",
      "args": {
        "query": "{{search_topic}}"
      }
    },
    {
      "name": "analyze_results",
      "goal": "Review the search results and identify the top 3 most relevant links. Format them as a summary."
    },
    {
      "name": "save_summary",
      "tool": "write_file",
      "args": {
        "path": "{{output_file}}",
        "content": "Research Topic: {{search_topic}}\n\n{{analyze_results_goal}}\n\nCompleted at: {{timestamp}}"
      }
    },
    {
      "name": "confirm",
      "goal": "Read the saved file and confirm the research summary was saved successfully."
    }
  ]
}
```

### Example 4: Multichannel Notification

**Workflow:** `broadcast_message.json`

```json
{
  "name": "Broadcast Message",
  "description": "Send a notification message to multiple chat platforms simultaneously",
  "variables": {
    "message": "Hello from Pepebot workflow!",
    "telegram_chat_id": "123456789",
    "discord_channel_id": "987654321098765432",
    "whatsapp_jid": "62812345678@s.whatsapp.net"
  },
  "steps": [
    {
      "name": "notify_telegram",
      "tool": "telegram_send",
      "args": {
        "text": "Telegram Broadcast: {{message}}",
        "chat_id": "{{telegram_chat_id}}"
      }
    },
    {
      "name": "notify_discord",
      "tool": "discord_send",
      "args": {
        "content": "Discord Broadcast: {{message}}",
        "channel_id": "{{discord_channel_id}}"
      }
    },
    {
      "name": "notify_whatsapp",
      "tool": "whatsapp_send",
      "args": {
        "text": "WhatsApp Broadcast: {{message}}",
        "jid": "{{whatsapp_jid}}"
      }
    }
  ]
}
```

**Usage:**
```
# Run directly from the CLI to broadcast a message
pepebot workflow run broadcast_message \
  --var message="The server backup completed successfully" \
  --var telegram_chat_id="your_admin_chat_id"
```

---

## Advanced Patterns

### Pattern 1: Conditional Logic via Goal Steps

```json
{
  "name": "adaptive_test",
  "description": "Test with conditional behavior based on device state",
  "variables": {
    "device": "auto"
  },
  "steps": [
    {
      "name": "check_battery",
      "tool": "adb_shell",
      "args": {
        "command": "dumpsys battery | grep level",
        "device": "{{device}}"
      }
    },
    {
      "name": "decide_action",
      "goal": "If battery level from 'check_battery' is below 20%, skip intensive tests and just take a screenshot. Otherwise, proceed with full UI testing. Store decision as 'action' variable (either 'skip' or 'full')."
    },
    {
      "name": "execute_based_on_decision",
      "goal": "Based on 'action' variable: if 'skip', take screenshot; if 'full', run complete UI dump and analysis."
    }
  ]
}
```

### Pattern 2: Data Aggregation and Reporting

```json
{
  "name": "performance_report",
  "description": "Collect multiple metrics and generate unified report",
  "variables": {
    "device": "auto",
    "report_file": "performance_report.md"
  },
  "steps": [
    {
      "name": "cpu_usage",
      "tool": "adb_shell",
      "args": {
        "command": "top -n 1",
        "device": "{{device}}"
      }
    },
    {
      "name": "memory_info",
      "tool": "adb_shell",
      "args": {
        "command": "cat /proc/meminfo",
        "device": "{{device}}"
      }
    },
    {
      "name": "process_list",
      "tool": "adb_shell",
      "args": {
        "command": "ps -A",
        "device": "{{device}}"
      }
    },
    {
      "name": "analyze_all",
      "goal": "Analyze cpu_usage, memory_info, and process_list outputs. Create a markdown report with: 1) Top CPU consumers, 2) Memory usage percentage, 3) Process count, 4) Recommendations."
    },
    {
      "name": "save_report",
      "tool": "write_file",
      "args": {
        "path": "{{report_file}}",
        "content": "# Performance Report - {{device}}\n\n{{analyze_all_goal}}"
      }
    }
  ]
}
```

### Pattern 3: Iterative Testing with Screenshots

```json
{
  "name": "ui_validation",
  "description": "Test UI states with visual verification",
  "variables": {
    "device": "auto",
    "app_package": "com.example.app"
  },
  "steps": [
    {
      "name": "launch_app",
      "tool": "adb_shell",
      "args": {
        "command": "monkey -p {{app_package}} 1",
        "device": "{{device}}"
      }
    },
    {
      "name": "wait_for_launch",
      "tool": "adb_shell",
      "args": {
        "command": "sleep 3",
        "device": "{{device}}"
      }
    },
    {
      "name": "screen_1",
      "tool": "adb_screenshot",
      "args": {
        "filename": "screen_1_launch.png",
        "device": "{{device}}"
      }
    },
    {
      "name": "ui_state_1",
      "tool": "adb_ui_dump",
      "args": {
        "device": "{{device}}"
      }
    },
    {
      "name": "find_next_button",
      "goal": "Analyze ui_state_1 and find the 'Next' or 'Continue' button. Extract coordinates as next_x and next_y."
    },
    {
      "name": "tap_next",
      "tool": "adb_tap",
      "args": {
        "x": "{{next_x}}",
        "y": "{{next_y}}",
        "device": "{{device}}"
      }
    },
    {
      "name": "screen_2",
      "tool": "adb_screenshot",
      "args": {
        "filename": "screen_2_next.png",
        "device": "{{device}}"
      }
    },
    {
      "name": "verify_navigation",
      "goal": "Compare screen_1 and screen_2 screenshots to confirm navigation occurred successfully."
    }
  ]
}
```

### Pattern 4: Cross-Tool Automation

```json
{
  "name": "research_and_test",
  "description": "Combine web research with device testing",
  "variables": {
    "app_name": "Example App",
    "device": "auto"
  },
  "steps": [
    {
      "name": "search_docs",
      "tool": "web_search",
      "args": {
        "query": "{{app_name}} testing best practices"
      }
    },
    {
      "name": "extract_tips",
      "goal": "From search_docs results, identify top 3 testing recommendations and store as 'test_tips'."
    },
    {
      "name": "save_tips",
      "tool": "write_file",
      "args": {
        "path": "testing_guide.txt",
        "content": "{{test_tips}}"
      }
    },
    {
      "name": "run_device_tests",
      "goal": "Based on the test_tips, execute appropriate ADB commands to test the app on device {{device}}."
    },
    {
      "name": "final_screenshot",
      "tool": "adb_screenshot",
      "args": {
        "filename": "test_complete.png",
        "device": "{{device}}"
      }
    }
  ]
}
```

---

## Tool Reference

### ADB Tools (Android)

Available when `adb` binary is detected:

#### adb_devices
List all connected Android devices.

**Parameters:** None

**Output:** List of device serials and states

#### adb_shell
Execute shell commands on Android device.

**Parameters:**
- `command` (string, required): Shell command to execute
- `device` (string, optional): Target device serial

**Output:** Command stdout

#### adb_tap
Tap screen coordinates.

**Parameters:**
- `x` (number, required): X coordinate
- `y` (number, required): Y coordinate
- `device` (string, optional): Target device serial

**Output:** Success confirmation

#### adb_input_text
Input text into focused field.

**Parameters:**
- `text` (string, required): Text to input
- `device` (string, optional): Target device serial

**Output:** Success confirmation

#### adb_screenshot
Capture device screenshot.

**Parameters:**
- `filename` (string, required): Output filename (saved in workspace)
- `device` (string, optional): Target device serial

**Output:** File path

#### adb_ui_dump
Get UI hierarchy XML dump.

**Parameters:**
- `device` (string, optional): Target device serial

**Output:** XML UI hierarchy

#### adb_swipe
Perform swipe gesture.

**Parameters:**
- `x1` (number, required): Start X
- `y1` (number, required): Start Y
- `x2` (number, required): End X
- `y2` (number, required): End Y
- `duration` (number, optional): Swipe duration in ms
- `device` (string, optional): Target device serial

**Output:** Success confirmation

### File Tools

#### read_file
Read file contents.

**Parameters:**
- `path` (string, required): File path (absolute or relative to workspace)

**Output:** File contents as string

#### write_file
Write content to file.

**Parameters:**
- `path` (string, required): File path (absolute or relative to workspace)
- `content` (string, required): Content to write

**Output:** Success confirmation

#### list_dir
List directory contents.

**Parameters:**
- `path` (string, required): Directory path (absolute or relative to workspace)

**Output:** List of files and directories

#### edit_file
Edit file using search and replace.

**Parameters:**
- `path` (string, required): File path
- `old_content` (string, required): Content to find
- `new_content` (string, required): Replacement content

**Output:** Success confirmation

### Web Tools

#### web_search
Search the web using Brave API.

**Parameters:**
- `query` (string, required): Search query

**Output:** Search results with titles, URLs, and snippets

#### web_fetch
Fetch and extract content from URL.

**Parameters:**
- `url` (string, required): URL to fetch

**Output:** Extracted text content

### Shell Tool

#### exec
Execute shell commands on the host system.

**Parameters:**
- `command` (string, required): Shell command to execute

**Output:** Command stdout

---

## Best Practices

### 1. Workflow Design

**Use Descriptive Names**
```json
{
  "name": "login_flow_test",  // Good: clear purpose
  "steps": [
    {
      "name": "tap_username_field",  // Good: specific action
      "tool": "adb_tap"
    }
  ]
}
```

```json
{
  "name": "test1",  // Bad: unclear purpose
  "steps": [
    {
      "name": "step1",  // Bad: non-descriptive
      "tool": "adb_tap"
    }
  ]
}
```

**Document Thoroughly**
```json
{
  "name": "User Registration Flow",
  "description": "Complete user registration: form filling, validation, submission, and verification. Requires empty database state.",
  "variables": {
    "email": "test@example.com",  // User email for registration
    "password": "Test123!",       // Must meet complexity requirements
    "device": "emulator-5554"     // Target test device
  }
}
```

**Keep Steps Atomic**
```json
// Good: Each step does one thing
{
  "steps": [
    {"name": "get_ui", "tool": "adb_ui_dump"},
    {"name": "find_button", "goal": "Locate login button"},
    {"name": "tap_button", "tool": "adb_tap"}
  ]
}

// Bad: Step tries to do too much
{
  "steps": [
    {
      "name": "do_everything",
      "goal": "Get UI, find login button, tap it, wait for response, and verify success"
    }
  ]
}
```

### 2. Variable Management

**Use Meaningful Variable Names**
```json
{
  "variables": {
    "login_button_x": 540,           // Good: purpose is clear
    "login_button_y": 960,
    "max_retry_count": 3,
    "screenshot_output_dir": "test_results/"
  }
}
```

**Provide Default Values**
```json
{
  "variables": {
    "device": "emulator-5554",        // Sensible default
    "timeout": 30,                    // Reasonable timeout
    "retry_on_failure": true          // Safe default behavior
  }
}
```

**Document Expected Overrides**
```json
{
  "name": "Multi-Device Test",
  "description": "Tests app on specified device. Override 'device' variable with target serial.",
  "variables": {
    "device": "emulator-5554",  // DEFAULT: Override with actual device serial
    "app_package": "com.example.app"
  }
}
```

### 3. Error Handling

**Use Goal Steps for Validation**
```json
{
  "steps": [
    {
      "name": "perform_action",
      "tool": "adb_tap",
      "args": {"x": 500, "y": 500}
    },
    {
      "name": "verify_success",
      "goal": "Take a screenshot and verify the action succeeded. If it failed, describe what went wrong."
    }
  ]
}
```

**Include Checkpoints**
```json
{
  "steps": [
    {
      "name": "check_device_connected",
      "tool": "adb_devices"
    },
    {
      "name": "verify_device",
      "goal": "Confirm that device {{device}} appears in the device list. If not, stop and report error."
    },
    // ... rest of workflow
  ]
}
```

### 4. Performance Optimization

**Minimize Goal Steps**
```json
// Good: Goal steps only where needed
{
  "steps": [
    {"name": "get_ui", "tool": "adb_ui_dump"},         // Fast tool
    {"name": "analyze", "goal": "Find button"},        // LLM analysis
    {"name": "tap", "tool": "adb_tap"}                 // Fast tool
  ]
}

// Bad: Unnecessary goal step
{
  "steps": [
    {"name": "get_ui", "tool": "adb_ui_dump"},
    {"name": "tap", "goal": "Tap at coordinates 500,500"}  // Should use adb_tap tool
  ]
}
```

**Batch Related Operations**
```json
{
  "steps": [
    {
      "name": "collect_all_data",
      "goal": "Collect battery status, memory info, and storage data using adb_shell commands."
    },
    {
      "name": "analyze_once",
      "goal": "Analyze all collected data and generate report."
    }
  ]
}
```

### 5. Maintainability

**Version Control Friendly**
- Use consistent formatting (2-space indentation)
- Add comments via description fields
- Keep workflows under 50 steps
- Split complex workflows into sub-workflows

**Modular Design**
```json
// Core workflow
{
  "name": "Full Test Suite",
  "steps": [
    {"name": "login", "tool": "workflow_execute", "args": {"workflow_name": "login_flow"}},
    {"name": "test_features", "tool": "workflow_execute", "args": {"workflow_name": "feature_tests"}},
    {"name": "logout", "tool": "workflow_execute", "args": {"workflow_name": "logout_flow"}}
  ]
}
```

**Reusable Patterns**
```json
// Save as template for similar workflows
{
  "name": "Template: Data Collection",
  "description": "TEMPLATE: Collect data from device, analyze, and save report. Customize steps 1-3 for specific data.",
  "variables": {
    "device": "auto",
    "output_file": "report.txt"
  },
  "steps": [
    {"name": "collect_1", "tool": "adb_shell", "args": {"command": "CUSTOMIZE_ME"}},
    {"name": "collect_2", "tool": "adb_shell", "args": {"command": "CUSTOMIZE_ME"}},
    {"name": "analyze", "goal": "Analyze collected data"},
    {"name": "save", "tool": "write_file", "args": {"path": "{{output_file}}"}}
  ]
}
```

---

## Troubleshooting

### Workflow Not Found

**Symptom:** `workflow not found: xyz`

**Solutions:**
1. Check file exists: `ls ~/.pepebot/workspace/workflows/xyz.json`
2. Verify workflow name (case-sensitive, no .json extension)
3. Use `workflow_list` tool to see available workflows
4. Check file permissions: `chmod 644 workflow_file.json`

### Variable Not Interpolated

**Symptom:** Literal `{{variable}}` appears in output

**Solutions:**
1. Verify variable is defined in workflow or previous step
2. Check spelling (case-sensitive: `{{Device}}` ≠ `{{device}}`)
3. Ensure variable is set before it's used
4. Check for typos in step names for output variables

**Example:**
```json
// Wrong: Typo in step name
{
  "steps": [
    {"name": "get_data", "tool": "adb_shell"},
    {"name": "use_data", "tool": "write_file", "args": {
      "content": "{{get_date_output}}"  // Typo: 'date' instead of 'data'
    }}
  ]
}
```

### Tool Execution Failed

**Symptom:** Tool returns error or unexpected result

**Solutions:**
1. Verify tool is registered: Check tool appears in `workflow_list` or agent capabilities
2. Check argument types match tool schema
3. Test tool individually before using in workflow
4. Review tool documentation for parameter requirements

**Common Issues:**
```json
// Wrong: String instead of number
{"x": "500", "y": "960"}  // May cause issues with some tools

// Right: Proper types
{"x": 500, "y": 960}

// Right: Variable that will be coerced
{"x": "{{tap_x}}", "y": "{{tap_y}}"}  // Workflow engine handles conversion
```

### ADB Tools Not Available

**Symptom:** ADB tools don't appear or fail to execute

**Solutions:**
1. Install Android Platform Tools
2. Add ADB to PATH: `export PATH=$PATH:$ANDROID_HOME/platform-tools`
3. Verify ADB works: `adb devices`
4. Check device connection: `adb devices` should show your device
5. Enable USB debugging on Android device
6. Restart ADB server: `adb kill-server && adb start-server`

### Goal Step Not Working as Expected

**Symptom:** Goal step produces unexpected results or fails

**Solutions:**
1. Make goals more specific and explicit
2. Reference exact step names for outputs: "from step 'get_ui'"
3. Explicitly state what variables to create
4. Break complex goals into multiple simpler goals
5. Include context about previous steps

**Example:**
```json
// Vague goal
{
  "name": "do_analysis",
  "goal": "Find the button"
}

// Specific goal
{
  "name": "locate_login_button",
  "goal": "Analyze the UI dump from step 'get_ui' and find the element with text 'Login' or resource-id containing 'login_button'. Extract its center coordinates and store as 'login_x' and 'login_y' integer variables."
}
```

### Workflow Execution Timeout

**Symptom:** Workflow stops partway through or times out

**Solutions:**
1. Reduce number of goal steps (they take longer)
2. Split long workflows into smaller workflows
3. Optimize slow operations (e.g., reduce screenshot count)
4. Check for infinite loops in goal logic
5. Monitor LLM token usage in goal steps

### Variable Type Mismatch

**Symptom:** Tool complains about wrong parameter type

**Solutions:**
1. Let workflow engine handle coercion for numeric types
2. Use explicit type conversion in goal steps
3. Verify output format from previous step

**Example:**
```json
// If step outputs "540" (string) but tool needs number:
{
  "name": "convert_and_tap",
  "goal": "Convert {{screen_x}} and {{screen_y}} to integers, then use adb_tap tool with those coordinates."
}
```

---

## FAQ

### Can workflows use skills?

Yes! Use skill steps to load skill content and combine it with a goal:

```json
{
  "name": "skill_analysis",
  "skill": "workflow",
  "goal": "Using the workflow skill knowledge, analyze this data: {{data_output}}"
}
```

The skill content is loaded and combined with the goal, stored as `{{step_name_output}}`.

### Can workflows delegate to other agents?

Yes! Use agent steps to delegate a goal to a different agent:

```json
{
  "name": "delegate_research",
  "agent": "researcher",
  "goal": "Research the topic '{{topic}}' and provide a summary."
}
```

The agent processes the goal independently and returns the response as `{{step_name_output}}`. Note: agent steps require the gateway (AgentManager); they are not available in standalone CLI mode.

### Can workflows call other workflows?

Not directly, but you can chain workflows using goal steps:

```json
{
  "name": "master_workflow",
  "steps": [
    {
      "name": "run_login",
      "goal": "Execute the 'login_flow' workflow using workflow_execute tool."
    },
    {
      "name": "run_test",
      "goal": "Execute the 'feature_test' workflow using workflow_execute tool."
    }
  ]
}
```

### Can I use conditional execution?

Yes, via goal steps that make decisions:

```json
{
  "name": "conditional_test",
  "steps": [
    {"name": "check", "tool": "adb_shell", "args": {"command": "..."}},
    {
      "name": "decide",
      "goal": "If check output contains 'ready', continue with testing. Otherwise, skip tests and just log status."
    }
  ]
}
```

### How do I debug workflows?

1. **Add checkpoints:**
```json
{"name": "checkpoint_1", "goal": "Log current state and variable values"}
```

2. **Take screenshots at key points:**
```json
{"name": "debug_screen", "tool": "adb_screenshot", "args": {"filename": "debug_step_5.png"}}
```

3. **Use descriptive step names:**
```json
{"name": "tap_username_field_at_540x960", "tool": "adb_tap"}
```

4. **Save intermediate data:**
```json
{"name": "save_debug", "tool": "write_file", "args": {
  "path": "debug_output.txt",
  "content": "Step 3 result: {{step_3_output}}"
}}
```

### What's the maximum workflow size?

**Technical limits:**
- No hard limit on step count
- JSON file size limited by filesystem
- LLM context window limits apply to goal steps

**Practical recommendations:**
- Keep workflows under 50 steps
- Split into sub-workflows if larger
- Minimize goal step complexity

### Can I run workflows in parallel?

Currently workflows execute sequentially. For parallel execution, use multiple workflow_execute calls:

```
User: "Run health check on devices A, B, and C"
Agent: [Makes 3 parallel workflow_execute calls with different device overrides]
```

---

## ADB Activity Recorder

The ADB Activity Recorder lets you generate workflows by performing actions on your Android device while Pepebot records them in real-time via ADB's `getevent` command. Instead of manually writing workflow JSON, you simply interact with your device and the recorder captures your taps and swipes.

### How It Works

1. The agent calls the `adb_record_workflow` tool
2. Pepebot discovers the touch input device and screen resolution
3. `getevent -l` streams raw touch events from the device
4. Touch events are parsed through a state machine that tracks BTN_TOUCH DOWN/UP, ABS_MT_POSITION_X/Y, and SYN_REPORT events
5. Raw coordinates are mapped to screen pixels using the device's input range
6. Gestures are classified as **taps** (short duration, small movement) or **swipes** (large displacement)
7. Press **Volume Down** on the device to stop recording
8. A final screenshot and UI dump are captured for verification
9. The workflow JSON is saved to `~/.pepebot/workspace/workflows/`

### Tool: adb_record_workflow

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `workflow_name` | string | Yes | Name for the workflow file |
| `description` | string | No | Description of what the workflow does |
| `device` | string | No | Device serial number (uses default if omitted) |
| `max_duration` | number | No | Maximum recording time in seconds (default: 300) |

**Example Usage:**
```
User: "Record my Android actions as a workflow named login_flow"
Agent: [Calls adb_record_workflow with workflow_name="login_flow"]
```

**Example Output:**
```json
{
  "workflow_name": "login_flow",
  "action_count": 5,
  "save_path": "/home/user/.pepebot/workspace/workflows/login_flow.json",
  "stopped_by_user": true,
  "screenshot_path": "/home/user/.pepebot/workspace/workflows/login_flow_final.png"
}
```

### Generated Workflow Format

The recorder generates a standard workflow with:
- **`adb_tap`** steps for tap gestures (short duration, small movement)
- **`adb_swipe`** steps for swipe gestures (large displacement)
- A **`{{device}}`** variable for device targeting during replay
- A final **`verify_final_state`** goal step with screenshot path and UI dump for LLM verification

```json
{
  "name": "login_flow",
  "description": "Recorded user actions from Android device",
  "variables": { "device": "" },
  "steps": [
    {
      "name": "action_1_tap",
      "tool": "adb_tap",
      "args": { "x": 540, "y": 960, "device": "{{device}}" }
    },
    {
      "name": "action_2_swipe",
      "tool": "adb_swipe",
      "args": { "x": 200, "y": 1500, "x2": 200, "y2": 800, "duration": 400, "device": "{{device}}" }
    },
    {
      "name": "verify_final_state",
      "goal": "Verify the final screen state matches the expected outcome. Final screen UI elements: [UI dump]. Screenshot saved at: workflows/login_flow_final.png"
    }
  ]
}
```

### Gesture Classification

| Gesture | Criteria |
|---------|----------|
| **Tap** | Movement < 30px AND duration < 300ms |
| **Swipe** | Movement >= 50px |
| **Ambiguous** | Treated as tap at average position |

Actions within 200ms of the previous action are debounced (discarded) to filter jitter.

### Tips

- **Keep movements deliberate**: Quick, intentional taps and swipes record best
- **Wait between actions**: Pause briefly between taps to avoid debounce filtering
- **Press Volume Down to stop**: This is the only way to end recording
- **Replay with variables**: Override the `device` variable when executing on a different device
- **Edit after recording**: The generated workflow is standard JSON — you can manually adjust coordinates or add goal steps

## Additional Resources

- **Example Workflows:** `workspace/workflows/examples/`
- **Tool Documentation:** See individual tool schemas in code
- **Skills:** Check `skills/` directory for skill-specific workflows
- **Community Workflows:** https://github.com/pepebot-space/workflows

---

**Document Version:** 1.2
**Last Updated:** 2026-02-22
**Pepebot Version:** 0.5.2
