package tools

import (
	"context"
	"strings"
)

type contextKey string

const sessionKeyContextKey contextKey = "pepebot_session_key"

// WithSessionKey stores a parent session key for tools executed in this context.
func WithSessionKey(ctx context.Context, sessionKey string) context.Context {
	if strings.TrimSpace(sessionKey) == "" {
		return ctx
	}
	return context.WithValue(ctx, sessionKeyContextKey, sessionKey)
}

// SessionKeyFromContext extracts a parent session key for tool execution.
func SessionKeyFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(sessionKeyContextKey).(string)
	return strings.TrimSpace(v)
}
