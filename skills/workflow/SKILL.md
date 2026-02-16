---
name: workflow
description: "Create and manage multi-step automation workflows for Android (ADB), web scraping, file operations, and system tasks. Combine any tools with variable interpolation and LLM-driven goal steps."
metadata: {"pepebot":{"emoji":"🔄","requires":{},"platform":"all"}}
---

# Workflow Creation Skill 🔄

Create powerful multi-step automation workflows that can execute any combination of tools. Workflows support variable interpolation, sequential execution, and LLM-driven goal-based steps.

## ⚠️ CRITICAL REQUIREMENT - READ THIS FIRST ⚠️

**EVERY TOOL STEP MUST HAVE AN `args` FIELD. NO EXCEPTIONS!**

```json
// ❌ WRONG - Will cause "command is required" error
{
  "name": "step_name",
  "tool": "adb_shell"
}

// ✅ CORRECT - Always include args field
{
  "name": "step_name",
  "tool": "adb_shell",
  "args": {
    "command": "your_command",
    "device": "{{device}}"
  }
}

// ✅ CORRECT - Even if no parameters, include empty args
{
  "name": "list_devices",
  "tool": "adb_devices",
  "args": {}
}
```

**If you forget the `args` field, the workflow WILL FAIL!**

---

## Overview

Workflows are JSON-based automation scripts that can:
- Execute **any registered tools** (ADB, shell, web, file operations)
- Pass data between steps via **variable interpolation**
- Include **LLM goal steps** for intelligent decision-making
- **Override variables** at runtime for flexibility
- Track **step outputs** for use in subsequent steps

## Available Workflow Tools

### 1. `workflow_save` - Create New Workflow
Save a workflow definition to JSON file.

**Parameters:**
- `workflow_name` (required): Name without .json extension
- `workflow_content` (required): JSON workflow definition as string

### 2. `workflow_execute` - Run Workflow
Execute a saved workflow with optional variable overrides.

**Parameters:**
- `workflow_name` (required): Name of workflow to execute
- `variables` (optional): Object with variable overrides

### 3. `workflow_list` - List Workflows
List all available workflows with metadata.

**Parameters:** None

---

## Workflow JSON Structure

```json
{
  "name": "Workflow Display Name",
  "description": "Clear description of what this workflow does",
  "variables": {
    "var1": "default_value",
    "var2": "another_value"
  },
  "steps": [
    {
      "name": "step_identifier",
      "tool": "tool_name",
      "args": {
        "param": "value or {{variable}}"
      }
    },
    {
      "name": "goal_step",
      "goal": "Natural language instruction for LLM"
    }
  ]
}
```

### Required Fields

- **name**: Display name (can contain spaces)
- **description**: What the workflow does
- **steps**: Array of step objects

### Optional Fields

- **variables**: Default variable values (can be overridden at runtime)

### Step Types

#### Tool Steps
Execute a specific tool with arguments:
```json
{
  "name": "capture_screen",
  "tool": "adb_screenshot",
  "args": {
    "filename": "screen.png",
    "device": "{{device}}"
  }
}
```

**Important:** Every tool step **must** have an `args` field, even if empty (`"args": {}`).

#### Goal Steps
Provide instructions for the LLM to interpret:
```json
{
  "name": "analyze_ui",
  "goal": "Find the login button coordinates in the UI dump and store as 'login_x' and 'login_y' variables"
}
```

---

## Creating Workflows - Best Practices

### ✅ DO:

1. **Always Include Args Field**
   ```json
   {
     "name": "list_devices",
     "tool": "adb_devices",
     "args": {}  // ✅ Required even if empty
   }
   ```

2. **Use Clear Step Names**
   ```json
   "name": "get_battery_level"  // ✅ Descriptive
   ```

3. **Define Default Variables**
   ```json
   "variables": {
     "device": "emulator-5554",  // ✅ Sensible defaults
     "output_dir": "screenshots"
   }
   ```

4. **Use Variable Interpolation**
   ```json
   "args": {
     "device": "{{device}}",  // ✅ Flexible
     "filename": "{{output_dir}}/screen_{{timestamp}}.png"
   }
   ```

