package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pepebot-space/pepebot/pkg/bus"
)

// resolveFilePath resolves a file path to an absolute path.
// If it's a URL, return as-is. If relative, resolve against workspace and common directories.
func resolveFilePath(path, workspace string) string {
	// URLs pass through unchanged
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "data:") {
		return path
	}

	// Already absolute and exists — use as-is
	if filepath.IsAbs(path) {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Try to find the file in common locations
	basename := filepath.Base(path)
	candidates := []string{
		path,
		filepath.Join(workspace, path),
		filepath.Join(workspace, basename),
		filepath.Join("/tmp", basename),
		filepath.Join("/tmp/pepebot_whatsapp", basename),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			abs, err := filepath.Abs(candidate)
			if err == nil {
				return abs
			}
			return candidate
		}
	}

	// If nothing found but path is relative, at least make it absolute via workspace
	if !filepath.IsAbs(path) {
		return filepath.Join(workspace, path)
	}

	return path
}

// ─── Telegram Send Tool ───────────────────────────────────────────────────────

type TelegramSendTool struct {
	token     string
	workspace string
}

func NewTelegramSendTool(token, workspace string) *TelegramSendTool {
	return &TelegramSendTool{token: token, workspace: workspace}
}

func (t *TelegramSendTool) Name() string { return "telegram_send" }

func (t *TelegramSendTool) Description() string {
	return "Send a message, image, or file directly to a Telegram chat via the Bot API. Works without the gateway running. Use this for Telegram notifications in workflows."
}

func (t *TelegramSendTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"chat_id": map[string]interface{}{
				"type":        "string",
				"description": "Telegram chat ID (numeric) or @username",
			},
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Message text (supports HTML formatting)",
			},
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "Local file path or URL to send as media/document",
			},
			"caption": map[string]interface{}{
				"type":        "string",
				"description": "Caption for the file/media",
			},
		},
		"required": []string{"chat_id"},
	}
}

func (t *TelegramSendTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	chatID, ok := args["chat_id"].(string)
	if !ok {
		return "", fmt.Errorf("chat_id must be a string")
	}

	text, _ := args["text"].(string)
	filePath, _ := args["file_path"].(string)
	caption, _ := args["caption"].(string)

	if text == "" && filePath == "" {
		return "", fmt.Errorf("either text or file_path must be provided")
	}

	apiBase := fmt.Sprintf("https://api.telegram.org/bot%s", t.token)

	if filePath != "" {
		filePath = resolveFilePath(filePath, t.workspace)
		return t.sendFile(ctx, apiBase, chatID, filePath, caption, text)
	}

	return t.sendText(ctx, apiBase, chatID, text)
}

func (t *TelegramSendTool) sendText(ctx context.Context, apiBase, chatID, text string) (string, error) {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", apiBase+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("telegram API error: %w", err)
	}
	defer resp.Body.Close()

	return parseTelegramResponse(resp.Body)
}

func (t *TelegramSendTool) sendFile(ctx context.Context, apiBase, chatID, filePath, caption, text string) (string, error) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filePath), "."))
	method, fieldName := telegramMethodForExt(ext)

	// For URLs, use JSON API
	if strings.HasPrefix(filePath, "http://") || strings.HasPrefix(filePath, "https://") {
		payload := map[string]interface{}{
			"chat_id": chatID,
			fieldName: filePath,
		}
		if caption != "" {
			payload["caption"] = caption
		} else if text != "" {
			payload["caption"] = text
		}
		body, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, "POST", apiBase+method, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("telegram API error: %w", err)
		}
		defer resp.Body.Close()
		return parseTelegramResponse(resp.Body)
	}

	// Local file — multipart upload
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("cannot open file %q: %w", filePath, err)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("chat_id", chatID)
	if caption != "" {
		_ = w.WriteField("caption", caption)
	} else if text != "" {
		_ = w.WriteField("caption", text)
	}
	fw, err := w.CreateFormFile(fieldName, filepath.Base(filePath))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return "", err
	}
	w.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", apiBase+method, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("telegram API error: %w", err)
	}
	defer resp.Body.Close()
	return parseTelegramResponse(resp.Body)
}

