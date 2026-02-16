# Pepebot Workflows

Workflows are multi-step automation sequences that can execute any combination of tools available in pepebot. They support variable interpolation, sequential execution, and goal-based steps for LLM-driven automation.

## Features

- **Generic Tool Support**: Works with ANY registered tools (ADB, shell, browser, file operations, etc.)
- **Variable Interpolation**: Use `{{variable}}` syntax to pass data between steps
- **Step Output Tracking**: Each step's output becomes available as `{{step_name_output}}`
- **Goal-Based Steps**: Natural language goals for LLM to interpret and act upon
- **Override Variables**: Provide custom values at execution time

## Tools Available

### workflow_execute
Execute a workflow from a JSON file.

**Parameters:**
- `workflow_name` (required): Name of the workflow file (without .json extension)
- `variables` (optional): Object with variable overrides

**Example:**
```json
{
  "workflow_name": "device_control",
  "variables": {
    "device": "emulator-5554"
  }
}
```

### workflow_save
Create and save a new workflow definition.

**Parameters:**
- `workflow_name` (required): Name for the workflow (no .json extension)
- `workflow_content` (required): JSON workflow definition as string

### workflow_list
List all available workflows with their descriptions.

**Parameters:** None

## Workflow JSON Structure

```json
{
  "name": "Workflow Display Name",
  "description": "What this workflow does",
  "variables": {
    "var1": "default_value",
    "var2": "another_value"
  },
  "steps": [
    {
      "name": "step_name",
      "tool": "tool_name",
      "args": {
        "param": "{{variable}}"
      }
    },
    {
      "name": "analysis_step",
      "goal": "Natural language goal for LLM to interpret"
    }
  ]
}
```

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

#### Goal Steps
Provide natural language goals for the LLM to interpret:
```json
{
  "name": "analyze_ui",
  "goal": "Find the login button coordinates in the UI dump and store as 'button_x' and 'button_y'"
}
```

## Variable Interpolation

Variables are replaced throughout the workflow:

1. **Default Variables**: Defined in the `variables` section
2. **Override Variables**: Passed during execution via `workflow_execute`
3. **Step Outputs**: Automatically available as `{{step_name_output}}`
4. **Goal Results**: Available as `{{step_name_goal}}`

**Example:**
```json
{
  "variables": {
    "device": "emulator-5554",
    "username": "test@example.com"
  },
  "steps": [
    {
      "name": "get_ui",
      "tool": "adb_ui_dump",
      "args": {"device": "{{device}}"}
    },
    {
      "name": "tap_field",
      "tool": "adb_tap",
      "args": {
        "x": 100,
        "y": 200,
        "device": "{{device}}"
      }
    },
    {
      "name": "input_text",
      "tool": "adb_input_text",
      "args": {
        "text": "{{username}}",
        "device": "{{device}}"
      }
    }
  ]
}
```

## Example Workflows

Check the `examples/` directory for sample workflows:

### ui_automation.json
Demonstrates Android app login automation:
- UI hierarchy analysis
- Coordinate-based tapping
- Text input
- Screenshot verification

### device_control.json
Device health monitoring:
- Battery status
- Memory usage
- Storage capacity
- Network configuration
- Automated reporting

### browser_automation.json
Web research workflow (non-ADB example):
- Web search
- Result analysis
- File operations
- Cross-tool coordination

## Usage Examples

### Execute a Workflow
```
User: "Execute the device_control workflow"
Agent: Uses workflow_execute with workflow_name="device_control"
```

### Create a Custom Workflow
```
User: "Create a workflow that checks battery and takes a screenshot"
Agent: Uses workflow_save with appropriate JSON structure
```

### List Available Workflows
```
User: "What workflows are available?"
Agent: Uses workflow_list
```

### Execute with Variable Overrides
```
User: "Run device_control on device abc123"
Agent: Uses workflow_execute with:
  workflow_name: "device_control"
  variables: {"device": "abc123"}
```

## Best Practices

1. **Descriptive Step Names**: Use clear names for easy tracking
2. **Error Handling**: Consider multiple paths for goal-based steps
3. **Variable Naming**: Use consistent naming conventions
4. **Documentation**: Add clear descriptions to all workflows
5. **Modular Design**: Break complex automations into smaller workflows
6. **Testing**: Test workflows with different devices/scenarios

## ADB Tools Available

When ADB is installed, these tools are available for workflows:

- `adb_devices` - List connected devices
- `adb_shell` - Execute shell commands
- `adb_tap` - Tap screen coordinates
- `adb_input_text` - Input text
- `adb_screenshot` - Capture screenshots
- `adb_ui_dump` - Get UI hierarchy
- `adb_swipe` - Swipe gestures

## Common Patterns

### Device Automation Loop
1. Get UI hierarchy (`adb_ui_dump`)
2. Analyze UI (goal step)
3. Interact with UI (`adb_tap`, `adb_input_text`)
4. Verify result (`adb_screenshot`)

### Health Monitoring
1. Collect metrics (`adb_shell` commands)
2. Store results (step outputs)
3. Analyze data (goal step)
4. Generate report (goal step or `write_file`)

### Multi-Tool Automation
1. Web research (`web_search`)
2. Device interaction (`adb_*` tools)
3. File operations (`write_file`, `read_file`)
4. Reporting (goal steps + file tools)

## Troubleshooting

### Workflow Not Found
- Ensure the file exists in `workspace/workflows/`
- Check file extension is `.json`
- Verify workflow name doesn't include `.json` when executing

### Variable Not Interpolated
- Check variable name matches exactly (case-sensitive)
- Ensure variable is defined or output from previous step
- Use correct syntax: `{{variable_name}}`

### Tool Execution Failed
- Verify tool is registered and available
- Check tool arguments match expected schema
- Review error message for specific issues

### ADB Tools Not Available
- Install Android Platform Tools
- Set `ANDROID_HOME` environment variable
- Ensure `adb` is in system PATH
- Verify device is connected: `adb devices`

## Contributing Workflows

Share your workflows by:
1. Creating well-documented JSON files
2. Testing with multiple scenarios
3. Adding clear descriptions and variable documentation
4. Submitting examples to the `examples/` directory

## Future Enhancements

- Conditional step execution
- Parallel step execution
- Workflow templates library
- Recording mode (capture → generate workflow)
- Enhanced error handling and retry logic
- Workflow composition (nested workflows)
