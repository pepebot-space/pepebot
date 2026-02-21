package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// WorkflowAgentProcessor allows workflows to delegate steps to other agents.
// Implemented by AgentManager to avoid circular imports.
type WorkflowAgentProcessor interface {
	ProcessDirect(ctx context.Context, content string, media []string, sessionKey string, agentName string) (string, error)
}

// WorkflowSkillProvider allows workflows to load skill content.
// Implemented by skills.SkillsLoader.
type WorkflowSkillProvider interface {
	LoadSkill(name string) (string, bool)
}

// WorkflowDefinition represents a workflow JSON structure
type WorkflowDefinition struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Variables   map[string]string `json:"variables,omitempty"`
	Steps       []WorkflowStep    `json:"steps"`
}

// WorkflowStep represents a single step in a workflow
type WorkflowStep struct {
	Name  string                 `json:"name"`
	Tool  string                 `json:"tool,omitempty"`  // Tool name to execute
	Args  map[string]interface{} `json:"args,omitempty"`  // Tool arguments
	Goal  string                 `json:"goal,omitempty"`  // Natural language goal for LLM
	Skill string                 `json:"skill,omitempty"` // Skill name to load and combine with goal
	Agent string                 `json:"agent,omitempty"` // Agent name to delegate goal to
}

// WorkflowHelper manages workflow execution and storage
type WorkflowHelper struct {
	workspace      string
	toolRegistry   *ToolRegistry
	skillProvider  WorkflowSkillProvider
	agentProcessor WorkflowAgentProcessor
}

// SetSkillProvider sets the skill provider for skill steps
func (h *WorkflowHelper) SetSkillProvider(provider WorkflowSkillProvider) {
	h.skillProvider = provider
}

// SetAgentProcessor sets the agent processor for agent steps
func (h *WorkflowHelper) SetAgentProcessor(processor WorkflowAgentProcessor) {
	h.agentProcessor = processor
}

// NewWorkflowHelper creates a new workflow helper
func NewWorkflowHelper(workspace string, registry *ToolRegistry) *WorkflowHelper {
	workflowsDir := filepath.Join(workspace, "workflows")
	os.MkdirAll(workflowsDir, 0755)
	return &WorkflowHelper{
		workspace:    workspace,
		toolRegistry: registry,
	}
}

// workflowsDir returns the workflows directory path
func (h *WorkflowHelper) workflowsDir() string {
	return filepath.Join(h.workspace, "workflows")
}

// listWorkflowNames returns names of all available workflow files
func (h *WorkflowHelper) listWorkflowNames() []string {
	entries, err := os.ReadDir(h.workflowsDir())
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return names
}

// loadWorkflow loads a workflow definition from file
func (h *WorkflowHelper) loadWorkflow(name string) (*WorkflowDefinition, error) {
	// Add .json extension if not present
	if !strings.HasSuffix(name, ".json") {
		name = name + ".json"
	}

	path := filepath.Join(h.workflowsDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file: %w", err)
	}

	var workflow WorkflowDefinition
	if err := json.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to parse workflow JSON: %w", err)
	}

	return &workflow, nil
}

// saveWorkflow saves a workflow definition to file
func (h *WorkflowHelper) saveWorkflow(name string, workflow *WorkflowDefinition) error {
	// Add .json extension if not present
	if !strings.HasSuffix(name, ".json") {
		name = name + ".json"
	}

	path := filepath.Join(h.workflowsDir(), name)
	data, err := json.MarshalIndent(workflow, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal workflow: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write workflow file: %w", err)
	}

	return nil
}

// interpolateVariables replaces {{variable}} placeholders with actual values
func interpolateVariables(input string, variables map[string]string) string {
	result := input
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// interpolateArgs applies variable interpolation to tool arguments
func interpolateArgs(args map[string]interface{}, variables map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range args {
		if strVal, ok := value.(string); ok {
			result[key] = interpolateVariables(strVal, variables)
		} else {
			result[key] = value
		}
	}
	return result
}

// coerceArgsForTool converts string values to the types expected by the tool's parameter schema.
// This fixes the issue where variable interpolation produces strings like "540"
// but tools like adb_tap expect float64.
func coerceArgsForTool(tool Tool, args map[string]interface{}) map[string]interface{} {
	params := tool.Parameters()
	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		return args
	}

	result := make(map[string]interface{})
	for k, v := range args {
		result[k] = v
	}

	for key, schema := range properties {
		val, exists := result[key]
		if !exists {
			continue
		}

		propSchema, ok := schema.(map[string]interface{})
		if !ok {
			continue
		}

		expectedType, _ := propSchema["type"].(string)
		strVal, isString := val.(string)

		// Convert string to number if tool expects number
		if isString && expectedType == "number" {
			if f, err := strconv.ParseFloat(strVal, 64); err == nil {
				result[key] = f
			}
		}

		// Convert string "true"/"false" to bool if tool expects boolean
		if isString && expectedType == "boolean" {
			if b, err := strconv.ParseBool(strVal); err == nil {
				result[key] = b
			}
		}

		// Convert string to integer if tool expects integer
		if isString && expectedType == "integer" {
			if i, err := strconv.ParseInt(strVal, 10, 64); err == nil {
				result[key] = i
			}
		}
	}

	return result
}