func telegramMethodForExt(ext string) (method, fieldName string) {
	switch ext {
	case "jpg", "jpeg", "png", "gif", "webp":
		return "/sendPhoto", "photo"
	case "mp4", "avi", "mov", "mkv", "webm":
		return "/sendVideo", "video"
	case "mp3", "wav", "ogg", "flac", "m4a":
		return "/sendAudio", "audio"
	default:
		return "/sendDocument", "document"
	}
}

func parseTelegramResponse(body io.Reader) (string, error) {
	var apiResp struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("failed to parse Telegram response: %w", err)
	}
	if !apiResp.OK {
		return "", fmt.Errorf("telegram API error: %s", apiResp.Description)
	}
	var msgResult struct {
		MessageID int `json:"message_id"`
	}
	json.Unmarshal(apiResp.Result, &msgResult)

	out, _ := json.Marshal(map[string]interface{}{
		"success":    true,
		"message_id": msgResult.MessageID,
	})
	return string(out), nil
}

// ─── Discord Send Tool ────────────────────────────────────────────────────────

type DiscordSendTool struct {
	token     string
	workspace string
}

func NewDiscordSendTool(token, workspace string) *DiscordSendTool {
	return &DiscordSendTool{token: token, workspace: workspace}
}

func (t *DiscordSendTool) Name() string { return "discord_send" }

func (t *DiscordSendTool) Description() string {
	return "Send a message or file directly to a Discord channel via the API. Works without the gateway running. Use this for Discord notifications in workflows."
}

func (t *DiscordSendTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"channel_id": map[string]interface{}{
				"type":        "string",
				"description": "Discord channel ID (numeric string)",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Message text (max 2000 characters)",
			},
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "Local file path to attach to the message",
			},
		},
		"required": []string{"channel_id"},
	}
}

func (t *DiscordSendTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	channelID, ok := args["channel_id"].(string)
	if !ok {
		return "", fmt.Errorf("channel_id must be a string")
	}

	content, _ := args["content"].(string)
	filePath, _ := args["file_path"].(string)

	if content == "" && filePath == "" {
		return "", fmt.Errorf("either content or file_path must be provided")
	}

	// Truncate content to Discord's limit
	if len(content) > 2000 {
		content = content[:2000]
	}

	if filePath != "" {
		filePath = resolveFilePath(filePath, t.workspace)
	}

	apiURL := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", channelID)

	var req *http.Request
	var err error
	if filePath != "" {
		req, err = t.buildFileRequest(ctx, apiURL, content, filePath)
	} else {
		req, err = t.buildTextRequest(ctx, apiURL, content)
	}
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bot "+t.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("discord API error: %w", err)
	}
	defer resp.Body.Close()

	return parseDiscordResponse(resp)
}

