package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ToolExecutor abstracts the tool registry for workflow step execution.
// Implemented by *tools.ToolRegistry to avoid a circular dependency.
type ToolExecutor interface {
	Execute(ctx context.Context, name string, args map[string]interface{}) (string, error)
	// GetToolSchema returns the parameters schema for a named tool.
	// Returns nil, false if the tool does not exist.
	GetToolSchema(name string) (map[string]interface{}, bool)
}

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

// WorkflowDefinition represents a workflow JSON structure.
type WorkflowDefinition struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Variables   map[string]string `json:"variables,omitempty"`
	Steps       []WorkflowStep    `json:"steps"`
}

// WorkflowStep represents a single step in a workflow.
type WorkflowStep struct {
	Name  string                 `json:"name"`
	Tool  string                 `json:"tool,omitempty"`  // Tool name to execute
	Args  map[string]interface{} `json:"args,omitempty"`  // Tool arguments
	Goal  string                 `json:"goal,omitempty"`  // Natural language goal for LLM
	Skill string                 `json:"skill,omitempty"` // Skill name to load and combine with goal
	Agent string                 `json:"agent,omitempty"` // Agent name to delegate goal to
}

// WorkflowHelper manages workflow execution and storage.
type WorkflowHelper struct {
	workspace      string
	executor       ToolExecutor
	skillProvider  WorkflowSkillProvider
	agentProcessor WorkflowAgentProcessor
}

// NewWorkflowHelper creates a new WorkflowHelper.
// executor is typically a *tools.ToolRegistry.
func NewWorkflowHelper(workspace string, executor ToolExecutor) *WorkflowHelper {
	workflowsDir := filepath.Join(workspace, "workflows")
	os.MkdirAll(workflowsDir, 0755)
	return &WorkflowHelper{
		workspace: workspace,
		executor:  executor,
	}
}

// SetSkillProvider sets the skill provider for skill steps.
func (h *WorkflowHelper) SetSkillProvider(provider WorkflowSkillProvider) {
	h.skillProvider = provider
}

// SetAgentProcessor sets the agent processor for agent steps.
func (h *WorkflowHelper) SetAgentProcessor(processor WorkflowAgentProcessor) {
	h.agentProcessor = processor
}

// WorkflowsDir returns the path to the workflows directory.
func (h *WorkflowHelper) WorkflowsDir() string {
	return filepath.Join(h.workspace, "workflows")
}

// ListWorkflows returns names of all available workflows in the workspace.
func (h *WorkflowHelper) ListWorkflows() []string {
	entries, err := os.ReadDir(h.WorkflowsDir())
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

// LoadWorkflow loads a workflow definition by name from the workspace.
func (h *WorkflowHelper) LoadWorkflow(name string) (*WorkflowDefinition, error) {
	if !strings.HasSuffix(name, ".json") {
		name = name + ".json"
	}
	path := filepath.Join(h.WorkflowsDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file: %w", err)
	}
	var wf WorkflowDefinition
	if err := json.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("failed to parse workflow JSON: %w", err)
	}
	return &wf, nil
}

// LoadWorkflowFile loads a workflow definition from an arbitrary file path.
func (h *WorkflowHelper) LoadWorkflowFile(filePath string) (*WorkflowDefinition, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file: %w", err)
	}
	var wf WorkflowDefinition
	if err := json.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("failed to parse workflow JSON: %w", err)
	}
	return &wf, nil
}

// SaveWorkflow saves a workflow definition to the workspace.
func (h *WorkflowHelper) SaveWorkflow(name string, wf *WorkflowDefinition) error {
	if !strings.HasSuffix(name, ".json") {
		name = name + ".json"
	}
	path := filepath.Join(h.WorkflowsDir(), name)
	data, err := json.MarshalIndent(wf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal workflow: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write workflow file: %w", err)
	}
	return nil
}

// RunWorkflow loads a named workflow from the workspace and executes it.
func (h *WorkflowHelper) RunWorkflow(ctx context.Context, name string, vars map[string]string) (string, error) {
	wf, err := h.LoadWorkflow(name)
	if err != nil {
		available := h.ListWorkflows()
		if len(available) > 0 {
			return "", fmt.Errorf("%w. Available workflows: %s", err, strings.Join(available, ", "))
		}
		return "", err
	}
	return h.ExecuteWorkflow(ctx, wf, vars)
}

// RunWorkflowFile loads a workflow from a file path and executes it.
func (h *WorkflowHelper) RunWorkflowFile(ctx context.Context, filePath string, vars map[string]string) (string, error) {
	wf, err := h.LoadWorkflowFile(filePath)
	if err != nil {
		return "", err
	}
	return h.ExecuteWorkflow(ctx, wf, vars)
}

