package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ==================== Types ====================

// InputDeviceInfo holds information about a touch input device
type InputDeviceInfo struct {
	DevicePath string
	RawMaxX    int
	RawMaxY    int
}

// ScreenResolution holds the pixel dimensions of the screen
type ScreenResolution struct {
	Width  int
	Height int
}

// TouchPoint represents a single raw touch coordinate with timestamp
type TouchPoint struct {
	RawX      int
	RawY      int
	Timestamp time.Time
}

// TouchGesture represents a complete touch from BTN_TOUCH DOWN to UP
type TouchGesture struct {
	Points []TouchPoint
	Start  time.Time
	End    time.Time
}

// RecordedAction represents a classified user action
type RecordedAction struct {
	Type      string // "tap" or "swipe"
	X         int    // pixel X (for tap: average position; for swipe: start)
	Y         int    // pixel Y
	X2        int    // pixel X end (swipe only)
	Y2        int    // pixel Y end (swipe only)
	Duration  int    // milliseconds (swipe only)
	Timestamp time.Time
}

// RecorderConfig holds tunable thresholds for gesture classification
type RecorderConfig struct {
	TapMaxDistance  float64       // max pixel distance to classify as tap (default: 30)
	TapMaxDuration time.Duration // max duration to classify as tap (default: 300ms)
	DebounceWindow time.Duration // min time between recorded actions (default: 200ms)
	SwipeMinDist   float64       // min pixel distance for swipe (default: 50)
}

// DefaultRecorderConfig returns sensible defaults
func DefaultRecorderConfig() RecorderConfig {
	return RecorderConfig{
		TapMaxDistance:  30,
		TapMaxDuration: 300 * time.Millisecond,
		DebounceWindow: 200 * time.Millisecond,
		SwipeMinDist:   50,
	}
}

// eventParserState tracks the state machine for parsing getevent output
type eventParserState int

const (
	stateIdle     eventParserState = iota
	stateTouching                  // actively receiving touch data
)

// eventParser processes getevent lines and produces gestures
type eventParser struct {
	state      eventParserState
	currentX   int
	currentY   int
	hasX       bool
	hasY       bool
	points     []TouchPoint
	touchStart time.Time
}

// ==================== Event Parsing ====================

// parsedEvent represents a single parsed getevent line
type parsedEvent struct {
	Device string
	Type   string
	Code   string
	Value  string
}

// getevent -l output format: /dev/input/eventN: EV_TYPE CODE VALUE
var geteventLineRegex = regexp.MustCompile(`^(/dev/input/event\d+):\s+(\w+)\s+(\w+)\s+(\w+)$`)

// parseEventLine parses a single line of `adb shell getevent -l` output
func parseEventLine(line string) (*parsedEvent, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}

	matches := geteventLineRegex.FindStringSubmatch(line)
	if matches == nil {
		return nil, fmt.Errorf("unrecognized format: %s", line)
	}

	return &parsedEvent{
		Device: matches[1],
		Type:   matches[2],
		Code:   matches[3],
		Value:  matches[4],
	}, nil
}

// hexToInt converts a hex string (with or without 0x prefix) to int
func hexToInt(s string) (int, error) {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	val, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		return 0, err
	}
	return int(val), nil
}

// ==================== Coordinate Mapping ====================

// mapCoordinate converts a raw touch coordinate to pixel coordinate
func mapCoordinate(raw, rawMax, screenSize int) int {
	if rawMax <= 0 {
		return raw
	}
	return raw * screenSize / rawMax
}

// ==================== Gesture Classification ====================