// executeWorkflow executes a workflow with given variables
func (h *WorkflowHelper) executeWorkflow(ctx context.Context, workflow *WorkflowDefinition, overrideVars map[string]string) (string, error) {
	// Merge variables: workflow defaults + overrides
	variables := make(map[string]string)
	for k, v := range workflow.Variables {
		variables[k] = v
	}
	for k, v := range overrideVars {
		variables[k] = v
	}

	results := []string{}
	results = append(results, fmt.Sprintf("Executing workflow: %s", workflow.Name))
	results = append(results, fmt.Sprintf("Description: %s", workflow.Description))
	results = append(results, "")

	// Execute steps sequentially
	for i, step := range workflow.Steps {
		results = append(results, fmt.Sprintf("Step %d/%d: %s", i+1, len(workflow.Steps), step.Name))

		// Tool step: Execute the tool
		if step.Tool != "" {
			// Interpolate variables in args
			interpolatedArgs := interpolateArgs(step.Args, variables)

			// Auto-coerce types (string "540" → float64 540.0) based on tool schema
			if tool, ok := h.toolRegistry.Get(step.Tool); ok {
				interpolatedArgs = coerceArgsForTool(tool, interpolatedArgs)
			}

			// Execute tool
			output, err := h.toolRegistry.Execute(ctx, step.Tool, interpolatedArgs)
			if err != nil {
				errMsg := fmt.Sprintf("  ERROR: %v", err)
				results = append(results, errMsg)
				return strings.Join(results, "\n"), fmt.Errorf("step %d (%s) failed: %w", i+1, step.Name, err)
			}

			// Store step output as variable for future steps
			variables[step.Name+"_output"] = output

			// Truncate output for display
			displayOutput := output
			maxLen := 500
			if len(displayOutput) > maxLen {
				displayOutput = displayOutput[:maxLen] + "... (truncated)"
			}
			results = append(results, fmt.Sprintf("  Tool: %s", step.Tool))
			results = append(results, fmt.Sprintf("  Output: %s", displayOutput))
		}

		// Skill step: Load skill content and combine with goal
		if step.Skill != "" {
			if h.skillProvider == nil {
				errMsg := fmt.Sprintf("  ERROR: skill provider not available")
				results = append(results, errMsg)
				return strings.Join(results, "\n"), fmt.Errorf("step %d (%s) failed: skill provider not available", i+1, step.Name)
			}
			skillContent, ok := h.skillProvider.LoadSkill(step.Skill)
			if !ok {
				errMsg := fmt.Sprintf("  ERROR: skill '%s' not found", step.Skill)
				results = append(results, errMsg)
				return strings.Join(results, "\n"), fmt.Errorf("step %d (%s) failed: skill '%s' not found", i+1, step.Name, step.Skill)
			}
			interpolatedGoal := interpolateVariables(step.Goal, variables)
			combined := fmt.Sprintf("Using skill '%s':\n\n%s\n\nGoal: %s", step.Skill, skillContent, interpolatedGoal)
			variables[step.Name+"_output"] = combined
			results = append(results, fmt.Sprintf("  Skill: %s", step.Skill))
			results = append(results, fmt.Sprintf("  Goal: %s", interpolatedGoal))
		}

		// Agent step: Delegate goal to another agent
		if step.Agent != "" {
			if h.agentProcessor == nil {
				errMsg := fmt.Sprintf("  ERROR: agent processor not available (standalone mode)")
				results = append(results, errMsg)
				return strings.Join(results, "\n"), fmt.Errorf("step %d (%s) failed: agent processor not available (standalone mode does not support agent steps)", i+1, step.Name)
			}
			interpolatedGoal := interpolateVariables(step.Goal, variables)
			sessionKey := fmt.Sprintf("workflow:%s:%s", workflow.Name, step.Name)
			agentResponse, err := h.agentProcessor.ProcessDirect(ctx, interpolatedGoal, nil, sessionKey, step.Agent)
			if err != nil {
				errMsg := fmt.Sprintf("  ERROR: agent '%s' failed: %v", step.Agent, err)
				results = append(results, errMsg)
				return strings.Join(results, "\n"), fmt.Errorf("step %d (%s) failed: %w", i+1, step.Name, err)
			}
			variables[step.Name+"_output"] = agentResponse
			displayOutput := agentResponse
			maxLen := 500
			if len(displayOutput) > maxLen {
				displayOutput = displayOutput[:maxLen] + "... (truncated)"
			}
			results = append(results, fmt.Sprintf("  Agent: %s", step.Agent))
			results = append(results, fmt.Sprintf("  Goal: %s", interpolatedGoal))
			results = append(results, fmt.Sprintf("  Response: %s", displayOutput))
		}

		// Goal step: Return goal for LLM to interpret
		if step.Goal != "" && step.Skill == "" && step.Agent == "" {
			interpolatedGoal := interpolateVariables(step.Goal, variables)
			results = append(results, fmt.Sprintf("  Goal: %s", interpolatedGoal))
			results = append(results, "  Note: This is a goal-based step. The LLM should interpret and act on this goal in the next iteration.")

			// Store goal as variable
			variables[step.Name+"_goal"] = interpolatedGoal
		}

		results = append(results, "")
	}

	results = append(results, "Workflow execution completed successfully!")
	return strings.Join(results, "\n"), nil
}

