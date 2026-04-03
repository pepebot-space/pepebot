package task

import (
	"github.com/pepebot-space/pepebot/pkg/logger"
)

// RunCleanup performs TTL-based cleanup on the task store.
// Intended to be called periodically (e.g., from heartbeat service).
func RunCleanup(store TaskStore, doneTTLDays, failedTTLDays int) {
	if store == nil {
		return
	}

	deleted, err := store.Cleanup(doneTTLDays, failedTTLDays)
	if err != nil {
		logger.WarnCF("task", "Cleanup failed", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	if deleted > 0 {
		logger.InfoCF("task", "Cleaned up expired tasks", map[string]interface{}{
			"deleted":    deleted,
			"done_ttl":   doneTTLDays,
			"failed_ttl": failedTTLDays,
		})
	}
}