// classifyGesture classifies a TouchGesture as a RecordedAction
func classifyGesture(gesture TouchGesture, device InputDeviceInfo, screen ScreenResolution, cfg RecorderConfig) *RecordedAction {
	if len(gesture.Points) == 0 {
		return nil
	}

	duration := gesture.End.Sub(gesture.Start)

	// Map all points to pixel coordinates
	first := gesture.Points[0]
	last := gesture.Points[len(gesture.Points)-1]

	firstPixelX := mapCoordinate(first.RawX, device.RawMaxX, screen.Width)
	firstPixelY := mapCoordinate(first.RawY, device.RawMaxY, screen.Height)
	lastPixelX := mapCoordinate(last.RawX, device.RawMaxX, screen.Width)
	lastPixelY := mapCoordinate(last.RawY, device.RawMaxY, screen.Height)

	dx := float64(lastPixelX - firstPixelX)
	dy := float64(lastPixelY - firstPixelY)
	dist := math.Sqrt(dx*dx + dy*dy)

	// Swipe: distance >= swipe min distance
	if dist >= cfg.SwipeMinDist {
		return &RecordedAction{
			Type:      "swipe",
			X:         firstPixelX,
			Y:         firstPixelY,
			X2:        lastPixelX,
			Y2:        lastPixelY,
			Duration:  int(duration.Milliseconds()),
			Timestamp: gesture.Start,
		}
	}

	// Tap: small distance AND short duration
	if dist < cfg.TapMaxDistance && duration < cfg.TapMaxDuration {
		// Use average position for tap
		avgX, avgY := 0, 0
		for _, p := range gesture.Points {
			avgX += mapCoordinate(p.RawX, device.RawMaxX, screen.Width)
			avgY += mapCoordinate(p.RawY, device.RawMaxY, screen.Height)
		}
		avgX /= len(gesture.Points)
		avgY /= len(gesture.Points)

		return &RecordedAction{
			Type:      "tap",
			X:         avgX,
			Y:         avgY,
			Timestamp: gesture.Start,
		}
	}

	// Ambiguous: treat as tap at average position
	avgX, avgY := 0, 0
	for _, p := range gesture.Points {
		avgX += mapCoordinate(p.RawX, device.RawMaxX, screen.Width)
		avgY += mapCoordinate(p.RawY, device.RawMaxY, screen.Height)
	}
	avgX /= len(gesture.Points)
	avgY /= len(gesture.Points)

	return &RecordedAction{
		Type:      "tap",
		X:         avgX,
		Y:         avgY,
		Timestamp: gesture.Start,
	}
}

// shouldDebounce returns true if the action should be discarded due to debounce window
func shouldDebounce(action *RecordedAction, lastAction *RecordedAction, window time.Duration) bool {
	if lastAction == nil {
		return false
	}
	return action.Timestamp.Sub(lastAction.Timestamp) < window
}

// ==================== Device Discovery ====================

// parseScreenResolution parses "Physical size: WxH" from `adb shell wm size`
func parseScreenResolution(output string) (ScreenResolution, error) {
	// Handle "Physical size: 1080x1920" or "Override size: ..."
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Physical size:") || strings.HasPrefix(line, "Override size:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			dims := strings.TrimSpace(parts[1])
			wh := strings.SplitN(dims, "x", 2)
			if len(wh) != 2 {
				continue
			}
			w, err := strconv.Atoi(strings.TrimSpace(wh[0]))
			if err != nil {
				continue
			}
			h, err := strconv.Atoi(strings.TrimSpace(wh[1]))
			if err != nil {
				continue
			}
			return ScreenResolution{Width: w, Height: h}, nil
		}
	}
	return ScreenResolution{}, fmt.Errorf("could not parse screen resolution from: %s", output)
}