// validateWorkflow validates workflow structure and checks for common errors.
// If registry is provided, also validates tool existence and required parameters.
func validateWorkflow(workflow *WorkflowDefinition, registry ...*ToolRegistry) error {
	if workflow.Name == "" {
		return fmt.Errorf("workflow must have a 'name' field")
	}

	if len(workflow.Steps) == 0 {
		return fmt.Errorf("workflow must have at least one step")
	}

	var reg *ToolRegistry
	if len(registry) > 0 {
		reg = registry[0]
	}

	// Collect variable names for interpolation checking
	definedVars := make(map[string]bool)
	for k := range workflow.Variables {
		definedVars[k] = true
	}

	for i, step := range workflow.Steps {
		if step.Name == "" {
			return fmt.Errorf("step %d: missing 'name' field", i+1)
		}

		// Step must have at least one type indicator
		if step.Tool == "" && step.Goal == "" && step.Skill == "" && step.Agent == "" {
			return fmt.Errorf("step %d (%s): must have at least one of 'tool', 'goal', 'skill', or 'agent' field", i+1, step.Name)
		}

		// Tool cannot be combined with skill or agent
		if step.Tool != "" && (step.Skill != "" || step.Agent != "") {
			return fmt.Errorf("step %d (%s): 'tool' cannot be combined with 'skill' or 'agent'", i+1, step.Name)
		}

		// Tool and goal are mutually exclusive (existing rule)
		if step.Tool != "" && step.Goal != "" {
			return fmt.Errorf("step %d (%s): has both 'tool' and 'goal'. Use only one per step", i+1, step.Name)
		}

		// Skill and agent are mutually exclusive
		if step.Skill != "" && step.Agent != "" {
			return fmt.Errorf("step %d (%s): 'skill' and 'agent' are mutually exclusive", i+1, step.Name)
		}

		// Skill and agent require goal
		if step.Skill != "" && step.Goal == "" {
			return fmt.Errorf("step %d (%s): 'skill' step requires a 'goal' field", i+1, step.Name)
		}
		if step.Agent != "" && step.Goal == "" {
			return fmt.Errorf("step %d (%s): 'agent' step requires a 'goal' field", i+1, step.Name)
		}

		// Tool step validation
		if step.Tool != "" {
			// CRITICAL: Tool steps MUST have args field
			if step.Args == nil {
				return fmt.Errorf("step %d (%s): missing 'args' field. Every tool step MUST include \"args\": {} (even if empty)", i+1, step.Name)
			}

			// Validate tool exists in registry
			if reg != nil {
				tool, exists := reg.Get(step.Tool)
				if !exists {
					return fmt.Errorf("step %d (%s): tool '%s' not found. Check tool name spelling", i+1, step.Name, step.Tool)
				}

				// Validate required parameters
				params := tool.Parameters()
				if required, ok := params["required"].([]string); ok {
					for _, reqParam := range required {
						if _, hasArg := step.Args[reqParam]; !hasArg {
							// Check if arg uses variable interpolation (allow {{var}})
							found := false
							for _, v := range step.Args {
								if strV, ok := v.(string); ok && strings.Contains(strV, "{{") {
									found = true
									break
								}
							}
							if !found {
								return fmt.Errorf("step %d (%s): tool '%s' requires parameter '%s' in args", i+1, step.Name, step.Tool, reqParam)
							}
						}
					}
				}

				// Also handle required as []interface{} (JSON unmarshal default)
				if required, ok := params["required"].([]interface{}); ok {
					for _, r := range required {
						reqParam, _ := r.(string)
						if reqParam == "" {
							continue
						}
						if _, hasArg := step.Args[reqParam]; !hasArg {
							return fmt.Errorf("step %d (%s): tool '%s' requires parameter '%s' in args", i+1, step.Name, step.Tool, reqParam)
						}
					}
				}
			}
		}

		// Track step output as available variable for subsequent steps
		definedVars[step.Name+"_output"] = true
		definedVars[step.Name+"_goal"] = true
	}

	return nil
}

