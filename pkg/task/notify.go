package task

import (
	"fmt"
)

// NotifyFunc is called when a task event occurs that warrants notification.
type NotifyFunc func(channel, chatID, message string)

// Notifier sends task event notifications to chat channels.
type Notifier struct {
	notify   NotifyFunc
	channels []NotifyChannel
}

// NotifyChannel defines a channel+chatID pair to send notifications to.
type NotifyChannel struct {
	Channel string // e.g., "telegram", "discord"
	ChatID  string
}

// NewNotifier creates a task notifier.
func NewNotifier(fn NotifyFunc, channels []NotifyChannel) *Notifier {
	return &Notifier{notify: fn, channels: channels}
}

// NotifyApprovalNeeded sends a notification that a task needs approval.
func (n *Notifier) NotifyApprovalNeeded(t *Task) {
	if n == nil || n.notify == nil || len(n.channels) == 0 {
		return
	}

	msg := fmt.Sprintf("🔔 *Task needs approval*\n\n"+
		"*%s*\n"+
		"Agent: %s\n"+
		"Priority: %s\n\n"+
		"Reply `/approve %s` or `/reject %s \"reason\"`",
		t.Title, t.AssignedAgent, t.Priority, t.ID, t.ID)

	for _, ch := range n.channels {
		n.notify(ch.Channel, ch.ChatID, msg)
	}
}

// NotifyTaskCompleted sends a notification that a task was completed.
func (n *Notifier) NotifyTaskCompleted(t *Task) {
	if n == nil || n.notify == nil || len(n.channels) == 0 {
		return
	}

	msg := fmt.Sprintf("✅ *Task completed*\n\n"+
		"*%s*\n"+
		"Agent: %s",
		t.Title, t.AssignedAgent)

	if t.Result != "" {
		result := t.Result
		if len(result) > 200 {
			result = result[:200] + "..."
		}
		msg += fmt.Sprintf("\nResult: %s", result)
	}

	for _, ch := range n.channels {
		n.notify(ch.Channel, ch.ChatID, msg)
	}
}

// NotifyTaskFailed sends a notification that a task failed.
func (n *Notifier) NotifyTaskFailed(t *Task) {
	if n == nil || n.notify == nil || len(n.channels) == 0 {
		return
	}

	msg := fmt.Sprintf("❌ *Task failed*\n\n"+
		"*%s*\n"+
		"Agent: %s",
		t.Title, t.AssignedAgent)

	if t.Error != "" {
		errMsg := t.Error
		if len(errMsg) > 200 {
			errMsg = errMsg[:200] + "..."
		}
		msg += fmt.Sprintf("\nError: %s", errMsg)
	}

	for _, ch := range n.channels {
		n.notify(ch.Channel, ch.ChatID, msg)
	}
}
