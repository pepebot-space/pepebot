package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PNG file signature (first 8 bytes)
var pngSignature = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

// AdbHelper provides ADB command execution utilities
type AdbHelper struct {
	adbPath   string
	workspace string
}

// NewAdbHelper creates a new ADB helper, discovering the ADB binary location
func NewAdbHelper(workspace string) (*AdbHelper, error) {
	// Try ANDROID_HOME first
	if androidHome := os.Getenv("ANDROID_HOME"); androidHome != "" {
		adbPath := filepath.Join(androidHome, "platform-tools", "adb")
		if _, err := os.Stat(adbPath); err == nil {
			return &AdbHelper{adbPath: adbPath, workspace: workspace}, nil
		}
	}

	// Try system PATH
	adbPath, err := exec.LookPath("adb")
	if err == nil {
		return &AdbHelper{adbPath: adbPath, workspace: workspace}, nil
	}

	return nil, fmt.Errorf("adb binary not found in ANDROID_HOME or PATH")
}

// execAdb executes an ADB command with optional device serial and returns string output
func (h *AdbHelper) execAdb(ctx context.Context, device string, timeout time.Duration, args ...string) (string, error) {
	cmdArgs := []string{}
	if device != "" {
		cmdArgs = append(cmdArgs, "-s", device)
	}
	cmdArgs = append(cmdArgs, args...)

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, h.adbPath, cmdArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("adb command timed out after %s", timeout)
		}
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = stdout.String()
		}
		return "", fmt.Errorf("adb command failed: %w: %s", err, strings.TrimSpace(errMsg))
	}

	return stdout.String(), nil
}

// execAdbBinary executes an ADB command and returns raw binary output (for exec-out)
func (h *AdbHelper) execAdbBinary(ctx context.Context, device string, timeout time.Duration, args ...string) ([]byte, error) {
	cmdArgs := []string{}
	if device != "" {
		cmdArgs = append(cmdArgs, "-s", device)
	}
	cmdArgs = append(cmdArgs, args...)

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, h.adbPath, cmdArgs...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("adb command timed out after %s", timeout)
		}
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = "(no error details)"
		}
		return nil, fmt.Errorf("adb command failed: %w: %s", err, strings.TrimSpace(errMsg))
	}

	return stdout.Bytes(), nil
}

// resolvePath resolves relative paths to workspace directory
func (h *AdbHelper) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(h.workspace, path)
}

// escapeAdbText escapes text for adb shell input text command
func escapeAdbText(text string) string {
	// Characters that need escaping for adb shell
	replacer := strings.NewReplacer(
		" ", "%s",
		"\t", "%s",
		"\\", "\\\\",
		"\"", "\\\"",
		"'", "\\'",
		"$", "\\$",
		"`", "\\`",
		"(", "\\(",
		")", "\\)",
		"&", "\\&",
		"|", "\\|",
		";", "\\;",
		"<", "\\<",
		">", "\\>",
		"*", "\\*",
		"?", "\\?",
		"#", "\\#",
		"~", "\\~",
		"!", "\\!",
		"{", "\\{",
		"}", "\\}",
		"[", "\\[",
		"]", "\\]",
	)
	return replacer.Replace(text)
}

// chunkText splits text into chunks of maxLen
func chunkText(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}
	var chunks []string
	for len(text) > 0 {
		end := maxLen
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, text[:end])
		text = text[end:]
	}
	return chunks
}

// ==================== ADB Devices Tool ====================

type AdbDevicesTool struct {
	helper *AdbHelper
}

func NewAdbDevicesTool(helper *AdbHelper) *AdbDevicesTool {
	return &AdbDevicesTool{helper: helper}
}

func (t *AdbDevicesTool) Name() string {
	return "adb_devices"
}

func (t *AdbDevicesTool) Description() string {
	return "List all connected Android devices via ADB. Returns device serial numbers and their connection status."
}

