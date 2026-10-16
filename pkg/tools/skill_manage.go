// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/pepebot-space/pepebot/pkg/skills"
)

// SkillManageTool is the agent's procedural memory: what it worked out once and
// should not have to work out again.
//
// It is deliberately narrower than write_file, which the agent already has.
// write_file can put anything anywhere in the workspace; this writes a
// SKILL.md with the frontmatter the loader needs, under a validated name,
// marked as the agent's own — which is what makes every one of them findable
// and removable later.
type SkillManageTool struct {
	author *skills.AgentAuthor
	loader *skills.SkillsLoader
}

func NewSkillManageTool(workspace string, loader *skills.SkillsLoader) *SkillManageTool {
	return &SkillManageTool{author: skills.NewAgentAuthor(workspace), loader: loader}
}

func (t *SkillManageTool) Name() string { return "skill_manage" }

func (t *SkillManageTool) Description() string {
	return "Save a procedure you worked out as a reusable skill, so future sessions can follow it instead of rediscovering it. " +
		"Use it after solving something non-trivial that will come up again — a multi-step workflow, a tool's quirks, " +
		"a convention this user keeps correcting you on. Do not save one-off answers, facts (use the memory tool), " +
		"or anything already covered by an existing skill: patch that one instead. " +
		"You can only patch or delete skills you wrote yourself."
}

func (t *SkillManageTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"create", "patch", "delete", "list"},
				"description": "create a new skill, patch one you wrote, delete one you wrote, or list your own",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "skill name: lowercase letters, digits and dashes (e.g. deploy-pepebot)",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "one line saying when this skill should be opened — it is all a future session sees before deciding",
			},
			"body": map[string]interface{}{
				"type":        "string",
				"description": "the procedure itself, in markdown: steps, commands, pitfalls",
			},
		},
		"required": []string{"action"},
	}
}

func (t *SkillManageTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	action, _ := args["action"].(string)
	name, _ := args["name"].(string)
	description, _ := args["description"].(string)
	body, _ := args["body"].(string)

	switch strings.ToLower(action) {
	case "create":
		if err := t.author.Create(name, description, body); err != nil {
			return "", err
		}
		return fmt.Sprintf("Saved skill %q. Future sessions will see its description and can open it when relevant.", name), nil

	case "patch":
		if err := t.author.Patch(name, description, body); err != nil {
			return "", err
		}
		return fmt.Sprintf("Updated skill %q.", name), nil

	case "delete":
		if err := t.author.Delete(name); err != nil {
			return "", err
		}
		return fmt.Sprintf("Removed skill %q.", name), nil

	case "list":
		var mine []string
		for _, s := range t.loader.ListSkills(false) {
			if t.author.IsAgentOwned(s.Name) {
				mine = append(mine, fmt.Sprintf("- %s: %s", s.Name, s.Description))
			}
		}
		if len(mine) == 0 {
			return "You have not written any skills yet.", nil
		}
		return strings.Join(mine, "\n"), nil

	default:
		return "", fmt.Errorf("unknown action %q: use create, patch, delete or list", action)
	}
}
