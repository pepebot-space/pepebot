package task

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TaskTemplate defines a reusable task blueprint.
type TaskTemplate struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Priority    TaskPriority      `json:"priority,omitempty"`
	Agent       string            `json:"agent,omitempty"`
	Labels      []string          `json:"labels,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
	SubTasks    []TaskTemplate    `json:"sub_tasks,omitempty"`
	Approval    bool              `json:"approval,omitempty"`
}

// TemplateLoader manages task templates stored as JSON files.
type TemplateLoader struct {
	dir string
}

// NewTemplateLoader creates a loader for the given directory.
func NewTemplateLoader(workspacePath string) *TemplateLoader {
	dir := filepath.Join(workspacePath, "task-templates")
	os.MkdirAll(dir, 0755)
	return &TemplateLoader{dir: dir}
}

// List returns all available template names.
func (l *TemplateLoader) List() []string {
	entries, err := os.ReadDir(l.dir)
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

// Load reads a template by name.
func (l *TemplateLoader) Load(name string) (*TaskTemplate, error) {
	if !strings.HasSuffix(name, ".json") {
		name += ".json"
	}
	data, err := os.ReadFile(filepath.Join(l.dir, name))
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}
	var tmpl TaskTemplate
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("invalid template: %w", err)
	}
	return &tmpl, nil
}

// Save writes a template to disk.
func (l *TemplateLoader) Save(name string, tmpl *TaskTemplate) error {
	if !strings.HasSuffix(name, ".json") {
		name += ".json"
	}
	data, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(l.dir, name), data, 0644)
}

// Delete removes a template.
func (l *TemplateLoader) Delete(name string) error {
	if !strings.HasSuffix(name, ".json") {
		name += ".json"
	}
	return os.Remove(filepath.Join(l.dir, name))
}

// CreateTasksFromTemplate instantiates tasks from a template, interpolating variables.
// Returns the created parent task and any sub-tasks.
func CreateTasksFromTemplate(store TaskStore, tmpl *TaskTemplate, vars map[string]string, createdBy string) ([]*Task, error) {
	// Merge variables: template defaults + overrides
	merged := make(map[string]string)
	for k, v := range tmpl.Variables {
		merged[k] = v
	}
	for k, v := range vars {
		merged[k] = v
	}

	parent := &Task{
		Title:       interpolate(tmpl.Name, merged),
		Description: interpolate(tmpl.Description, merged),
		Priority:    tmpl.Priority,
		Labels:      tmpl.Labels,
		Approval:    tmpl.Approval,
		CreatedBy:   createdBy,
		TemplateID:  tmpl.Name,
	}
	if parent.Priority == "" {
		parent.Priority = TaskPriorityMedium
	}
	if tmpl.Agent != "" {
		parent.AssignedAgent = tmpl.Agent
	}

	if err := store.Create(parent); err != nil {
		return nil, fmt.Errorf("failed to create parent task: %w", err)
	}

	created := []*Task{parent}

	// Create sub-tasks
	for _, sub := range tmpl.SubTasks {
		child := &Task{
			Title:       interpolate(sub.Name, merged),
			Description: interpolate(sub.Description, merged),
			Priority:    sub.Priority,
			Labels:      sub.Labels,
			Approval:    sub.Approval,
			ParentID:    parent.ID,
			CreatedBy:   createdBy,
			TemplateID:  tmpl.Name,
		}
		if child.Priority == "" {
			child.Priority = parent.Priority
		}
		if sub.Agent != "" {
			child.AssignedAgent = sub.Agent
		}

		if err := store.Create(child); err != nil {
			return created, fmt.Errorf("failed to create sub-task '%s': %w", sub.Name, err)
		}
		created = append(created, child)
	}

	return created, nil
}

func interpolate(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}