// ==================== Workflow Execute Tool ====================

type WorkflowExecuteTool struct {
	helper *WorkflowHelper
}

func NewWorkflowExecuteTool(helper *WorkflowHelper) *WorkflowExecuteTool {
	return &WorkflowExecuteTool{helper: helper}
}

func (t *WorkflowExecuteTool) Name() string {
	return "workflow_execute"
}

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

	// Load workflow
	workflow, err := t.helper.loadWorkflow(workflowName)
	if err != nil {
		// List available workflows to help the user
		available := t.helper.listWorkflowNames()
		if len(available) > 0 {
			return "", fmt.Errorf("%w. Available workflows: %s", err, strings.Join(available, ", "))
		}
		return "", err
	}

	// Extract variable overrides
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

	// Execute workflow
	return t.helper.executeWorkflow(ctx, workflow, overrideVars)
}

// ==================== Workflow Save Tool ====================

type WorkflowSaveTool struct {
	helper *WorkflowHelper
}

func NewWorkflowSaveTool(helper *WorkflowHelper) *WorkflowSaveTool {
	return &WorkflowSaveTool{helper: helper}
}

func (t *WorkflowSaveTool) Name() string {
	return "workflow_save"
}

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

	// Validate workflow name (no special characters)
	if strings.ContainsAny(workflowName, "/\\:*?\"<>|") {
		return "", fmt.Errorf("invalid workflow name: contains special characters")
	}

	// Parse workflow JSON to validate structure
	var workflow WorkflowDefinition
	if err := json.Unmarshal([]byte(workflowContent), &workflow); err != nil {
		return "", fmt.Errorf("invalid workflow JSON: %w. Check for missing commas, brackets, or quotes", err)
	}

	// Validate workflow structure (with registry for tool/param checking)
	if err := validateWorkflow(&workflow, t.helper.toolRegistry); err != nil {
		return "", fmt.Errorf("validation error: %w", err)
	}

	// Save workflow
	if err := t.helper.saveWorkflow(workflowName, &workflow); err != nil {
		return "", err
	}

	path := filepath.Join(t.helper.workflowsDir(), workflowName)
	if !strings.HasSuffix(path, ".json") {
		path += ".json"
	}

	return fmt.Sprintf("Workflow saved successfully to: %s\nName: %s\nDescription: %s\nSteps: %d", path, workflow.Name, workflow.Description, len(workflow.Steps)), nil
}

// ==================== Workflow List Tool ====================

type WorkflowListTool struct {
	helper *WorkflowHelper
}

func NewWorkflowListTool(helper *WorkflowHelper) *WorkflowListTool {
	return &WorkflowListTool{helper: helper}
}

func (t *WorkflowListTool) Name() string {
	return "workflow_list"
}

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
	// Read workflows directory
	entries, err := os.ReadDir(t.helper.workflowsDir())
	if err != nil {
		return "", fmt.Errorf("failed to read workflows directory: %w", err)
	}

	workflows := []map[string]interface{}{}

	// Parse each workflow file
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		workflowName := strings.TrimSuffix(entry.Name(), ".json")
		workflow, err := t.helper.loadWorkflow(workflowName)
		if err != nil {
			continue // Skip invalid workflows
		}

		workflows = append(workflows, map[string]interface{}{
			"name":        workflowName,
			"description": workflow.Description,
			"steps":       len(workflow.Steps),
		})
	}

	if len(workflows) == 0 {
		return "No workflows found in workspace/workflows/", nil
	}

	result, _ := json.MarshalIndent(workflows, "", "  ")
	return string(result), nil
}
