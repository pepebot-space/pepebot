package workflow

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// mockGoalProcessor simulates an LLM processing a goal.
type mockGoalProcessor struct {
	response string
}

func (m *mockGoalProcessor) ProcessGoal(ctx context.Context, goal string) (string, error) {
	return m.response, nil
}

// mockToolExecutor simulates tool execution and records calls.
type mockToolExecutor struct {
	lastTool string
	lastArgs map[string]interface{}
}

func (m *mockToolExecutor) Execute(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	m.lastTool = name
	m.lastArgs = args
	return fmt.Sprintf("sent: %v", args["content"]), nil
}

func (m *mockToolExecutor) GetToolSchema(name string) (map[string]interface{}, bool) {
	return nil, false
}

// TestGoalStepWithProcessor tests that goal steps use the LLM when goalProcessor is set.
func TestGoalStepWithProcessor(t *testing.T) {
	executor := &mockToolExecutor{}
	helper := &WorkflowHelper{
		workspace: t.TempDir(),
		executor:  executor,
		goalProcessor: &mockGoalProcessor{
			response: "Halo tim ops! Jangan lupa cek #bugs-war ya.\n\nPantun:\nBuah mangga di atas peti,\nJangan lupa cek bugs hari ini!",
		},
	}

	wf := &WorkflowDefinition{
		Name:        "test_goal_workflow",
		Description: "Test goal step processing",
		Steps: []WorkflowStep{
			{
				Name: "generate_message",
				Goal: "Buat pesan pengingat untuk tim operation agar memeriksa channel #bugs-war.",
			},
			{
				Name: "send_reminder",
				Tool: "discord_send",
				Args: map[string]interface{}{
					"channel_id": "924934474846830642",
					"content":    "{{generate_message_output}}",
				},
			},
		},
	}

	result, err := helper.ExecuteWorkflow(context.Background(), wf, nil)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	t.Logf("Workflow output:\n%s", result)

	// Verify the discord_send tool received the LLM output, not the raw goal
	if executor.lastTool != "discord_send" {
		t.Errorf("expected tool 'discord_send', got '%s'", executor.lastTool)
	}

	content, ok := executor.lastArgs["content"].(string)
	if !ok {
		t.Fatal("content arg is not a string")
	}

	if strings.Contains(content, "Buat pesan pengingat") {
		t.Errorf("discord_send received the raw goal text instead of LLM output: %s", content)
	}

	if !strings.Contains(content, "Pantun") {
		t.Errorf("discord_send did not receive LLM-generated content: %s", content)
	}

	t.Logf("discord_send received content: %s", content)
}

// TestGoalStepWithoutProcessor tests that goal steps without a processor fall through
// and do NOT populate the _output variable.
func TestGoalStepWithoutProcessor(t *testing.T) {
	executor := &mockToolExecutor{}
	helper := &WorkflowHelper{
		workspace: t.TempDir(),
		executor:  executor,
		// goalProcessor is nil — simulates the bug
	}

	wf := &WorkflowDefinition{
		Name:        "test_no_processor",
		Description: "Test goal step without processor",
		Steps: []WorkflowStep{
			{
				Name: "generate_message",
				Goal: "Buat pesan pengingat untuk tim ops.",
			},
			{
				Name: "send_reminder",
				Tool: "discord_send",
				Args: map[string]interface{}{
					"channel_id": "123",
					"content":    "{{generate_message_output}}",
				},
			},
		},
	}

	result, err := helper.ExecuteWorkflow(context.Background(), wf, nil)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	t.Logf("Workflow output:\n%s", result)

	// Without a goal processor, generate_message_output is never set,
	// so the template variable remains as-is.
	content, _ := executor.lastArgs["content"].(string)
	t.Logf("discord_send received content: %s", content)

	if content == "Buat pesan pengingat untuk tim ops." {
		t.Error("BUG: discord_send received raw goal text — goalProcessor was not used")
	}
}

// TestGoalStepVariableNaming verifies _output, base name, and _goal variables.
func TestGoalStepVariableNaming(t *testing.T) {
	executor := &mockToolExecutor{}
	llmResponse := "Generated reminder message with pantun"
	helper := &WorkflowHelper{
		workspace:     t.TempDir(),
		executor:      executor,
		goalProcessor: &mockGoalProcessor{response: llmResponse},
	}

	wf := &WorkflowDefinition{
		Name:        "test_variables",
		Description: "Test variable naming",
		Steps: []WorkflowStep{
			{
				Name: "gen",
				Goal: "original goal text",
			},
			{
				Name: "check_output",
				Tool: "echo",
				Args: map[string]interface{}{
					"via_output": "{{gen_output}}",
					"via_base":   "{{gen}}",
					"via_goal":   "{{gen_goal}}",
				},
			},
		},
	}

	_, err := helper.ExecuteWorkflow(context.Background(), wf, nil)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}

	// {{gen_output}} and {{gen}} should be the LLM response
	if v := executor.lastArgs["via_output"]; v != llmResponse {
		t.Errorf("{{gen_output}} = %q, want %q", v, llmResponse)
	}
	if v := executor.lastArgs["via_base"]; v != llmResponse {
		t.Errorf("{{gen}} = %q, want %q", v, llmResponse)
	}
	// {{gen_goal}} should be the original goal text
	if v := executor.lastArgs["via_goal"]; v != "original goal text" {
		t.Errorf("{{gen_goal}} = %q, want %q", v, "original goal text")
	}
}