5. **Write Clear Goal Instructions**
   ```json
   "goal": "Analyze the battery data from previous step and determine if device needs charging. Store result as 'needs_charging' variable (true/false)."
   ```

### ❌ DON'T:

1. **Missing Args Field**
   ```json
   {
     "name": "list_devices",
     "tool": "adb_devices"
     // ❌ Missing "args": {}
   }
   ```

2. **Vague Step Names**
   ```json
   "name": "step1"  // ❌ Not descriptive
   ```

3. **Hardcoded Values When Variables Exist**
   ```json
   "device": "emulator-5554"  // ❌ Use "{{device}}" instead
   ```

4. **Ambiguous Goals**
   ```json
   "goal": "Do something with the data"  // ❌ Too vague
   ```

---

## Common Workflow Patterns

### Pattern 1: Device Health Check
```json
{
  "name": "Device Health Monitor",
  "description": "Check battery, memory, storage, and generate report",
  "variables": {
    "device": ""
  },
  "steps": [
    {
      "name": "check_battery",
      "tool": "adb_shell",
      "args": {
        "command": "dumpsys battery",
        "device": "{{device}}"
      }
    },
    {
      "name": "check_memory",
      "tool": "adb_shell",
      "args": {
        "command": "free -h",
        "device": "{{device}}"
      }
    },
    {
      "name": "analyze_health",
      "goal": "Analyze battery and memory data. Generate a health report with status (OK/WARNING/CRITICAL) and recommendations."
    },
    {
      "name": "save_report",
      "tool": "write_file",
      "args": {
        "path": "health_report_{{timestamp}}.txt",
        "content": "{{analyze_health_goal}}"
      }
    }
  ]
}
```

### Pattern 2: App Automation
```json
{
  "name": "App Launcher & Screenshot",
  "description": "Launch app, wait for load, capture screen",
  "variables": {
    "device": "",
    "app_package": "com.android.settings",
    "app_activity": ".Settings"
  },
  "steps": [
    {
      "name": "launch_app",
      "tool": "adb_shell",
      "args": {
        "command": "am start -n {{app_package}}/{{app_activity}}",
        "device": "{{device}}"
      }
    },
    {
      "name": "wait_for_load",
      "tool": "adb_shell",
      "args": {
        "command": "sleep 3",
        "device": "{{device}}"
      }
    },
    {
      "name": "capture_screen",
      "tool": "adb_screenshot",
      "args": {
        "filename": "{{app_package}}_screen.png",
        "device": "{{device}}"
      }
    }
  ]
}
```

### Pattern 3: UI Element Interaction
```json
{
  "name": "UI Automation Flow",
  "description": "Find and interact with UI elements",
  "variables": {
    "device": "",
    "target_text": "Login"
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
      "name": "find_element",
      "goal": "Parse the UI dump from step 'get_ui' and find the element with text '{{target_text}}'. Extract its bounds coordinates and store as 'element_x' and 'element_y'."
    },
    {
      "name": "tap_element",
      "tool": "adb_tap",
      "args": {
        "x": "{{element_x}}",
        "y": "{{element_y}}",
        "device": "{{device}}"
      }
    },
    {
      "name": "verify",
      "tool": "adb_screenshot",
      "args": {
        "filename": "after_tap.png",
        "device": "{{device}}"
      }
    }
  ]
}
```

### Pattern 4: Multi-Tool Integration
```json
{
  "name": "Research & Document",
  "description": "Web research, device interaction, file operations",
  "variables": {
    "search_topic": "Android ADB tips",
    "device": ""
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
      "name": "summarize",
      "goal": "Summarize the top 3 search results from 'search_web' into a concise bulleted list."
    },
    {
      "name": "get_device_info",
      "tool": "adb_shell",
      "args": {
        "command": "getprop ro.build.version.release",
        "device": "{{device}}"
      }
    },
    {
      "name": "create_report",
      "tool": "write_file",
      "args": {
        "path": "research_report.md",
        "content": "# Research: {{search_topic}}\n\n## Findings:\n{{summarize_goal}}\n\n## Device Info:\nAndroid Version: {{get_device_info_output}}"
      }
    }
  ]
}
```

