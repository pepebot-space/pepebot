// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pepebot-space/pepebot/pkg/bus"
	"github.com/pepebot-space/pepebot/pkg/logger"
	"github.com/pepebot-space/pepebot/pkg/providers"
	"github.com/pepebot-space/pepebot/pkg/tasks"
)

// This file is where delegation meets the conversation: pepebot's own agent
// loop as an executor, and the report that goes back to whoever asked.
//
// The reporting matters as much as the running. A task whose result lands in a
// log file has not been delegated, it has been lost — so a finished task is
// read by the agent and answered in the chat that asked for it, in that
// chat's own session.

// internalExecutor runs a delegated task on pepebot's own agent loop, with its
// tools, in a session of its own.
//
// A session of its own because a long job should not pour its intermediate
// turns into the conversation a human is having; the chat gets the conclusion,
// not the working.
func (al *AgentLoop) internalExecutor() tasks.Executor {
	return tasks.FuncExecutor(func(ctx context.Context, t *tasks.Task) (string, error) {
		prompt := t.Prompt
		if t.Depth > 0 {
			prompt = fmt.Sprintf("%s\n\n(Ini sub-tugas tingkat %d. Jangan mendelegasikan lebih dalam dari yang perlu.)", prompt, t.Depth)
		}
		return al.ProcessDirect(ctx, prompt, nil, "task:"+t.ID)
	})
}

// TaskFinished is called by the task manager when a delegated job reaches a
// terminal state. It reads the result and answers in the chat that asked.
func (al *AgentLoop) TaskFinished(t *tasks.Task) {
	if t.Origin.Channel == "" || t.Origin.ChatID == "" {
		logger.InfoCF("tasks", "Task finished with no chat to report to", map[string]interface{}{
			"task": t.ID, "status": t.Status,
		})
		return
	}

	al.sendToOrigin(t, al.summariseTask(t))
}

// TaskStale reports work that a restart interrupted, and asks what to do about
// it. The user decided this: tell the requester and let them choose, rather
// than retrying on their behalf or quietly dropping it.
func (al *AgentLoop) TaskStale(t *tasks.Task) {
	if t.Origin.Channel == "" || t.Origin.ChatID == "" {
		return
	}
	name := t.Label
	if name == "" {
		name = truncateString(t.Prompt, 60)
	}
	al.sendToOrigin(t, fmt.Sprintf(
		"⚠️ Tugas `%s` (%s) terhenti karena pepebot restart, jadi tidak ada yang menyelesaikannya.\n"+
			"Mau dilanjutkan? Balas \"lanjutkan %s\" untuk menjalankan ulang, atau \"batalkan %s\".",
		t.ID, name, t.ID, t.ID))
}

// summariseTask turns a raw result into something worth reading.
//
// The agent reads it rather than the chat getting a dump: a delegated job
// answers in the shape of the question that started it. A failure is reported
// plainly instead of being dressed up.
func (al *AgentLoop) summariseTask(t *tasks.Task) string {
	name := t.Label
	if name == "" {
		name = truncateString(t.Prompt, 60)
	}

	if t.Status != tasks.StatusDone {
		msg := fmt.Sprintf("❌ Tugas `%s` (%s) %s.", t.ID, name, t.Status)
		if t.Error != "" {
			msg += "\n" + truncateString(t.Error, 400)
		}
		return msg
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	prompt := fmt.Sprintf(`Sebuah tugas yang kamu delegasikan sudah selesai. Laporkan hasilnya kepada orang yang memintanya.

Tugas: %s
Hasil mentah:
%s

Tulis balasan singkat dalam bahasa yang sama dengan permintaan aslinya: apa yang ditemukan atau dikerjakan, dan apa yang perlu diputuskan orang itu kalau ada. Jangan menyalin hasil mentah apa adanya, jangan memakai basa-basi pembuka.`,
		t.Prompt, t.Result)

	response, err := al.provider.Chat(ctx, []providers.Message{{Role: "user", Content: prompt}},
		nil, al.model, al.chatOptions())
	if err != nil || strings.TrimSpace(response.Content) == "" {
		// The summary is a convenience; the result is the point. If the model
		// is unavailable the raw answer still gets delivered.
		logger.WarnCF("tasks", "Could not summarise task result; sending it raw", map[string]interface{}{
			"task": t.ID, "error": fmt.Sprintf("%v", err),
		})
		return fmt.Sprintf("✅ Tugas `%s` (%s) selesai:\n\n%s", t.ID, name, t.Result)
	}

	out := fmt.Sprintf("✅ Tugas `%s` (%s) selesai.\n\n%s", t.ID, name, strings.TrimSpace(response.Content))
	if t.ResultPath != "" {
		out += fmt.Sprintf("\n\n_Hasil lengkap: %s_", t.ResultPath)
	}
	return out
}

func (al *AgentLoop) sendToOrigin(t *tasks.Task, content string) {
	if al.bus == nil {
		return
	}
	if t.Origin.Requester != "" && t.Origin.Channel == "discord" {
		content = fmt.Sprintf("<@%s> %s", t.Origin.Requester, content)
	}
	al.bus.PublishOutbound(bus.OutboundMessage{
		Channel: t.Origin.Channel,
		ChatID:  t.Origin.ChatID,
		Content: content,
	})
}