func (t *AdbDevicesTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *AdbDevicesTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	output, err := t.helper.execAdb(ctx, "", 10*time.Second, "devices", "-l")
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	devices := []map[string]string{}

	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			device := map[string]string{
				"serial": parts[0],
				"status": parts[1],
			}
			// Parse additional info (model, device name, etc.)
			for _, part := range parts[2:] {
				kv := strings.SplitN(part, ":", 2)
				if len(kv) == 2 {
					device[kv[0]] = kv[1]
				}
			}
			devices = append(devices, device)
		}
	}

	result, _ := json.MarshalIndent(devices, "", "  ")
	return string(result), nil
}

// ==================== ADB Shell Tool ====================

type AdbShellTool struct {
	helper *AdbHelper
}

func NewAdbShellTool(helper *AdbHelper) *AdbShellTool {
	return &AdbShellTool{helper: helper}
}

func (t *AdbShellTool) Name() string {
	return "adb_shell"
}

func (t *AdbShellTool) Description() string {
	return "Execute a shell command on the connected Android device via ADB. Useful for running system commands, checking properties, or interacting with the device."
}

func (t *AdbShellTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Shell command to execute on the device",
			},
			"device": map[string]interface{}{
				"type":        "string",
				"description": "Device serial number (optional, uses default device if not specified)",
			},
		},
		"required": []string{"command"},
	}
}

func (t *AdbShellTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command is required")
	}

	device, _ := args["device"].(string)

	output, err := t.helper.execAdb(ctx, device, 30*time.Second, "shell", command)
	if err != nil {
		return "", err
	}

	maxLen := 10000
	if len(output) > maxLen {
		output = output[:maxLen] + fmt.Sprintf("\n... (truncated, %d more chars)", len(output)-maxLen)
	}

	return output, nil
}

// ==================== ADB Tap Tool ====================

type AdbTapTool struct {
	helper *AdbHelper
}

func NewAdbTapTool(helper *AdbHelper) *AdbTapTool {
	return &AdbTapTool{helper: helper}
}

func (t *AdbTapTool) Name() string {
	return "adb_tap"
}

func (t *AdbTapTool) Description() string {
	return "Simulate a tap or long press gesture at specific screen coordinates on the Android device. Supports single tap, multi-tap, and long press."
}

func (t *AdbTapTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"x": map[string]interface{}{
				"type":        "number",
				"description": "X coordinate for tap",
			},
			"y": map[string]interface{}{
				"type":        "number",
				"description": "Y coordinate for tap",
			},
			"count": map[string]interface{}{
				"type":        "number",
				"description": "Number of taps (default: 1, use 2 for double-tap)",
			},
			"long_press": map[string]interface{}{
				"type":        "boolean",
				"description": "If true, perform a long press instead of tap (holds for 550ms)",
			},
			"device": map[string]interface{}{
				"type":        "string",
				"description": "Device serial number (optional)",
			},
		},
		"required": []string{"x", "y"},
	}
}

func (t *AdbTapTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	x, ok := args["x"].(float64)
	if !ok {
		return "", fmt.Errorf("x coordinate is required")
	}

	y, ok := args["y"].(float64)
	if !ok {
		return "", fmt.Errorf("y coordinate is required")
	}

	device, _ := args["device"].(string)
	xs := fmt.Sprintf("%d", int(x))
	ys := fmt.Sprintf("%d", int(y))

	longPress, _ := args["long_press"].(bool)
	if longPress {
		// Long press: swipe from same point to same point with 550ms duration
		_, err := t.helper.execAdb(ctx, device, 10*time.Second,
			"shell", "input", "swipe", xs, ys, xs, ys, "550")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Long pressed at (%s, %s) for 550ms", xs, ys), nil
	}

	count := 1
	if c, ok := args["count"].(float64); ok && int(c) > 1 {
		count = int(c)
	}

	for i := 0; i < count; i++ {
		_, err := t.helper.execAdb(ctx, device, 8*time.Second,
			"shell", "input", "tap", xs, ys)
		if err != nil {
			return "", err
		}
	}

	if count > 1 {
		return fmt.Sprintf("Tapped %d times at (%s, %s)", count, xs, ys), nil
	}
	return fmt.Sprintf("Tapped at (%s, %s)", xs, ys), nil
}

