// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pepebot-space/pepebot/pkg/logger"
	"github.com/pepebot-space/pepebot/pkg/memory"
	"github.com/pepebot-space/pepebot/pkg/providers"
	"github.com/pepebot-space/pepebot/pkg/skills"
)

// reviewPrompt asks for memory operations and nothing else. The review model
// never talks to the user, so plain JSON back is enough — no tool-calling loop,
// which also means it cannot touch the filesystem or the network.
const reviewPrompt = `You maintain an AI agent's long-term memory. Read the conversation below and decide whether anything is worth remembering for FUTURE sessions.

Save only durable facts: the user's preferences and working style, their projects, environment and conventions, and corrections they had to repeat. Never save what is specific to this conversation, anything you can look up again, or secrets (keys, tokens, passwords).

Most turns need nothing. Returning an empty list is the normal, correct answer.

Current memory:
%s

Conversation:
%s

You may also save a PROCEDURE as a skill: something non-trivial that was worked out in this conversation and will come up again — a multi-step workflow, a tool's quirks, a convention the user keeps correcting. A skill is read only when it is relevant, so it can be longer than a memory entry; a fact that must always be known is memory, not a skill.

Existing skills (do not duplicate one — patch it instead):
%s

Reply with JSON only:
{"ops": [{"action": "add|replace|remove", "target": "memory|user", "content": "...", "old_text": "..."}],
 "skills": [{"action": "create|patch", "name": "kebab-case-name", "description": "when to open this", "body": "the procedure in markdown"}]}
- target "user" is the user's own profile, "memory" is everything else.
- Use replace or remove to correct or drop an entry that is now wrong; old_text is a substring of it.
- Keep each entry one short sentence. Space is limited, so prefer replacing a related entry over adding a near-duplicate.
- Skills are rare. Most conversations produce none; an empty list is the normal answer. Never save a skill for a one-off answer, for something a single memory entry covers, or for anything already in the list above.`