// ExecuteWorkflow executes an already-loaded workflow definition.
func (h *WorkflowHelper) ExecuteWorkflow(ctx context.Context, wf *WorkflowDefinition, overrideVars map[string]string) (string, error) {
	// Merge variables: workflow defaults + overrides
	variables := make(map[string]string)
	for k, v := range wf.Variables {
		variables[k] = v
	}
	for k, v := range overrideVars {
		variables[k] = v
	}

	results := []string{}
	results = append(results, fmt.Sprintf("Executing workflow: %s", wf.Name))
	results = append(results, fmt.Sprintf("Description: %s", wf.Description))
	results = append(results, "")

	for i, step := range wf.Steps {
		results = append(results, fmt.Sprintf("Step %d/%d: %s", i+1, len(wf.Steps), step.Name))

		// Tool step
		if step.Tool != "" {
			interpolatedArgs := interpolateArgs(step.Args, variables)

			if schema, ok := h.executor.GetToolSchema(step.Tool); ok {
				interpolatedArgs = coerceArgs(schema, interpolatedArgs)
			}

			output, err := h.executor.Execute(ctx, step.Tool, interpolatedArgs)
			if err != nil {
				results = append(results, fmt.Sprintf("  ERROR: %v", err))
				return strings.Join(results, "\n"), fmt.Errorf("step %d (%s) failed: %w", i+1, step.Name, err)
			}

			variables[step.Name+"_output"] = output

			displayOutput := output
			if len(displayOutput) > 500 {
				displayOutput = displayOutput[:500] + "... (truncated)"
			}
			results = append(results, fmt.Sprintf("  Tool: %s", step.Tool))
			results = append(results, fmt.Sprintf("  Output: %s", displayOutput))
		}

		// Skill step
		if step.Skill != "" {
			if h.skillProvider == nil {
				results = append(results, "  ERROR: skill provider not available")
				return strings.Join(results, "\n"), fmt.Errorf("step %d (%s) failed: skill provider not available", i+1, step.Name)
			}
			skillContent, ok := h.skillProvider.LoadSkill(step.Skill)
			if !ok {
				results = append(results, fmt.Sprintf("  ERROR: skill '%s' not found", step.Skill))
				return strings.Join(results, "\n"), fmt.Errorf("step %d (%s) failed: skill '%s' not found", i+1, step.Name, step.Skill)
			}
			interpolatedGoal := interpolateVariables(step.Goal, variables)
			combined := fmt.Sprintf("Using skill '%s':\n\n%s\n\nGoal: %s", step.Skill, skillContent, interpolatedGoal)
			variables[step.Name+"_output"] = combined
			results = append(results, fmt.Sprintf("  Skill: %s", step.Skill))
			results = append(results, fmt.Sprintf("  Goal: %s", interpolatedGoal))
		}

		// Agent step
		if step.Agent != "" {
			if h.agentProcessor == nil {
				results = append(results, "  ERROR: agent processor not available (standalone mode)")
				return strings.Join(results, "\n"), fmt.Errorf("step %d (%s) failed: agent processor not available (standalone mode does not support agent steps)", i+1, step.Name)
			}
			interpolatedGoal := interpolateVariables(step.Goal, variables)
			sessionKey := fmt.Sprintf("workflow:%s:%s", wf.Name, step.Name)
			agentResponse, err := h.agentProcessor.ProcessDirect(ctx, interpolatedGoal, nil, sessionKey, step.Agent)
			if err != nil {
				results = append(results, fmt.Sprintf("  ERROR: agent '%s' failed: %v", step.Agent, err))
				return strings.Join(results, "\n"), fmt.Errorf("step %d (%s) failed: %w", i+1, step.Name, err)
			}
			variables[step.Name+"_output"] = agentResponse
			displayOutput := agentResponse
			if len(displayOutput) > 500 {
				displayOutput = displayOutput[:500] + "... (truncated)"
			}
			results = append(results, fmt.Sprintf("  Agent: %s", step.Agent))
			results = append(results, fmt.Sprintf("  Goal: %s", interpolatedGoal))
			results = append(results, fmt.Sprintf("  Response: %s", displayOutput))
		}

		// Goal step (pure LLM, no skill/agent)
		if step.Goal != "" && step.Skill == "" && step.Agent == "" {
			interpolatedGoal := interpolateVariables(step.Goal, variables)
			results = append(results, fmt.Sprintf("  Goal: %s", interpolatedGoal))
			results = append(results, "  Note: This is a goal-based step. The LLM should interpret and act on this goal in the next iteration.")
			variables[step.Name+"_goal"] = interpolatedGoal
		}

		results = append(results, "")
	}

	results = append(results, "Workflow execution completed successfully!")
	return strings.Join(results, "\n"), nil
}