### Pattern 5: Batch Operations
```json
{
  "name": "Multi-Screenshot Capture",
  "description": "Capture multiple screenshots with intervals",
  "variables": {
    "device": "",
    "interval_seconds": "5",
    "count": "3"
  },
  "steps": [
    {
      "name": "screenshot_1",
      "tool": "adb_screenshot",
      "args": {
        "filename": "batch_screen_1.png",
        "device": "{{device}}"
      }
    },
    {
      "name": "wait_1",
      "tool": "adb_shell",
      "args": {
        "command": "sleep {{interval_seconds}}",
        "device": "{{device}}"
      }
    },
    {
      "name": "screenshot_2",
      "tool": "adb_screenshot",
      "args": {
        "filename": "batch_screen_2.png",
        "device": "{{device}}"
      }
    },
    {
      "name": "wait_2",
      "tool": "adb_shell",
      "args": {
        "command": "sleep {{interval_seconds}}",
        "device": "{{device}}"
      }
    },
    {
      "name": "screenshot_3",
      "tool": "adb_screenshot",
      "args": {
        "filename": "batch_screen_3.png",
        "device": "{{device}}"
      }
    }
  ]
}
```

---

## Variable System

### Variable Types

1. **Default Variables** (defined in workflow)
   ```json
   "variables": {
     "device": "emulator-5554",
     "timeout": "30"
   }
   ```

2. **Runtime Overrides** (passed to workflow_execute)
   ```
   Execute workflow with variables: device="001a6de80412", timeout="60"
   ```

3. **Step Outputs** (automatic)
   ```
   After step "get_version" completes:
   Variable "get_version_output" contains the output
   ```

4. **Goal Results** (automatic)
   ```
   After goal step "analyze" completes:
   Variable "analyze_goal" contains the LLM response
   ```

### Interpolation Syntax

Use `{{variable_name}}` anywhere in:
- Tool arguments (strings)
- Goal text
- File paths
- Commands

**Examples:**
```json
"filename": "screen_{{device}}_{{timestamp}}.png"
"command": "am start -n {{package}}/{{activity}}"
"goal": "Analyze {{step_name_output}} and determine {{target_metric}}"
```

---

## Creating Workflows - Step by Step

### Step 1: Understand the Task

Ask yourself:
1. What is the end goal?
2. What tools are needed?
3. What data flows between steps?
4. Are there LLM decisions needed?

### Step 2: Design the Flow

```
1. Check device connection (adb_devices)
2. Get current state (adb_ui_dump)
3. Analyze state (LLM goal)
4. Take action based on analysis (adb_tap)
5. Verify result (adb_screenshot)
```

### Step 3: Define Variables

```json
"variables": {
  "device": "",           // Empty = use default device
  "app_name": "Settings", // Can be overridden
  "timeout": "30"         // Numeric as string
}
```

### Step 4: Write Steps

For each step:
1. Choose descriptive name (snake_case)
2. Select appropriate tool OR write goal
3. **Always include `args` field for tool steps**
4. Use `{{variables}}` where needed

### Step 5: Test & Iterate

```bash
# Save workflow
pepebot agent -m "save this workflow as 'my_workflow'"

# Execute with defaults
pepebot agent -m "execute my_workflow"

# Execute with overrides
pepebot agent -m "execute my_workflow with device='001a6de80412'"
```

---

## Available Tools for Workflows

### 📱 ADB Tools
- `adb_devices` - List connected devices
- `adb_shell` - Execute shell commands
- `adb_tap` - Tap coordinates
- `adb_input_text` - Type text
- `adb_screenshot` - Capture screen
- `adb_ui_dump` - Get UI hierarchy
- `adb_swipe` - Swipe gesture

### 📂 File Tools
- `read_file` - Read file contents
- `write_file` - Write/create files
- `list_dir` - List directory

### 🌐 Web Tools
- `web_search` - Search internet
- `web_fetch` - Fetch URL content

### 🔧 System Tools
- `exec` - Execute shell command

---

## Example Workflows by Use Case