// ==================== ADB Input Text Tool ====================

type AdbInputTextTool struct {
	helper *AdbHelper
}

func NewAdbInputTextTool(helper *AdbHelper) *AdbInputTextTool {
	return &AdbInputTextTool{helper: helper}
}

func (t *AdbInputTextTool) Name() string {
	return "adb_input_text"
}

func (t *AdbInputTextTool) Description() string {
	return "Input text into the currently focused field on the Android device. Text is automatically chunked and escaped for reliable input. Optionally sends Enter key after input."
}

func (t *AdbInputTextTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Text to input",
			},
			"press_enter": map[string]interface{}{
				"type":        "boolean",
				"description": "If true, press Enter (keyevent 66) after inputting text (default: false)",
			},
			"device": map[string]interface{}{
				"type":        "string",
				"description": "Device serial number (optional)",
			},
		},
		"required": []string{"text"},
	}
}

func (t *AdbInputTextTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	text, ok := args["text"].(string)
	if !ok {
		return "", fmt.Errorf("text is required")
	}

	device, _ := args["device"].(string)
	pressEnter, _ := args["press_enter"].(bool)

	// Split by newlines, input each line separately
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line == "" && i < len(lines)-1 {
			// Send Enter for empty lines
			_, err := t.helper.execAdb(ctx, device, 8*time.Second,
				"shell", "input", "keyevent", "66")
			if err != nil {
				return "", fmt.Errorf("failed to send Enter key: %w", err)
			}
			continue
		}

		// Chunk and escape text for reliable input
		chunks := chunkText(line, 80)
		for _, chunk := range chunks {
			escaped := escapeAdbText(chunk)
			if escaped == "" {
				continue
			}
			_, err := t.helper.execAdb(ctx, device, 10*time.Second,
				"shell", "input", "text", escaped)
			if err != nil {
				return "", fmt.Errorf("failed to input text chunk: %w", err)
			}
		}

		// Send Enter between lines (but not after the last line unless press_enter)
		if i < len(lines)-1 {
			_, err := t.helper.execAdb(ctx, device, 8*time.Second,
				"shell", "input", "keyevent", "66")
			if err != nil {
				return "", fmt.Errorf("failed to send Enter key: %w", err)
			}
		}
	}

	// Optionally press Enter after all text
	if pressEnter {
		_, err := t.helper.execAdb(ctx, device, 8*time.Second,
			"shell", "input", "keyevent", "66")
		if err != nil {
			return "", fmt.Errorf("failed to send Enter key: %w", err)
		}
	}

	return fmt.Sprintf("Input text: %s", text), nil
}

// ==================== ADB Screenshot Tool ====================

type AdbScreenshotTool struct {
	helper *AdbHelper
}

func NewAdbScreenshotTool(helper *AdbHelper) *AdbScreenshotTool {
	return &AdbScreenshotTool{helper: helper}
}

func (t *AdbScreenshotTool) Name() string {
	return "adb_screenshot"
}

func (t *AdbScreenshotTool) Description() string {
	return "Capture a screenshot from the Android device. Uses exec-out for direct PNG capture. Can save to file or return as base64."
}

func (t *AdbScreenshotTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"filename": map[string]interface{}{
				"type":        "string",
				"description": "Filename for the screenshot (e.g., 'screenshot.png'). If omitted, returns base64-encoded PNG.",
			},
			"device": map[string]interface{}{
				"type":        "string",
				"description": "Device serial number (optional)",
			},
		},
	}
}