// parseInputDeviceInfo parses `adb shell getevent -p` output to find touch device
// Looks for a device with ABS_MT_POSITION_X (0035) and extracts max values
func parseInputDeviceInfo(output string) (InputDeviceInfo, error) {
	lines := strings.Split(output, "\n")

	var currentDevice string
	var foundDevice string
	var maxX, maxY int
	inAbsSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Device header: "add device N: /dev/input/eventN"
		if strings.HasPrefix(trimmed, "add device") {
			parts := strings.Fields(trimmed)
			for _, p := range parts {
				if strings.HasPrefix(p, "/dev/input/event") {
					currentDevice = p
					inAbsSection = false
					break
				}
			}
			continue
		}

		// Check for ABS section
		if strings.Contains(trimmed, "ABS") && strings.Contains(trimmed, "(0003)") {
			inAbsSection = true
			continue
		}

		// Non-ABS section header resets
		if len(trimmed) > 0 && !strings.HasPrefix(trimmed, "0035") && !strings.HasPrefix(trimmed, "0036") &&
			!strings.HasPrefix(trimmed, "value") && !strings.HasPrefix(trimmed, "min") &&
			!strings.HasPrefix(trimmed, "max") && !strings.HasPrefix(trimmed, "fuzz") &&
			!strings.HasPrefix(trimmed, "flat") && !strings.HasPrefix(trimmed, "resolution") &&
			strings.Contains(trimmed, "(") && !inAbsSection {
			inAbsSection = false
		}

		if !inAbsSection || currentDevice == "" {
			continue
		}

		// ABS_MT_POSITION_X (0035)
		if strings.Contains(trimmed, "0035") || strings.Contains(trimmed, "ABS_MT_POSITION_X") {
			foundDevice = currentDevice
			// Parse max value from this or following lines
			if mx := extractMaxValue(trimmed); mx > 0 {
				maxX = mx
			}
			continue
		}

		// ABS_MT_POSITION_Y (0036)
		if strings.Contains(trimmed, "0036") || strings.Contains(trimmed, "ABS_MT_POSITION_Y") {
			if mx := extractMaxValue(trimmed); mx > 0 {
				maxY = mx
			}
			continue
		}

		// Look for "max" line in device info
		if foundDevice == currentDevice && strings.Contains(trimmed, "max") {
			if maxX == 0 {
				if mx := extractMaxFromLine(trimmed); mx > 0 {
					maxX = mx
				}
			} else if maxY == 0 {
				if mx := extractMaxFromLine(trimmed); mx > 0 {
					maxY = mx
				}
			}
		}
	}

	if foundDevice == "" {
		return InputDeviceInfo{}, fmt.Errorf("no touch input device found with ABS_MT_POSITION_X")
	}

	return InputDeviceInfo{
		DevicePath: foundDevice,
		RawMaxX:    maxX,
		RawMaxY:    maxY,
	}, nil
}

// extractMaxValue extracts the max value from a getevent -p info line
// Format: "0035  : value 0, min 0, max 1079, fuzz 0, flat 0, resolution 0"
func extractMaxValue(line string) int {
	idx := strings.Index(line, "max")
	if idx < 0 {
		return 0
	}
	rest := line[idx+3:]
	// Skip whitespace and possible colon/comma
	rest = strings.TrimLeft(rest, " :,")
	// Read number
	numStr := ""
	for _, c := range rest {
		if c >= '0' && c <= '9' {
			numStr += string(c)
		} else if len(numStr) > 0 {
			break
		}
	}
	if numStr == "" {
		return 0
	}
	val, err := strconv.Atoi(numStr)
	if err != nil {
		return 0
	}
	return val
}

// extractMaxFromLine extracts max value from a standalone "max N" line
func extractMaxFromLine(line string) int {
	return extractMaxValue(line)
}

// discoverInputDevice gets screen resolution and finds the touch input device
func discoverInputDevice(ctx context.Context, helper *AdbHelper, device string) (InputDeviceInfo, ScreenResolution, error) {
	// Get screen resolution
	wmOutput, err := helper.execAdb(ctx, device, 10*time.Second, "shell", "wm", "size")
	if err != nil {
		return InputDeviceInfo{}, ScreenResolution{}, fmt.Errorf("failed to get screen size: %w", err)
	}

	screen, err := parseScreenResolution(wmOutput)
	if err != nil {
		return InputDeviceInfo{}, ScreenResolution{}, err
	}

	// Get input device info
	gevOutput, err := helper.execAdb(ctx, device, 10*time.Second, "shell", "getevent", "-p")
	if err != nil {
		return InputDeviceInfo{}, ScreenResolution{}, fmt.Errorf("failed to get input device info: %w", err)
	}

	inputDev, err := parseInputDeviceInfo(gevOutput)
	if err != nil {
		return InputDeviceInfo{}, ScreenResolution{}, err
	}

	return inputDev, screen, nil
}