### Use Case 1: Automated Testing
```json
{
  "name": "App Login Test",
  "description": "Automated login flow testing",
  "variables": {
    "device": "",
    "username": "testuser@example.com",
    "password": "testpass123"
  },
  "steps": [
    {
      "name": "launch_app",
      "tool": "adb_shell",
      "args": {
        "command": "am start -n com.example.app/.MainActivity",
        "device": "{{device}}"
      }
    },
    {
      "name": "wait",
      "tool": "adb_shell",
      "args": {
        "command": "sleep 3",
        "device": "{{device}}"
      }
    },
    {
      "name": "get_ui",
      "tool": "adb_ui_dump",
      "args": {
        "device": "{{device}}"
      }
    },
    {
      "name": "find_username_field",
      "goal": "Find username/email input field in UI dump, return coordinates as 'username_x' and 'username_y'"
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
      "name": "screenshot_result",
      "tool": "adb_screenshot",
      "args": {
        "filename": "login_test_result.png",
        "device": "{{device}}"
      }
    }
  ]
}
```

### Use Case 2: Device Monitoring
```json
{
  "name": "Hourly Health Check",
  "description": "Monitor device health and alert on issues",
  "variables": {
    "device": "",
    "battery_threshold": "20"
  },
  "steps": [
    {
      "name": "get_battery",
      "tool": "adb_shell",
      "args": {
        "command": "dumpsys battery | grep level",
        "device": "{{device}}"
      }
    },
    {
      "name": "check_threshold",
      "goal": "Extract battery percentage from 'get_battery_output'. If below {{battery_threshold}}%, set 'alert' to 'true', else 'false'."
    },
    {
      "name": "save_log",
      "tool": "write_file",
      "args": {
        "path": "health_log.txt",
        "content": "Timestamp: {{timestamp}}\nBattery: {{get_battery_output}}\nAlert: {{alert}}"
      }
    }
  ]
}
```

### Use Case 3: Data Collection
```json
{
  "name": "App Performance Profiler",
  "description": "Collect app performance metrics",
  "variables": {
    "device": "",
    "app_package": "com.example.app"
  },
  "steps": [
    {
      "name": "start_app",
      "tool": "adb_shell",
      "args": {
        "command": "am start -n {{app_package}}/.MainActivity",
        "device": "{{device}}"
      }
    },
    {
      "name": "get_cpu",
      "tool": "adb_shell",
      "args": {
        "command": "dumpsys cpuinfo | grep {{app_package}}",
        "device": "{{device}}"
      }
    },
    {
      "name": "get_memory",
      "tool": "adb_shell",
      "args": {
        "command": "dumpsys meminfo {{app_package}}",
        "device": "{{device}}"
      }
    },
    {
      "name": "analyze_performance",
      "goal": "Analyze CPU and memory data. Determine if app is performing normally or has issues. Provide performance rating (Good/Fair/Poor) and recommendations."
    },
    {
      "name": "save_profile",
      "tool": "write_file",
      "args": {
        "path": "performance_profile_{{timestamp}}.txt",
        "content": "App: {{app_package}}\n\nCPU:\n{{get_cpu_output}}\n\nMemory:\n{{get_memory_output}}\n\nAnalysis:\n{{analyze_performance_goal}}"
      }
    }
  ]
}
```

---

## Tips for LLM When Creating Workflows

### When User Asks to "Create a Workflow"

1. **Ask Clarifying Questions** (if needed):
   - What device/target?
   - What should happen in each step?
   - Any specific variables to parameterize?

2. **Structure the Workflow**:
   - Clear name and description
   - Define sensible default variables
   - Order steps logically
   - **Always include `args: {}`** even if empty

3. **Use `workflow_save` Tool**:
   ```json
   {
     "workflow_name": "my_workflow",
     "workflow_content": "{\"name\":\"...\", ...}"
   }
   ```

4. **Validate JSON**:
   - Ensure valid JSON syntax
   - All required fields present
   - No missing commas or braces

5. **Offer to Execute**:
   After saving, ask: "Would you like me to execute this workflow now?"

### Common User Requests