func (t *AdbScreenshotTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	device, _ := args["device"].(string)

	// Use exec-out for direct binary PNG capture (no temp file on device)
	data, err := t.helper.execAdbBinary(ctx, device, 15*time.Second,
		"exec-out", "screencap", "-p")
	if err != nil {
		return "", fmt.Errorf("failed to capture screenshot: %w", err)
	}

	// Validate PNG signature
	if len(data) < 8 || !bytes.Equal(data[:8], pngSignature) {
		return "", fmt.Errorf("device screencap did not return valid PNG data (got %d bytes)", len(data))
	}

	filename, _ := args["filename"].(string)
	if filename == "" {
		// Return base64-encoded PNG
		encoded := base64.StdEncoding.EncodeToString(data)
		result := map[string]interface{}{
			"format":   "png",
			"size":     len(data),
			"data_b64": encoded,
		}
		out, _ := json.Marshal(result)
		return string(out), nil
	}

	// Save to file
	localPath := t.helper.resolvePath(filename)
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(localPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write screenshot: %w", err)
	}

	return fmt.Sprintf("Screenshot saved to: %s (%d bytes)", localPath, len(data)), nil
}

// ==================== ADB UI Dump Tool ====================

type AdbUIDumpTool struct {
	helper *AdbHelper
}

func NewAdbUIDumpTool(helper *AdbHelper) *AdbUIDumpTool {
	return &AdbUIDumpTool{helper: helper}
}

func (t *AdbUIDumpTool) Name() string {
	return "adb_ui_dump"
}

func (t *AdbUIDumpTool) Description() string {
	return "Get the UI hierarchy (accessibility tree) of the current screen on the Android device. Returns XML structure with UI element information including bounds, text, and resource IDs."
}

func (t *AdbUIDumpTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"device": map[string]interface{}{
				"type":        "string",
				"description": "Device serial number (optional)",
			},
		},
	}
}

func (t *AdbUIDumpTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	device, _ := args["device"].(string)

	// Try multiple dump paths - /sdcard/ is not always writable on some devices
	dumpPaths := []string{
		"/sdcard/window_dump.xml",
		"/data/local/tmp/window_dump.xml",
	}

	var output string

	for _, dumpPath := range dumpPaths {
		// Try dump (ignore command output - it varies across Android versions/devices)
		// Some output to stdout, some to stderr, some output nothing
		t.helper.execAdb(ctx, device, 15*time.Second,
			"shell", "uiautomator", "dump", dumpPath)

		// Small delay to ensure file is fully written
		time.Sleep(200 * time.Millisecond)

		// Try to read the dumped file - this is the real success check
		content, err := t.helper.execAdb(ctx, device, 12*time.Second,
			"exec-out", "cat", dumpPath)
		if err != nil || len(strings.TrimSpace(content)) == 0 {
			// Fallback to shell cat
			content, err = t.helper.execAdb(ctx, device, 12*time.Second,
				"shell", "cat", dumpPath)
		}

		// Clean up (best effort)
		t.helper.execAdb(ctx, device, 5*time.Second, "shell", "rm", dumpPath)

		if err != nil || len(strings.TrimSpace(content)) == 0 {
			continue
		}

		// Strip content before XML declaration
		if idx := strings.Index(content, "<?xml"); idx > 0 {
			content = content[idx:]
		}

		// Check if we got valid XML hierarchy
		if strings.Contains(content, "<hierarchy") || strings.Contains(content, "<node") {
			output = strings.TrimSpace(content)
			break
		}
	}

	// If all paths failed, try one more time with default path (no explicit path arg)
	if output == "" {
		t.helper.execAdb(ctx, device, 15*time.Second,
			"shell", "uiautomator", "dump")
		time.Sleep(200 * time.Millisecond)

		// uiautomator dump without path defaults to /sdcard/window_dump.xml
		content, err := t.helper.execAdb(ctx, device, 12*time.Second,
			"shell", "cat", "/sdcard/window_dump.xml")
		if err == nil {
			if idx := strings.Index(content, "<?xml"); idx > 0 {
				content = content[idx:]
			}
			if strings.Contains(content, "<hierarchy") || strings.Contains(content, "<node") {
				output = strings.TrimSpace(content)
			}
		}
		t.helper.execAdb(ctx, device, 5*time.Second, "shell", "rm", "/sdcard/window_dump.xml")
	}

	if output == "" {
		return "", fmt.Errorf("failed to dump UI hierarchy: uiautomator dump returned no valid XML. Device screen may be locked or accessibility service unavailable")
	}

	// Truncate if too long
	maxLen := 20000
	if len(output) > maxLen {
		output = output[:maxLen] + fmt.Sprintf("\n... (truncated, %d more chars)", len(output)-maxLen)
	}

	return output, nil
}

