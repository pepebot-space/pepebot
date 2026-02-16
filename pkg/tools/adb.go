package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

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

// execAdb executes an ADB command with optional device serial
func (h *AdbHelper) execAdb(ctx context.Context, device string, args ...string) (string, error) {
	cmdArgs := []string{}
	if device != "" {
		cmdArgs = append(cmdArgs, "-s", device)
	}
	cmdArgs = append(cmdArgs, args...)

	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, h.adbPath, cmdArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\nSTDERR:\n" + stderr.String()
	}

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command timed out after 30s")
		}
		return "", fmt.Errorf("adb command failed: %w\nOutput: %s", err, output)
	}

	return output, nil
}

// resolvePath resolves relative paths to workspace directory
func (h *AdbHelper) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(h.workspace, path)
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
	output, err := t.helper.execAdb(ctx, "", "devices")
	if err != nil {
		return "", err
	}

	// Parse output into structured JSON
	lines := strings.Split(strings.TrimSpace(output), "\n")
	devices := []map[string]string{}

	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // Skip header and empty lines
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			devices = append(devices, map[string]string{
				"serial": parts[0],
				"status": parts[1],
			})
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

	output, err := t.helper.execAdb(ctx, device, "shell", command)
	if err != nil {
		return "", err
	}

	// Truncate long output
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
	return "Simulate a tap gesture at specific screen coordinates on the Android device. Use this to interact with UI elements when you know their position."
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

	_, err := t.helper.execAdb(ctx, device, "shell", "input", "tap", fmt.Sprintf("%d", int(x)), fmt.Sprintf("%d", int(y)))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Tapped at coordinates (%d, %d)", int(x), int(y)), nil
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
	return "Input text into the currently focused field on the Android device. Make sure a text field is focused (via tap) before using this."
}

func (t *AdbInputTextTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Text to input",
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

	// Escape spaces for ADB input command
	escapedText := strings.ReplaceAll(text, " ", "%s")

	_, err := t.helper.execAdb(ctx, device, "shell", "input", "text", escapedText)
	if err != nil {
		return "", err
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
	return "Capture a screenshot from the Android device and save it to a file. The screenshot is saved to the workspace directory."
}

func (t *AdbScreenshotTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"filename": map[string]interface{}{
				"type":        "string",
				"description": "Filename for the screenshot (e.g., 'screenshot.png')",
			},
			"device": map[string]interface{}{
				"type":        "string",
				"description": "Device serial number (optional)",
			},
		},
		"required": []string{"filename"},
	}
}

func (t *AdbScreenshotTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	filename, ok := args["filename"].(string)
	if !ok {
		return "", fmt.Errorf("filename is required")
	}

	device, _ := args["device"].(string)

	// Resolve path relative to workspace
	localPath := t.helper.resolvePath(filename)

	// Ensure directory exists
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Capture screenshot on device
	devicePath := "/sdcard/screenshot_temp.png"
	_, err := t.helper.execAdb(ctx, device, "shell", "screencap", "-p", devicePath)
	if err != nil {
		return "", fmt.Errorf("failed to capture screenshot: %w", err)
	}

	// Pull screenshot to local
	_, err = t.helper.execAdb(ctx, device, "pull", devicePath, localPath)
	if err != nil {
		return "", fmt.Errorf("failed to pull screenshot: %w", err)
	}

	// Clean up device screenshot
	t.helper.execAdb(ctx, device, "shell", "rm", devicePath)

	return fmt.Sprintf("Screenshot saved to: %s", localPath), nil
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

	// Dump UI hierarchy to device
	devicePath := "/sdcard/ui_dump.xml"
	_, err := t.helper.execAdb(ctx, device, "shell", "uiautomator", "dump", devicePath)
	if err != nil {
		return "", fmt.Errorf("failed to dump UI: %w", err)
	}

	// Read the XML file
	output, err := t.helper.execAdb(ctx, device, "shell", "cat", devicePath)
	if err != nil {
		return "", fmt.Errorf("failed to read UI dump: %w", err)
	}

	// Clean up
	t.helper.execAdb(ctx, device, "shell", "rm", devicePath)

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
	return "Simulate a swipe gesture on the Android device from one coordinate to another. Useful for scrolling, swiping between screens, or dragging elements."
}

func (t *AdbSwipeTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"x1": map[string]interface{}{
				"type":        "number",
				"description": "Starting X coordinate",
			},
			"y1": map[string]interface{}{
				"type":        "number",
				"description": "Starting Y coordinate",
			},
			"x2": map[string]interface{}{
				"type":        "number",
				"description": "Ending X coordinate",
			},
			"y2": map[string]interface{}{
				"type":        "number",
				"description": "Ending Y coordinate",
			},
			"duration": map[string]interface{}{
				"type":        "number",
				"description": "Swipe duration in milliseconds (default: 300)",
			},
			"device": map[string]interface{}{
				"type":        "string",
				"description": "Device serial number (optional)",
			},
		},
		"required": []string{"x1", "y1", "x2", "y2"},
	}
}

func (t *AdbSwipeTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	x1, ok := args["x1"].(float64)
	if !ok {
		return "", fmt.Errorf("x1 coordinate is required")
	}

	y1, ok := args["y1"].(float64)
	if !ok {
		return "", fmt.Errorf("y1 coordinate is required")
	}

	x2, ok := args["x2"].(float64)
	if !ok {
		return "", fmt.Errorf("x2 coordinate is required")
	}

	y2, ok := args["y2"].(float64)
	if !ok {
		return "", fmt.Errorf("y2 coordinate is required")
	}

	duration := 300.0
	if d, ok := args["duration"].(float64); ok {
		duration = d
	}

	device, _ := args["device"].(string)

	_, err := t.helper.execAdb(ctx, device, "shell", "input", "swipe",
		fmt.Sprintf("%d", int(x1)),
		fmt.Sprintf("%d", int(y1)),
		fmt.Sprintf("%d", int(x2)),
		fmt.Sprintf("%d", int(y2)),
		fmt.Sprintf("%d", int(duration)))

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Swiped from (%d, %d) to (%d, %d) in %dms", int(x1), int(y1), int(x2), int(y2), int(duration)), nil
}