// ==================== Event Stream Processing ====================

// processEventStream reads getevent -l output and produces RecordedActions
// It stops when KEY_VOLUMEDOWN is detected or context is cancelled
func processEventStream(
	scanner *bufio.Scanner,
	inputDevice InputDeviceInfo,
	screen ScreenResolution,
	cfg RecorderConfig,
	targetDevice string, // filter events to this device path
) ([]RecordedAction, bool) {
	parser := &eventParser{state: stateIdle}
	var actions []RecordedAction
	var lastAction *RecordedAction
	stopped := false

	for scanner.Scan() {
		line := scanner.Text()

		event, err := parseEventLine(line)
		if err != nil {
			continue
		}

		// Filter to target device if specified
		if targetDevice != "" && event.Device != targetDevice {
			// Check for volume down on any device
			if event.Type == "EV_KEY" && event.Code == "KEY_VOLUMEDOWN" && event.Value == "DOWN" {
				stopped = true
				break
			}
			continue
		}

		// Check for stop signal (Volume Down)
		if event.Type == "EV_KEY" && event.Code == "KEY_VOLUMEDOWN" && event.Value == "DOWN" {
			stopped = true
			break
		}

		now := time.Now()

		switch event.Type {
		case "EV_KEY":
			if event.Code == "BTN_TOUCH" {
				if event.Value == "DOWN" {
					parser.state = stateTouching
					parser.points = nil
					parser.hasX = false
					parser.hasY = false
					parser.touchStart = now
				} else if event.Value == "UP" && parser.state == stateTouching {
					// End of touch - classify gesture
					gesture := TouchGesture{
						Points: parser.points,
						Start:  parser.touchStart,
						End:    now,
					}
					action := classifyGesture(gesture, inputDevice, screen, cfg)
					if action != nil && !shouldDebounce(action, lastAction, cfg.DebounceWindow) {
						actions = append(actions, *action)
						lastAction = action
					}
					parser.state = stateIdle
				}
			}

		case "EV_ABS":
			if parser.state != stateTouching {
				continue
			}

			val, err := hexToInt(event.Value)
			if err != nil {
				continue
			}

			if event.Code == "ABS_MT_POSITION_X" {
				parser.currentX = val
				parser.hasX = true
			} else if event.Code == "ABS_MT_POSITION_Y" {
				parser.currentY = val
				parser.hasY = true
			}

		case "EV_SYN":
			if event.Code == "SYN_REPORT" && parser.state == stateTouching && parser.hasX && parser.hasY {
				parser.points = append(parser.points, TouchPoint{
					RawX:      parser.currentX,
					RawY:      parser.currentY,
					Timestamp: now,
				})
			}
		}
	}

	return actions, stopped
}

// ==================== Workflow Building ====================

// buildWorkflowFromActions creates a WorkflowDefinition from recorded actions
func buildWorkflowFromActions(name, description string, actions []RecordedAction, goalText string) *WorkflowDefinition {
	if description == "" {
		description = "Recorded user actions from Android device"
	}

	steps := make([]WorkflowStep, 0, len(actions)+1)

	for i, action := range actions {
		stepName := fmt.Sprintf("action_%d_%s", i+1, action.Type)

		switch action.Type {
		case "tap":
			steps = append(steps, WorkflowStep{
				Name: stepName,
				Tool: "adb_tap",
				Args: map[string]interface{}{
					"x":      action.X,
					"y":      action.Y,
					"device": "{{device}}",
				},
			})
		case "swipe":
			steps = append(steps, WorkflowStep{
				Name: stepName,
				Tool: "adb_swipe",
				Args: map[string]interface{}{
					"x":        action.X,
					"y":        action.Y,
					"x2":       action.X2,
					"y2":       action.Y2,
					"duration": action.Duration,
					"device":   "{{device}}",
				},
			})
		}
	}

	// Add verification goal step at the end
	if goalText != "" {
		steps = append(steps, WorkflowStep{
			Name: "verify_final_state",
			Goal: goalText,
		})
	}

	return &WorkflowDefinition{
		Name:        name,
		Description: description,
		Variables:   map[string]string{"device": ""},
		Steps:       steps,
	}
}