// ==================== ADB Swipe Tool ====================

type AdbSwipeTool struct {
	helper *AdbHelper
}

func NewAdbSwipeTool(helper *AdbHelper) *AdbSwipeTool {
	return &AdbSwipeTool{helper: helper}
}

func (t *AdbSwipeTool) Name() string {
	return "adb_swipe"
}

func (t *AdbSwipeTool) Description() string {
	return "Simulate a swipe gesture on the Android device. Can use explicit coordinates (x1,y1 to x2,y2) or direction-based swipe from a starting point. Useful for scrolling, swiping between screens, or dragging."
}

func (t *AdbSwipeTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"x": map[string]interface{}{
				"type":        "number",
				"description": "Starting X coordinate (used with direction, or as x1 for coordinate-based swipe)",
			},
			"y": map[string]interface{}{
				"type":        "number",
				"description": "Starting Y coordinate (used with direction, or as y1 for coordinate-based swipe)",
			},
			"x2": map[string]interface{}{
				"type":        "number",
				"description": "Ending X coordinate (for coordinate-based swipe, ignored if direction is set)",
			},
			"y2": map[string]interface{}{
				"type":        "number",
				"description": "Ending Y coordinate (for coordinate-based swipe, ignored if direction is set)",
			},
			"direction": map[string]interface{}{
				"type":        "string",
				"description": "Swipe direction: 'up', 'down', 'left', 'right'. When set, calculates end coordinates automatically from start point.",
				"enum":        []string{"up", "down", "left", "right"},
			},
			"distance": map[string]interface{}{
				"type":        "number",
				"description": "Swipe distance in pixels when using direction (default: 500)",
			},
			"duration": map[string]interface{}{
				"type":        "number",
				"description": "Swipe duration in milliseconds (default: 220)",
			},
			"device": map[string]interface{}{
				"type":        "string",
				"description": "Device serial number (optional)",
			},
		},
		"required": []string{"x", "y"},
	}
}

func (t *AdbSwipeTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	x, ok := args["x"].(float64)
	if !ok {
		return "", fmt.Errorf("x coordinate is required")
	}

	y, ok := args["y"].(float64)
	if !ok {
		return "", fmt.Errorf("y coordinate is required")
	}

	device, _ := args["device"].(string)

	duration := 220.0
	if d, ok := args["duration"].(float64); ok {
		duration = d
	}

	var endX, endY float64
	direction, hasDirection := args["direction"].(string)

	if hasDirection && direction != "" {
		// Direction-based swipe
		dist := 500.0
		if d, ok := args["distance"].(float64); ok {
			dist = d
		}

		endX, endY = x, y
		switch direction {
		case "up":
			endY = y - dist
			if endY < 0 {
				endY = 0
			}
		case "down":
			endY = y + dist
		case "left":
			endX = x - dist
			if endX < 0 {
				endX = 0
			}
		case "right":
			endX = x + dist
		default:
			return "", fmt.Errorf("invalid direction: %s (use up, down, left, right)", direction)
		}
	} else {
		// Coordinate-based swipe (backward compatible)
		var ok2 bool
		endX, ok2 = args["x2"].(float64)
		if !ok2 {
			return "", fmt.Errorf("x2 is required when direction is not set")
		}
		endY, ok2 = args["y2"].(float64)
		if !ok2 {
			return "", fmt.Errorf("y2 is required when direction is not set")
		}
	}

	_, err := t.helper.execAdb(ctx, device, 10*time.Second,
		"shell", "input", "swipe",
		fmt.Sprintf("%d", int(x)),
		fmt.Sprintf("%d", int(y)),
		fmt.Sprintf("%d", int(endX)),
		fmt.Sprintf("%d", int(endY)),
		fmt.Sprintf("%d", int(duration)))

	if err != nil {
		return "", err
	}

	if hasDirection && direction != "" {
		return fmt.Sprintf("Swiped %s from (%d, %d) to (%d, %d) in %dms",
			direction, int(x), int(y), int(endX), int(endY), int(duration)), nil
	}
	return fmt.Sprintf("Swiped from (%d, %d) to (%d, %d) in %dms",
		int(x), int(y), int(endX), int(endY), int(duration)), nil
}