// reviewSkill is a procedure the review wants kept. Separate from reviewOp
// because the two land in different places: memory is a line in a bounded file,
// a skill is a document that is only read when it is relevant.
type reviewSkill struct {
	Action      string `json:"action"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
}

// reviewOp is one proposed change.
type reviewOp struct {
	Action  string `json:"action"`
	Target  string `json:"target"`
	Content string `json:"content"`
	OldText string `json:"old_text"`
}

// turnsSinceReview counts turns per session so the review runs on a nudge
// interval rather than after every message.
var reviewCounters sync.Map

// maybeReview runs the background memory review if this session is due for one.
//
// It is the self-improvement half of the memory system: without it, memory only
// ever records what the user explicitly asked to be remembered, and a
// correction the user has made three times is still not learned.
func (al *AgentLoop) maybeReview(sessionKey string) {
	cfg := al.memoryCfg
	if !cfg.Enabled || !cfg.Review || cfg.ReviewInterval <= 0 {
		return
	}

	v, _ := reviewCounters.LoadOrStore(sessionKey, new(int64))
	if atomic.AddInt64(v.(*int64), 1)%int64(cfg.ReviewInterval) != 0 {
		return
	}

	// One review per session at a time; a slow one must not stack up behind the
	// next turn.
	if _, busy := al.reviewing.LoadOrStore(sessionKey, true); busy {
		return
	}
	go func() {
		defer al.reviewing.Delete(sessionKey)
		al.reviewTurn(sessionKey)
	}()
}

func (al *AgentLoop) reviewTurn(sessionKey string) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	history := al.sessions.GetHistory(sessionKey)
	if len(history) == 0 {
		return
	}
	// Recent turns only: the durable facts worth keeping show up in what was
	// just said, and a long replay would cost more than the memory is worth.
	if len(history) > 12 {
		history = history[len(history)-12:]
	}

	var convo strings.Builder
	for _, m := range history {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		text := contentText(m.Content)
		if text == "" {
			continue
		}
		fmt.Fprintf(&convo, "%s: %s\n", m.Role, truncateString(text, 1500))
	}
	if convo.Len() == 0 {
		return
	}

	notes, profile := al.memoryStores()
	current := strings.TrimSpace(notes.Render() + "\n\n" + profile.Render())
	if current == "" {
		current = "(empty)"
	}

	existing := "(none)"
	if summary := al.contextBuilder.SkillsLoader().BuildSkillsSummary(); summary != "" {
		existing = truncateString(summary, 4000)
	}

	prompt := fmt.Sprintf(reviewPrompt, current, convo.String(), existing)
	response, err := al.provider.Chat(ctx, []providers.Message{{Role: "user", Content: prompt}}, nil, al.model, map[string]interface{}{
		"max_tokens":  1024,
		"temperature": 0.2,
	})
	if err != nil {
		logger.WarnCF("memory", "Background review failed", map[string]interface{}{"error": err.Error()})
		return
	}

	ops, proposed, err := parseReviewOps(response.Content)
	if err != nil {
		logger.DebugCF("memory", "Review returned no usable operations", map[string]interface{}{
			"error": err.Error(), "reply": truncateString(response.Content, 200),
		})
		return
	}

	for _, op := range ops {
		store := notes
		if strings.EqualFold(op.Target, "user") {
			store = profile
		}

		var applyErr error
		switch strings.ToLower(op.Action) {
		case "add":
			applyErr = store.Add(op.Content)
		case "replace":
			applyErr = store.Replace(op.OldText, op.Content)
		case "remove":
			applyErr = store.Remove(op.OldText)
		default:
			continue
		}

		switch {
		case applyErr == nil:
			logger.InfoCF("memory", "Memory updated by review", map[string]interface{}{
				"action": op.Action, "target": op.Target,
				"entry": truncateString(op.Content, 120),
			})
		case errors.Is(applyErr, memory.ErrDuplicate):
			// Already known — the common case, and not worth a log line.
		default:
			// A full store or a refused entry is expected, not a fault: the next
			// foreground turn can consolidate with the memory tool.
			logger.DebugCF("memory", "Review write rejected", map[string]interface{}{
				"action": op.Action, "error": applyErr.Error(),
			})
		}
	}
	al.applyReviewSkills(proposed)
}

// applyReviewSkills writes what the review decided is worth keeping as a
// procedure. It goes through the same author as the tool, so a skill written
// unattended gets the same name validation, the same content scan, the same
// size limit and the same created_by marker as one the agent wrote on purpose.
func (al *AgentLoop) applyReviewSkills(proposed []reviewSkill) {
	if !al.skillsManaged || len(proposed) == 0 {
		return
	}
	author := skills.NewAgentAuthor(al.workspace)

	for _, p := range proposed {
		var err error
		switch strings.ToLower(p.Action) {
		case "create":
			err = author.Create(p.Name, p.Description, p.Body)
		case "patch":
			err = author.Patch(p.Name, p.Description, p.Body)
		default:
			continue
		}

		if err != nil {
			// Refused names, duplicates and foreign skills are the expected
			// outcome, not a fault: the review proposes, the author decides.
			logger.DebugCF("skills", "Review skill proposal rejected", map[string]interface{}{
				"skill": p.Name, "action": p.Action, "error": err.Error(),
			})
			continue
		}
		logger.InfoCF("skills", "Skill written by background review", map[string]interface{}{
			"skill": p.Name, "action": p.Action, "description": truncateString(p.Description, 120),
		})
	}
}

// parseReviewOps pulls the ops out of a reply that may be wrapped in prose or a
// code fence, which small models do regardless of instructions.
func parseReviewOps(reply string) ([]reviewOp, []reviewSkill, error) {
	start := strings.Index(reply, "{")
	end := strings.LastIndex(reply, "}")
	if start < 0 || end <= start {
		return nil, nil, errors.New("no JSON object in reply")
	}

	var parsed struct {
		Ops    []reviewOp    `json:"ops"`
		Skills []reviewSkill `json:"skills"`
	}
	if err := json.Unmarshal([]byte(reply[start:end+1]), &parsed); err != nil {
		return nil, nil, err
	}
	return parsed.Ops, parsed.Skills, nil
}

// contentText flattens a message body to text. A multimodal turn carries blocks
// rather than a string, and the review only has any use for the words.
func contentText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []providers.ContentBlock:
		var parts []string
		for _, b := range v {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}