// Validate validates a workflow's structure using the executor for tool/param checking.
func (h *WorkflowHelper) Validate(wf *WorkflowDefinition) error {
	return validateWorkflow(wf, h.executor)
}

// ValidateDefinition validates a workflow definition without a tool executor (structure only).
func ValidateDefinition(wf *WorkflowDefinition) error {
	return validateWorkflow(wf, nil)
}

// ==================== Internal helpers ====================

func interpolateVariables(input string, variables map[string]string) string {
	result := input
	for key, value := range variables {
		result = strings.ReplaceAll(result, fmt.Sprintf("{{%s}}", key), value)
	}
	return result
}

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

// coerceArgs converts string values to the types expected by the tool's parameter schema.
// This fixes the issue where variable interpolation produces strings like "540"
// but tools like adb_tap expect float64.
func coerceArgs(schema map[string]interface{}, args map[string]interface{}) map[string]interface{} {
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return args
	}

	result := make(map[string]interface{})
	for k, v := range args {
		result[k] = v
	}

	for key, propRaw := range properties {
		val, exists := result[key]
		if !exists {
			continue
		}
		propSchema, ok := propRaw.(map[string]interface{})
		if !ok {
			continue
		}
		expectedType, _ := propSchema["type"].(string)
		strVal, isString := val.(string)

		if isString && expectedType == "number" {
			if f, err := strconv.ParseFloat(strVal, 64); err == nil {
				result[key] = f
			}
		}
		if isString && expectedType == "boolean" {
			if b, err := strconv.ParseBool(strVal); err == nil {
				result[key] = b
			}
		}
		if isString && expectedType == "integer" {
			if i, err := strconv.ParseInt(strVal, 10, 64); err == nil {
				result[key] = i
			}
		}
	}
	return result
}

func validateWorkflow(wf *WorkflowDefinition, executor ToolExecutor) error {
	if wf.Name == "" {
		return fmt.Errorf("workflow must have a 'name' field")
	}
	if len(wf.Steps) == 0 {
		return fmt.Errorf("workflow must have at least one step")
	}

	definedVars := make(map[string]bool)
	for k := range wf.Variables {
		definedVars[k] = true
	}

	for i, step := range wf.Steps {
		if step.Name == "" {
			return fmt.Errorf("step %d: missing 'name' field", i+1)
		}
		if step.Tool == "" && step.Goal == "" && step.Skill == "" && step.Agent == "" {
			return fmt.Errorf("step %d (%s): must have at least one of 'tool', 'goal', 'skill', or 'agent' field", i+1, step.Name)
		}
		if step.Tool != "" && (step.Skill != "" || step.Agent != "") {
			return fmt.Errorf("step %d (%s): 'tool' cannot be combined with 'skill' or 'agent'", i+1, step.Name)
		}
		if step.Tool != "" && step.Goal != "" {
			return fmt.Errorf("step %d (%s): has both 'tool' and 'goal'. Use only one per step", i+1, step.Name)
		}
		if step.Skill != "" && step.Agent != "" {
			return fmt.Errorf("step %d (%s): 'skill' and 'agent' are mutually exclusive", i+1, step.Name)
		}
		if step.Skill != "" && step.Goal == "" {
			return fmt.Errorf("step %d (%s): 'skill' step requires a 'goal' field", i+1, step.Name)
		}
		if step.Agent != "" && step.Goal == "" {
			return fmt.Errorf("step %d (%s): 'agent' step requires a 'goal' field", i+1, step.Name)
		}

		if step.Tool != "" {
			if step.Args == nil {
				return fmt.Errorf("step %d (%s): missing 'args' field. Every tool step MUST include \"args\": {} (even if empty)", i+1, step.Name)
			}

			if executor != nil {
				schema, exists := executor.GetToolSchema(step.Tool)
				if !exists {
					return fmt.Errorf("step %d (%s): tool '%s' not found. Check tool name spelling", i+1, step.Name, step.Tool)
				}

				if required, ok := schema["required"].([]string); ok {
					for _, reqParam := range required {
						if _, hasArg := step.Args[reqParam]; !hasArg {
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
				if required, ok := schema["required"].([]interface{}); ok {
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

		definedVars[step.Name+"_output"] = true
		definedVars[step.Name+"_goal"] = true
	}

	return nil
}