// ==================== ADB Open App Tool ====================

type AdbOpenAppTool struct {
	helper *AdbHelper
}

func NewAdbOpenAppTool(helper *AdbHelper) *AdbOpenAppTool {
	return &AdbOpenAppTool{helper: helper}
}

func (t *AdbOpenAppTool) Name() string {
	return "adb_open_app"
}

func (t *AdbOpenAppTool) Description() string {
	return "Launch an application on the Android device by package name. Uses am start with fallback to monkey launcher. Common packages: com.android.settings, com.android.chrome, com.whatsapp, com.google.android.apps.photos."
}

func (t *AdbOpenAppTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"package": map[string]interface{}{
				"type":        "string",
				"description": "Android package name (e.g., 'com.android.settings')",
			},
			"device": map[string]interface{}{
				"type":        "string",
				"description": "Device serial number (optional)",
			},
		},
		"required": []string{"package"},
	}
}

func (t *AdbOpenAppTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pkg, ok := args["package"].(string)
	if !ok {
		return "", fmt.Errorf("package name is required")
	}

	device, _ := args["device"].(string)

	// Try am start with launcher category first
	output, err := t.helper.execAdb(ctx, device, 10*time.Second,
		"shell", "am", "start", "-a", "android.intent.action.MAIN",
		"-c", "android.intent.category.LAUNCHER", pkg)

	if err != nil || strings.Contains(output, "Error") {
		// Fallback: use monkey to launch the app
		output, err = t.helper.execAdb(ctx, device, 10*time.Second,
			"shell", "monkey", "-p", pkg,
			"-c", "android.intent.category.LAUNCHER", "1")
		if err != nil {
			return "", fmt.Errorf("failed to launch app %s: %w", pkg, err)
		}
	}

	return fmt.Sprintf("Launched app: %s\n%s", pkg, strings.TrimSpace(output)), nil
}

// ==================== ADB Key Event Tool ====================

type AdbKeyEventTool struct {
	helper *AdbHelper
}

func NewAdbKeyEventTool(helper *AdbHelper) *AdbKeyEventTool {
	return &AdbKeyEventTool{helper: helper}
}

func (t *AdbKeyEventTool) Name() string {
	return "adb_keyevent"
}

func (t *AdbKeyEventTool) Description() string {
	return "Send a key event to the Android device. Common keycodes: 3=Home, 4=Back, 24=Volume Up, 25=Volume Down, 26=Power, 66=Enter, 67=Backspace, 82=Menu, 187=Recent Apps."
}

func (t *AdbKeyEventTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"keycode": map[string]interface{}{
				"type":        "number",
				"description": "Android keycode number (e.g., 3 for Home, 4 for Back)",
			},
			"device": map[string]interface{}{
				"type":        "string",
				"description": "Device serial number (optional)",
			},
		},
		"required": []string{"keycode"},
	}
}

func (t *AdbKeyEventTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	keycode, ok := args["keycode"].(float64)
	if !ok {
		return "", fmt.Errorf("keycode is required")
	}

	device, _ := args["device"].(string)

	keycodeStr := fmt.Sprintf("%d", int(keycode))
	_, err := t.helper.execAdb(ctx, device, 8*time.Second,
		"shell", "input", "keyevent", keycodeStr)
	if err != nil {
		return "", err
	}

	// Map common keycodes to names for better output
	names := map[int]string{
		3: "HOME", 4: "BACK", 24: "VOLUME_UP", 25: "VOLUME_DOWN",
		26: "POWER", 66: "ENTER", 67: "BACKSPACE", 82: "MENU",
		187: "RECENT_APPS",
	}

	name := keycodeStr
	if n, ok := names[int(keycode)]; ok {
		name = n
	}

	return fmt.Sprintf("Sent keyevent: %s (%s)", name, keycodeStr), nil
}
