package providers

import "context"

type ToolCall struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type,omitempty"`
	Function  *FunctionCall          `json:"function,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type LLMResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string     `json:"finish_reason"`
	Usage        *UsageInfo `json:"usage,omitempty"`
}

type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Message struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"` // Can be string or []ContentBlock for multimodal
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ContentBlock represents a piece of content (text, image, or file)
type ContentBlock struct {
	Type     string    `json:"type"` // "text", "image_url", "file"
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
	File     *FileData `json:"file,omitempty"`
}

// ImageURL represents an image URL in vision requests
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "low", "high", or "auto"
}

// FileData represents file data for multimodal requests (documents, audio, video, etc.)
// Format: { "type": "file", "file": { "file_data": "data:mime/type;base64,..." } }
// Reference: https://developers.openai.com/api/docs/guides/pdf-files
type FileData struct {
	FileData string `json:"file_data,omitempty"` // Base64 data URL (e.g., "data:application/pdf;base64,...")
	FileID   string `json:"file_id,omitempty"`   // Uploaded file ID (e.g., "file-xxxxx")
}

// StreamChunk represents a single chunk of streamed LLM output
type StreamChunk struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
}

// StreamCallback is called for each chunk during streaming
type StreamCallback func(chunk StreamChunk)

type LLMProvider interface {
	Chat(ctx context.Context, messages []Message, tools []ToolDefinition, model string, options map[string]interface{}) (*LLMResponse, error)
	ChatStream(ctx context.Context, messages []Message, model string, options map[string]interface{}, callback StreamCallback) error
	GetDefaultModel() string
}

type ToolDefinition struct {
	Type     string                 `json:"type"`
	Function ToolFunctionDefinition `json:"function"`
}

type ToolFunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}