// ==================== ADB Record Workflow Tool ====================

type AdbRecordWorkflowTool struct {
	helper         *AdbHelper
	workflowHelper *WorkflowHelper
}

func NewAdbRecordWorkflowTool(helper *AdbHelper, workflowHelper *WorkflowHelper) *AdbRecordWorkflowTool {
	return &AdbRecordWorkflowTool{helper: helper, workflowHelper: workflowHelper}
}

func (t *AdbRecordWorkflowTool) Name() string {
	return "adb_record_workflow"
}

func (t *AdbRecordWorkflowTool) Description() string {
	return "Record user interactions on an Android device (taps, swipes) and auto-generate a workflow JSON file. " +
		"IMPORTANT: Only use this tool when the user EXPLICITLY asks to record or create a workflow from device actions. " +
		"Do NOT proactively use this tool. " +
		"Do NOT use workflow_save for this — workflow_save is for manually writing workflow JSON. " +
		"This tool captures real touch events from the device via ADB getevent. " +
		"IMPORTANT: This tool BLOCKS while recording. You MUST first explain to the user: " +
		"(1) recording will capture their taps and swipes on the device, " +
		"(2) they should press Volume Down to stop recording, " +
		"(3) a workflow file will be auto-generated. " +
		"Get user confirmation BEFORE calling with confirmed=true. " +
		"Without confirmed=true, returns preparation instructions only."
}

func (t *AdbRecordWorkflowTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"workflow_name": map[string]interface{}{
				"type":        "string",
				"description": "Name for the workflow (no .json extension needed)",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Description of what this workflow does (optional)",
			},
			"device": map[string]interface{}{
				"type":        "string",
				"description": "Device serial number (optional, uses default device if not specified)",
			},
			"max_duration": map[string]interface{}{
				"type":        "number",
				"description": "Maximum recording duration in seconds (default: 300)",
			},
			"confirmed": map[string]interface{}{
				"type":        "boolean",
				"description": "Must be true to start recording. First call without confirmed=true returns instructions for the user. Only set to true after user has confirmed they are ready.",
			},
		},
		"required": []string{"workflow_name"},
	}
}

