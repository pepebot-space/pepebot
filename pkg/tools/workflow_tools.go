package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pepebot-space/pepebot/pkg/workflow"
)

// NewWorkflowExecuteTool creates the workflow_execute tool.
func NewWorkflowExecuteTool(helper *workflow.WorkflowHelper) *WorkflowExecuteTool {
	return &WorkflowExecuteTool{helper: helper}
}

// NewWorkflowSaveTool creates the workflow_save tool.
func NewWorkflowSaveTool(helper *workflow.WorkflowHelper) *WorkflowSaveTool {
	return &WorkflowSaveTool{helper: helper}
}

// NewWorkflowListTool creates the workflow_list tool.
func NewWorkflowListTool(helper *workflow.WorkflowHelper) *WorkflowListTool {
	return &WorkflowListTool{helper: helper}
}

// ==================== workflow_execute ====================

type WorkflowExecuteTool struct {
	helper *workflow.WorkflowHelper
}

func (t *WorkflowExecuteTool) Name() string { return "workflow_execute" }

func (t *WorkflowExecuteTool) Description() string {
	return "Execute a workflow from a JSON file. Workflows are multi-step automations that can call any registered tools (ADB, shell, browser, etc.) with variable interpolation and goal-based steps."
}

func (t *WorkflowExecuteTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"workflow_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the workflow file (without .json extension)",
			},
			"variables": map[string]interface{}{
				"type":        "object",
				"description": "Override workflow variables (optional)",
			},
		},
		"required": []string{"workflow_name"},
	}
}

func (t *WorkflowExecuteTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	workflowName, ok := args["workflow_name"].(string)
	if !ok {
		return "", fmt.Errorf("workflow_name is required")
	}

	wf, err := t.helper.LoadWorkflow(workflowName)
	if err != nil {
		available := t.helper.ListWorkflows()
		if len(available) > 0 {
			return "", fmt.Errorf("%w. Available workflows: %s", err, strings.Join(available, ", "))
		}
		return "", err
	}

	overrideVars := make(map[string]string)
	if varsRaw, ok := args["variables"].(map[string]interface{}); ok {
		for k, v := range varsRaw {
			if strVal, ok := v.(string); ok {
				overrideVars[k] = strVal
			} else {
				overrideVars[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	return t.helper.ExecuteWorkflow(ctx, wf, overrideVars)
}

// ==================== workflow_save ====================

type WorkflowSaveTool struct {
	helper *workflow.WorkflowHelper
}

func (t *WorkflowSaveTool) Name() string { return "workflow_save" }

func (t *WorkflowSaveTool) Description() string {
	return `Save a workflow JSON file. IMPORTANT: Only use this tool when the user EXPLICITLY asks to create or save a workflow. Do NOT proactively create workflows.

4 STEP TYPES:
- Tool step: {"name":"id", "tool":"tool_name", "args":{"param":"value"}} — Execute a registered tool. MUST have "args" even if empty {}.
- Goal step: {"name":"id", "goal":"instruction"} — Natural language for LLM to interpret.
- Skill step: {"name":"id", "skill":"skill_name", "goal":"instruction"} — Load a skill's content + combine with goal. IMPORTANT: When the user says "use skill X" or "with skill X", ALWAYS use this step type. Do NOT manually replicate the skill's commands via tool steps.
- Agent step: {"name":"id", "agent":"agent_name", "goal":"instruction"} — Delegate goal to another agent. The agent processes independently and returns a response.

RULES: (1) "tool" cannot combine with "skill"/"agent". (2) "skill" and "agent" are mutually exclusive. (3) "skill" and "agent" REQUIRE "goal". (4) Use {{variable}} for interpolation. (5) Step outputs auto-stored as {{step_name_output}}.

NOTE: If the user wants to record/capture actions from their Android device to create a workflow, use adb_record_workflow instead.`
}

func (t *WorkflowSaveTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"workflow_name": map[string]interface{}{
				"type":        "string",
				"description": "Name for the workflow (no .json extension needed)",
			},
			"workflow_content": map[string]interface{}{
				"type":        "string",
				"description": "JSON workflow definition as a string",
			},
		},
		"required": []string{"workflow_name", "workflow_content"},
	}
}

func (t *WorkflowSaveTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	workflowName, ok := args["workflow_name"].(string)
	if !ok {
		return "", fmt.Errorf("workflow_name is required")
	}
	workflowContent, ok := args["workflow_content"].(string)
	if !ok {
		return "", fmt.Errorf("workflow_content is required")
	}

	if strings.ContainsAny(workflowName, "/\\:*?\"<>|") {
		return "", fmt.Errorf("invalid workflow name: contains special characters")
	}

	var wf workflow.WorkflowDefinition
	if err := json.Unmarshal([]byte(workflowContent), &wf); err != nil {
		return "", fmt.Errorf("invalid workflow JSON: %w. Check for missing commas, brackets, or quotes", err)
	}

	if err := t.helper.Validate(&wf); err != nil {
		return "", fmt.Errorf("validation error: %w", err)
	}

	if err := t.helper.SaveWorkflow(workflowName, &wf); err != nil {
		return "", err
	}

	path := filepath.Join(t.helper.WorkflowsDir(), workflowName)
	if !strings.HasSuffix(path, ".json") {
		path += ".json"
	}
	return fmt.Sprintf("Workflow saved successfully to: %s\nName: %s\nDescription: %s\nSteps: %d", path, wf.Name, wf.Description, len(wf.Steps)), nil
}

// ==================== workflow_list ====================

type WorkflowListTool struct {
	helper *workflow.WorkflowHelper
}

func (t *WorkflowListTool) Name() string { return "workflow_list" }

func (t *WorkflowListTool) Description() string {
	return "List all available workflows in the workspace. Shows workflow names, descriptions, and step counts."
}

func (t *WorkflowListTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *WorkflowListTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	names := t.helper.ListWorkflows()

	workflows := []map[string]interface{}{}
	for _, name := range names {
		wf, err := t.helper.LoadWorkflow(name)
		if err != nil {
			continue
		}
		workflows = append(workflows, map[string]interface{}{
			"name":        name,
			"description": wf.Description,
			"steps":       len(wf.Steps),
		})
	}

	if len(workflows) == 0 {
		return "No workflows found in workspace/workflows/", nil
	}

	result, _ := json.MarshalIndent(workflows, "", "  ")
	return string(result), nil
}