**"Create a workflow to..."**
→ Use `workflow_save` with complete JSON

**"Run the X workflow"**
→ Use `workflow_execute` with workflow name

**"What workflows are available?"**
→ Use `workflow_list`

**"Execute workflow with device ABC"**
→ Use `workflow_execute` with `variables: {"device": "ABC"}`

---

## Error Handling

### Common Errors

1. **"command is required"**
   - Cause: `adb_shell` step missing `command` in args
   - Fix: Add `"command": "your command here"`

2. **"workflow not found"**
   - Cause: Workflow file doesn't exist
   - Fix: Use `workflow_list` to see available workflows

3. **"device not found"**
   - Cause: Device ID doesn't match connected device
   - Fix: Override with correct device serial from `adb_devices`

4. **"step failed"**
   - Cause: Tool execution error
   - Fix: Check tool arguments, device connection, permissions

### Debugging Workflows

1. **Test Steps Individually**:
   Execute tools one by one before creating workflow

2. **Check Variable Interpolation**:
   Verify variables are defined and named correctly

3. **Validate JSON Syntax**:
   Use JSON validator or `jq` to check syntax

4. **Start Simple**:
   Create minimal workflow, then add steps incrementally

---

## Advanced Features

### Dynamic Timestamps

Use in goal steps:
```json
{
  "name": "add_timestamp",
  "goal": "Generate current timestamp in format YYYY-MM-DD_HH-MM-SS and store as 'timestamp' variable"
}
```

### Conditional Logic (via Goals)

```json
{
  "name": "check_condition",
  "goal": "If battery level from 'get_battery_output' is below 20%, set 'low_battery' to true, else false"
},
{
  "name": "conditional_action",
  "goal": "If 'low_battery' is true, send notification. Otherwise, do nothing."
}
```

### Data Transformation

```json
{
  "name": "parse_data",
  "goal": "Extract the numeric value from 'level: 85' string in 'battery_output' and store as 'battery_level'"
}
```

### Multi-Device Workflows

```json
{
  "name": "compare_devices",
  "goal": "Execute same command on device1 and device2, compare results, report which has better performance"
}
```

---

## Best Practices Summary

✅ **DO:**
- Include `args` field in all tool steps (even if empty)
- Use descriptive step names
- Define variables for flexibility
- Write clear goal instructions
- Test workflows before deploying
- Use variable interpolation for reusability
- Document workflow purpose in description

❌ **DON'T:**
- Omit `args` field from tool steps
- Use vague step names like "step1"
- Hardcode values that could be variables
- Write ambiguous goals
- Create overly complex workflows (split into smaller ones)
- Forget to escape special characters in commands

---

## Examples Library

Check these locations for more examples:
- `~/.pepebot/workspace/workflows/` - Your saved workflows
- `workspace/workflows/examples/` - Example workflows
  - `device_control.json` - Health monitoring
  - `ui_automation.json` - Login automation
  - `browser_automation.json` - Web + file operations

---

## Quick Reference

### Minimal Workflow
```json
{
  "name": "Simple Test",
  "description": "Minimal workflow example",
  "steps": [
    {
      "name": "list_devices",
      "tool": "adb_devices",
      "args": {}
    }
  ]
}
```

### Tool Step Template
```json
{
  "name": "step_name",
  "tool": "tool_name",
  "args": {
    "param1": "value",
    "param2": "{{variable}}"
  }
}
```

### Goal Step Template
```json
{
  "name": "goal_name",
  "goal": "Clear instruction for LLM. Specify what to analyze and what variables to create."
}
```

---

## 🎯 Summary

Workflows enable powerful automation by:
- **Combining tools** from different domains
- **Passing data** between steps
- **Leveraging LLM intelligence** for decisions
- **Parameterizing** with variables
- **Scaling** from simple to complex tasks

**Start simple, iterate, and build powerful automations!** 🚀

---

## 🔗 See Also

- `WORKFLOW_TEST_COMMANDS.md` - Testing commands
- `workspace/workflows/README.md` - Detailed workflow documentation
- `IMPLEMENTATION_SUMMARY.md` - Technical implementation details