func (t *AdbRecordWorkflowTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	workflowName, ok := args["workflow_name"].(string)
	if !ok || workflowName == "" {
		return "", fmt.Errorf("workflow_name is required")
	}

	if strings.ContainsAny(workflowName, "/\\:*?\"<>|") {
		return "", fmt.Errorf("invalid workflow name: contains special characters")
	}

	// Confirmation gate: first call returns instructions, user must confirm before recording starts
	confirmed, _ := args["confirmed"].(bool)
	if !confirmed {
		return fmt.Sprintf("Ready to record workflow '%s'.\n\n"+
			"INSTRUCTIONS FOR USER:\n"+
			"1. Recording will start as soon as you confirm\n"+
			"2. Interact with your Android device normally (tap, swipe)\n"+
			"3. Press VOLUME DOWN on the device to stop recording\n"+
			"4. A workflow file will be generated from your actions\n\n"+
			"Ask the user to confirm they are ready, then call this tool again with confirmed=true to start recording.", workflowName), nil
	}

	description, _ := args["description"].(string)
	device, _ := args["device"].(string)

	maxDuration := 300.0
	if d, ok := args["max_duration"].(float64); ok && d > 0 {
		maxDuration = d
	}

	// Step 1: Discover input device and screen resolution
	inputDev, screen, err := discoverInputDevice(ctx, t.helper, device)
	if err != nil {
		return "", fmt.Errorf("failed to discover input device: %w", err)
	}

	// Step 2: Start getevent -l via streaming
	recordCtx, cancel := context.WithTimeout(ctx, time.Duration(maxDuration)*time.Second)
	defer cancel()

	cmd, stdout, err := t.helper.execAdbStreaming(recordCtx, device, "shell", "getevent", "-l")
	if err != nil {
		return "", fmt.Errorf("failed to start getevent: %w", err)
	}

	// Step 3: Process event stream
	scanner := bufio.NewScanner(stdout)
	cfg := DefaultRecorderConfig()

	actions, stopped := processEventStream(scanner, inputDev, screen, cfg, inputDev.DevicePath)

	// Step 4: Kill getevent process
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
	cmd.Wait()

	if !stopped && len(actions) == 0 {
		return "", fmt.Errorf("recording ended with no actions captured. Ensure you interact with the device screen and press Volume Down to stop")
	}

	// Step 5: Post-recording - capture screenshot and UI dump
	goalText := ""
	screenshotPath := ""

	// Capture screenshot
	screenshotFilename := fmt.Sprintf("workflows/%s_final.png", workflowName)
	screenshotData, err := t.helper.execAdbBinary(ctx, device, 15*time.Second, "exec-out", "screencap", "-p")
	if err == nil && len(screenshotData) >= 8 && bytes.Equal(screenshotData[:8], pngSignature) {
		localPath := t.helper.resolvePath(screenshotFilename)
		dir := filepath.Dir(localPath)
		os.MkdirAll(dir, 0755)
		if writeErr := os.WriteFile(localPath, screenshotData, 0644); writeErr == nil {
			screenshotPath = localPath
		}
	}

	// Capture UI dump
	uiDumpSummary := ""
	t.helper.execAdb(ctx, device, 15*time.Second, "shell", "uiautomator", "dump", "/sdcard/window_dump.xml")
	time.Sleep(200 * time.Millisecond)
	uiContent, err := t.helper.execAdb(ctx, device, 12*time.Second, "shell", "cat", "/sdcard/window_dump.xml")
	if err == nil && strings.Contains(uiContent, "<hierarchy") {
		// Truncate UI dump for goal text
		if len(uiContent) > 2000 {
			uiContent = uiContent[:2000] + "..."
		}
		uiDumpSummary = uiContent
	}
	t.helper.execAdb(ctx, device, 5*time.Second, "shell", "rm", "/sdcard/window_dump.xml")

	// Build goal text
	goalParts := []string{"Verify the final screen state matches the expected outcome."}
	if uiDumpSummary != "" {
		goalParts = append(goalParts, fmt.Sprintf("Final screen UI elements: %s", uiDumpSummary))
	}
	if screenshotPath != "" {
		goalParts = append(goalParts, fmt.Sprintf("Screenshot saved at: %s", screenshotFilename))
	}
	goalText = strings.Join(goalParts, " ")

	// Step 6: Build and save workflow
	workflow := buildWorkflowFromActions(workflowName, description, actions, goalText)

	if err := t.workflowHelper.saveWorkflow(workflowName, workflow); err != nil {
		return "", fmt.Errorf("failed to save workflow: %w", err)
	}

	savePath := filepath.Join(t.workflowHelper.workflowsDir(), workflowName+".json")

	// Step 7: Return summary
	result := map[string]interface{}{
		"workflow_name":   workflowName,
		"action_count":    len(actions),
		"save_path":       savePath,
		"stopped_by_user": stopped,
	}
	if screenshotPath != "" {
		result["screenshot_path"] = screenshotPath
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	return string(out), nil
}