func (t *DiscordSendTool) buildTextRequest(ctx context.Context, apiURL, content string) (*http.Request, error) {
	body, _ := json.Marshal(map[string]string{"content": content})
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (t *DiscordSendTool) buildFileRequest(ctx context.Context, apiURL, content, filePath string) (*http.Request, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open file %q: %w", filePath, err)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// payload_json part
	payloadJSON := map[string]string{}
	if content != "" {
		payloadJSON["content"] = content
	}
	payloadBytes, _ := json.Marshal(payloadJSON)
	payloadPart, err := w.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="payload_json"`},
		"Content-Type":        {"application/json"},
	})
	if err != nil {
		return nil, err
	}
	payloadPart.Write(payloadBytes)

	// File attachment part
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filePath), "."))
	mimeType := mime.TypeByExtension("." + ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	filePart, err := w.CreatePart(map[string][]string{
		"Content-Disposition": {fmt.Sprintf(`form-data; name="files[0]"; filename="%s"`, filepath.Base(filePath))},
		"Content-Type":        {mimeType},
	})
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(filePart, f); err != nil {
		return nil, err
	}
	w.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req, nil
}

func parseDiscordResponse(resp *http.Response) (string, error) {
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("discord API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	var msg struct {
		ID string `json:"id"`
	}
	json.Unmarshal(respBody, &msg)

	out, _ := json.Marshal(map[string]interface{}{
		"success":    true,
		"message_id": msg.ID,
	})
	return string(out), nil
}

// ─── WhatsApp Send Tool (gateway HTTP — for CLI/workflow use) ─────────────────

// WhatsAppSendHTTPTool sends WhatsApp messages by forwarding to the running gateway's
// POST /v1/send endpoint. Use this in CLI/workflow mode when no local bus is available.
type WhatsAppSendHTTPTool struct {
	gatewayURL string
	workspace  string
}

func NewWhatsAppSendViaGateway(host string, port int, workspace string) *WhatsAppSendHTTPTool {
	return &WhatsAppSendHTTPTool{
		gatewayURL: fmt.Sprintf("http://%s:%d/v1/send", host, port),
		workspace:  workspace,
	}
}

func (t *WhatsAppSendHTTPTool) Name() string { return "whatsapp_send" }

func (t *WhatsAppSendHTTPTool) Description() string {
	return "Send a message or file to a WhatsApp contact or group. Requires the gateway to be running. Use the JID format: 628123456789@s.whatsapp.net for contacts, or groupid@g.us for groups."
}

func (t *WhatsAppSendHTTPTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"jid": map[string]interface{}{
				"type":        "string",
				"description": "WhatsApp JID, e.g. 628123456789@s.whatsapp.net or groupid@g.us",
			},
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Text message to send",
			},
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "Local file path to send as media",
			},
			"caption": map[string]interface{}{
				"type":        "string",
				"description": "Caption for the media file",
			},
		},
		"required": []string{"jid"},
	}
}

func (t *WhatsAppSendHTTPTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	jid, ok := args["jid"].(string)
	if !ok {
		return "", fmt.Errorf("jid must be a string")
	}

	text, _ := args["text"].(string)
	filePath, _ := args["file_path"].(string)
	caption, _ := args["caption"].(string)

	if text == "" && filePath == "" {
		return "", fmt.Errorf("either text or file_path must be provided")
	}

	media := []string{}
	if filePath != "" {
		media = append(media, resolveFilePath(filePath, t.workspace))
	}

	content := text
	if content == "" {
		content = caption
	}

	payload := map[string]interface{}{
		"channel": "whatsapp",
		"chat_id": jid,
		"content": content,
		"media":   media,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", t.gatewayURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gateway not reachable — is `pepebot gateway` running? (%w)", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("gateway error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	return string(respBody), nil
}

// ─── WhatsApp Send Tool (bus — for gateway/agent use) ─────────────────────────

type WhatsAppSendTool struct {
	bus       *bus.MessageBus
	workspace string
}

func NewWhatsAppSendTool(b *bus.MessageBus, workspace string) *WhatsAppSendTool {
	return &WhatsAppSendTool{bus: b, workspace: workspace}
}

func (t *WhatsAppSendTool) Name() string { return "whatsapp_send" }

func (t *WhatsAppSendTool) Description() string {
	return "Send a message or file to a WhatsApp contact or group. Requires the gateway to be running. Use the JID format: 628123456789@s.whatsapp.net for contacts, or groupid@g.us for groups."
}

func (t *WhatsAppSendTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"jid": map[string]interface{}{
				"type":        "string",
				"description": "WhatsApp JID, e.g. 628123456789@s.whatsapp.net or groupid@g.us",
			},
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Text message to send",
			},
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "Local file path to send as media",
			},
			"caption": map[string]interface{}{
				"type":        "string",
				"description": "Caption for the media file",
			},
		},
		"required": []string{"jid"},
	}
}

func (t *WhatsAppSendTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	jid, ok := args["jid"].(string)
	if !ok {
		return "", fmt.Errorf("jid must be a string")
	}

	text, _ := args["text"].(string)
	filePath, _ := args["file_path"].(string)
	caption, _ := args["caption"].(string)

	if text == "" && filePath == "" {
		return "", fmt.Errorf("either text or file_path must be provided")
	}

	media := []string{}
	if filePath != "" {
		media = append(media, resolveFilePath(filePath, t.workspace))
	}

	content := text
	if content == "" {
		content = caption
	}

	t.bus.PublishOutbound(bus.OutboundMessage{
		Channel: "whatsapp",
		ChatID:  jid,
		Content: content,
		Media:   media,
	})

	out, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"note":    "Message queued for WhatsApp delivery. Requires gateway to be running.",
	})
	return string(out), nil
}
