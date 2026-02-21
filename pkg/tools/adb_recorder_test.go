package tools

import (
	"bufio"
	"strings"
	"testing"
	"time"
)

func TestParseEventLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    *parsedEvent
		wantErr bool
	}{
		{
			name: "BTN_TOUCH DOWN",
			line: "/dev/input/event2: EV_KEY BTN_TOUCH DOWN",
			want: &parsedEvent{
				Device: "/dev/input/event2",
				Type:   "EV_KEY",
				Code:   "BTN_TOUCH",
				Value:  "DOWN",
			},
		},
		{
			name: "BTN_TOUCH UP",
			line: "/dev/input/event2: EV_KEY BTN_TOUCH UP",
			want: &parsedEvent{
				Device: "/dev/input/event2",
				Type:   "EV_KEY",
				Code:   "BTN_TOUCH",
				Value:  "UP",
			},
		},
		{
			name: "ABS_MT_POSITION_X",
			line: "/dev/input/event2: EV_ABS ABS_MT_POSITION_X 0000021c",
			want: &parsedEvent{
				Device: "/dev/input/event2",
				Type:   "EV_ABS",
				Code:   "ABS_MT_POSITION_X",
				Value:  "0000021c",
			},
		},
		{
			name: "ABS_MT_POSITION_Y",
			line: "/dev/input/event2: EV_ABS ABS_MT_POSITION_Y 000003c0",
			want: &parsedEvent{
				Device: "/dev/input/event2",
				Type:   "EV_ABS",
				Code:   "ABS_MT_POSITION_Y",
				Value:  "000003c0",
			},
		},
		{
			name: "SYN_REPORT",
			line: "/dev/input/event2: EV_SYN SYN_REPORT 00000000",
			want: &parsedEvent{
				Device: "/dev/input/event2",
				Type:   "EV_SYN",
				Code:   "SYN_REPORT",
				Value:  "00000000",
			},
		},
		{
			name: "KEY_VOLUMEDOWN",
			line: "/dev/input/event0: EV_KEY KEY_VOLUMEDOWN DOWN",
			want: &parsedEvent{
				Device: "/dev/input/event0",
				Type:   "EV_KEY",
				Code:   "KEY_VOLUMEDOWN",
				Value:  "DOWN",
			},
		},
		{
			name:    "empty line",
			line:    "",
			wantErr: true,
		},
		{
			name:    "garbage",
			line:    "some random text",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEventLine(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseEventLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Device != tt.want.Device || got.Type != tt.want.Type ||
				got.Code != tt.want.Code || got.Value != tt.want.Value {
				t.Errorf("parseEventLine() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestHexToInt(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"0000021c", 540},
		{"000003c0", 960},
		{"00000000", 0},
		{"00000437", 1079},
		{"0x21c", 540},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := hexToInt(tt.input)
			if err != nil {
				t.Fatalf("hexToInt(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("hexToInt(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestMapCoordinate(t *testing.T) {
	tests := []struct {
		name       string
		raw        int
		rawMax     int
		screenSize int
		want       int
	}{
		{"1:1 mapping", 540, 1080, 1080, 540},
		{"scale up", 540, 1080, 2160, 1080},
		{"zero", 0, 1080, 1080, 0},
		{"max", 1079, 1080, 1080, 1079},
		{"rawMax zero fallback", 500, 0, 1080, 500},
		{"different ratio", 500, 32767, 1080, 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapCoordinate(tt.raw, tt.rawMax, tt.screenSize)
			if got != tt.want {
				t.Errorf("mapCoordinate(%d, %d, %d) = %d, want %d",
					tt.raw, tt.rawMax, tt.screenSize, got, tt.want)
			}
		})
	}
}

func TestClassifyGesture_Tap(t *testing.T) {
	cfg := DefaultRecorderConfig()
	device := InputDeviceInfo{RawMaxX: 1080, RawMaxY: 1920}
	screen := ScreenResolution{Width: 1080, Height: 1920}

	now := time.Now()
	gesture := TouchGesture{
		Points: []TouchPoint{
			{RawX: 540, RawY: 960, Timestamp: now},
			{RawX: 541, RawY: 961, Timestamp: now.Add(50 * time.Millisecond)},
			{RawX: 540, RawY: 960, Timestamp: now.Add(100 * time.Millisecond)},
		},
		Start: now,
		End:   now.Add(100 * time.Millisecond),
	}

	action := classifyGesture(gesture, device, screen, cfg)
	if action == nil {
		t.Fatal("expected non-nil action")
	}
	if action.Type != "tap" {
		t.Errorf("expected tap, got %s", action.Type)
	}
	// Average of 540, 541, 540 = 540 (integer division)
	if action.X != 540 {
		t.Errorf("expected X=540, got %d", action.X)
	}
	if action.Y != 960 {
		t.Errorf("expected Y=960, got %d", action.Y)
	}
}

func TestClassifyGesture_Swipe(t *testing.T) {
	cfg := DefaultRecorderConfig()
	device := InputDeviceInfo{RawMaxX: 1080, RawMaxY: 1920}
	screen := ScreenResolution{Width: 1080, Height: 1920}

	now := time.Now()
	gesture := TouchGesture{
		Points: []TouchPoint{
			{RawX: 200, RawY: 1500, Timestamp: now},
			{RawX: 200, RawY: 1200, Timestamp: now.Add(100 * time.Millisecond)},
			{RawX: 200, RawY: 800, Timestamp: now.Add(300 * time.Millisecond)},
		},
		Start: now,
		End:   now.Add(300 * time.Millisecond),
	}

	action := classifyGesture(gesture, device, screen, cfg)
	if action == nil {
		t.Fatal("expected non-nil action")
	}
	if action.Type != "swipe" {
		t.Errorf("expected swipe, got %s", action.Type)
	}
	if action.X != 200 || action.Y != 1500 {
		t.Errorf("expected start (200, 1500), got (%d, %d)", action.X, action.Y)
	}
	if action.X2 != 200 || action.Y2 != 800 {
		t.Errorf("expected end (200, 800), got (%d, %d)", action.X2, action.Y2)
	}
	if action.Duration != 300 {
		t.Errorf("expected duration 300ms, got %d", action.Duration)
	}
}

func TestClassifyGesture_Empty(t *testing.T) {
	cfg := DefaultRecorderConfig()
	device := InputDeviceInfo{RawMaxX: 1080, RawMaxY: 1920}
	screen := ScreenResolution{Width: 1080, Height: 1920}

	gesture := TouchGesture{Points: nil}
	action := classifyGesture(gesture, device, screen, cfg)
	if action != nil {
		t.Errorf("expected nil action for empty gesture, got %+v", action)
	}
}

func TestDebounce(t *testing.T) {
	window := 200 * time.Millisecond
	now := time.Now()

	lastAction := &RecordedAction{
		Type:      "tap",
		X:         100,
		Y:         200,
		Timestamp: now,
	}

	// Action within debounce window - should be debounced
	tooSoon := &RecordedAction{
		Type:      "tap",
		X:         150,
		Y:         250,
		Timestamp: now.Add(100 * time.Millisecond),
	}
	if !shouldDebounce(tooSoon, lastAction, window) {
		t.Error("expected action within 100ms to be debounced")
	}

	// Action outside debounce window - should not be debounced
	okAction := &RecordedAction{
		Type:      "tap",
		X:         150,
		Y:         250,
		Timestamp: now.Add(250 * time.Millisecond),
	}
	if shouldDebounce(okAction, lastAction, window) {
		t.Error("expected action at 250ms to not be debounced")
	}

	// No previous action - should not be debounced
	if shouldDebounce(tooSoon, nil, window) {
		t.Error("expected first action to not be debounced")
	}
}

func TestEventParser_FullSequence(t *testing.T) {
	// Simulate a complete tap sequence followed by volume down stop
	events := `/dev/input/event2: EV_KEY BTN_TOUCH DOWN
/dev/input/event2: EV_ABS ABS_MT_POSITION_X 0000021c
/dev/input/event2: EV_ABS ABS_MT_POSITION_Y 000003c0
/dev/input/event2: EV_SYN SYN_REPORT 00000000
/dev/input/event2: EV_KEY BTN_TOUCH UP
/dev/input/event0: EV_KEY KEY_VOLUMEDOWN DOWN`

	device := InputDeviceInfo{
		DevicePath: "/dev/input/event2",
		RawMaxX:    1080,
		RawMaxY:    1920,
	}
	screen := ScreenResolution{Width: 1080, Height: 1920}
	cfg := DefaultRecorderConfig()

	scanner := bufio.NewScanner(strings.NewReader(events))
	actions, stopped := processEventStream(scanner, device, screen, cfg, "/dev/input/event2")

	if !stopped {
		t.Error("expected recording to be stopped by volume down")
	}

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	action := actions[0]
	if action.Type != "tap" {
		t.Errorf("expected tap, got %s", action.Type)
	}

	// 0x21c = 540, mapped with rawMax=1080, screen=1080 → 540
	if action.X != 540 {
		t.Errorf("expected X=540, got %d", action.X)
	}
	// 0x3c0 = 960, mapped with rawMax=1920, screen=1920 → 960
	if action.Y != 960 {
		t.Errorf("expected Y=960, got %d", action.Y)
	}
}

func TestEventParser_SwipeSequence(t *testing.T) {
	// Simulate a swipe from (200, 1500) to (200, 800)
	events := `/dev/input/event2: EV_KEY BTN_TOUCH DOWN
/dev/input/event2: EV_ABS ABS_MT_POSITION_X 000000c8
/dev/input/event2: EV_ABS ABS_MT_POSITION_Y 000005dc
/dev/input/event2: EV_SYN SYN_REPORT 00000000
/dev/input/event2: EV_ABS ABS_MT_POSITION_Y 00000320
/dev/input/event2: EV_SYN SYN_REPORT 00000000
/dev/input/event2: EV_KEY BTN_TOUCH UP
/dev/input/event0: EV_KEY KEY_VOLUMEDOWN DOWN`

	device := InputDeviceInfo{
		DevicePath: "/dev/input/event2",
		RawMaxX:    1080,
		RawMaxY:    1920,
	}
	screen := ScreenResolution{Width: 1080, Height: 1920}
	cfg := DefaultRecorderConfig()

	scanner := bufio.NewScanner(strings.NewReader(events))
	actions, stopped := processEventStream(scanner, device, screen, cfg, "/dev/input/event2")

	if !stopped {
		t.Error("expected recording to be stopped by volume down")
	}

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	action := actions[0]
	if action.Type != "swipe" {
		t.Errorf("expected swipe, got %s", action.Type)
	}
	// 0xc8 = 200
	if action.X != 200 {
		t.Errorf("expected start X=200, got %d", action.X)
	}
	// 0x5dc = 1500
	if action.Y != 1500 {
		t.Errorf("expected start Y=1500, got %d", action.Y)
	}
	// 0x320 = 800
	if action.Y2 != 800 {
		t.Errorf("expected end Y=800, got %d", action.Y2)
	}
}

func TestBuildWorkflow(t *testing.T) {
	actions := []RecordedAction{
		{Type: "tap", X: 540, Y: 960},
		{Type: "swipe", X: 200, Y: 1500, X2: 200, Y2: 800, Duration: 400},
	}

	workflow := buildWorkflowFromActions("test_wf", "Test workflow", actions, "Verify final state.")

	if workflow.Name != "test_wf" {
		t.Errorf("expected name 'test_wf', got %q", workflow.Name)
	}
	if workflow.Description != "Test workflow" {
		t.Errorf("expected description 'Test workflow', got %q", workflow.Description)
	}

	// 2 action steps + 1 goal step
	if len(workflow.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(workflow.Steps))
	}

	// Tap step
	step1 := workflow.Steps[0]
	if step1.Tool != "adb_tap" {
		t.Errorf("step 1 tool = %q, want 'adb_tap'", step1.Tool)
	}
	if step1.Args["x"] != 540 {
		t.Errorf("step 1 x = %v, want 540", step1.Args["x"])
	}

	// Swipe step
	step2 := workflow.Steps[1]
	if step2.Tool != "adb_swipe" {
		t.Errorf("step 2 tool = %q, want 'adb_swipe'", step2.Tool)
	}
	if step2.Args["x2"] != 200 {
		t.Errorf("step 2 x2 = %v, want 200", step2.Args["x2"])
	}

	// Goal step
	step3 := workflow.Steps[2]
	if step3.Goal == "" {
		t.Error("step 3 should have a goal")
	}
	if step3.Name != "verify_final_state" {
		t.Errorf("step 3 name = %q, want 'verify_final_state'", step3.Name)
	}
}

func TestBuildWorkflow_NoGoal(t *testing.T) {
	actions := []RecordedAction{
		{Type: "tap", X: 100, Y: 200},
	}

	workflow := buildWorkflowFromActions("test", "", actions, "")

	// 1 action step, no goal step
	if len(workflow.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(workflow.Steps))
	}
	if workflow.Description != "Recorded user actions from Android device" {
		t.Errorf("expected default description, got %q", workflow.Description)
	}
}

func TestParseScreenResolution(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ScreenResolution
		wantErr bool
	}{
		{
			name:  "standard",
			input: "Physical size: 1080x1920",
			want:  ScreenResolution{Width: 1080, Height: 1920},
		},
		{
			name:  "override",
			input: "Physical size: 1080x1920\nOverride size: 720x1280",
			want:  ScreenResolution{Width: 1080, Height: 1920},
		},
		{
			name:  "with whitespace",
			input: "  Physical size: 1080x1920  \n",
			want:  ScreenResolution{Width: 1080, Height: 1920},
		},
		{
			name:    "no match",
			input:   "some random output",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseScreenResolution(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseScreenResolution() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && (got.Width != tt.want.Width || got.Height != tt.want.Height) {
				t.Errorf("parseScreenResolution() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseInputDeviceInfo(t *testing.T) {
	// Real getevent -p output sample (simplified)
	input := `add device 1: /dev/input/event0
  name:     "gpio-keys"
add device 2: /dev/input/event2
  name:     "touchscreen"
  events:
    ABS (0003):
      0035  : value 0, min 0, max 1079, fuzz 0, flat 0, resolution 0
      0036  : value 0, min 0, max 1919, fuzz 0, flat 0, resolution 0
add device 3: /dev/input/event1
  name:     "power-keys"
`
	got, err := parseInputDeviceInfo(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DevicePath != "/dev/input/event2" {
		t.Errorf("DevicePath = %q, want /dev/input/event2", got.DevicePath)
	}
	if got.RawMaxX != 1079 {
		t.Errorf("RawMaxX = %d, want 1079", got.RawMaxX)
	}
	if got.RawMaxY != 1919 {
		t.Errorf("RawMaxY = %d, want 1919", got.RawMaxY)
	}
}

func TestParseInputDeviceInfo_NoTouchDevice(t *testing.T) {
	input := `add device 1: /dev/input/event0
  name:     "gpio-keys"
  events:
    KEY (0001):
      0074  : value 0
`
	_, err := parseInputDeviceInfo(input)
	if err == nil {
		t.Error("expected error for missing touch device")
	}
}

func TestParseInputDeviceInfo_WithLabels(t *testing.T) {
	// Some devices include ABS_MT_POSITION_X labels directly
	input := `add device 2: /dev/input/event3
  name:     "touch_dev"
  events:
    ABS (0003):
      ABS_MT_POSITION_X  : value 0, min 0, max 32767, fuzz 0, flat 0, resolution 0
      ABS_MT_POSITION_Y  : value 0, min 0, max 32767, fuzz 0, flat 0, resolution 0
`
	got, err := parseInputDeviceInfo(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DevicePath != "/dev/input/event3" {
		t.Errorf("DevicePath = %q, want /dev/input/event3", got.DevicePath)
	}
	if got.RawMaxX != 32767 {
		t.Errorf("RawMaxX = %d, want 32767", got.RawMaxX)
	}
	if got.RawMaxY != 32767 {
		t.Errorf("RawMaxY = %d, want 32767", got.RawMaxY)
	}
}

func TestExtractMaxValue(t *testing.T) {
	tests := []struct {
		line string
		want int
	}{
		{"      0035  : value 0, min 0, max 1079, fuzz 0, flat 0, resolution 0", 1079},
		{"      0036  : value 0, min 0, max 1919, fuzz 0", 1919},
		{"      ABS_MT_POSITION_X  : value 0, min 0, max 32767", 32767},
		{"no max here", 0},
	}

	for _, tt := range tests {
		got := extractMaxValue(tt.line)
		if got != tt.want {
			t.Errorf("extractMaxValue(%q) = %d, want %d", tt.line, got, tt.want)
		}
	}
}

func TestEventParser_MultipleActions(t *testing.T) {
	// Two taps followed by volume down
	events := `/dev/input/event2: EV_KEY BTN_TOUCH DOWN
/dev/input/event2: EV_ABS ABS_MT_POSITION_X 00000064
/dev/input/event2: EV_ABS ABS_MT_POSITION_Y 000000c8
/dev/input/event2: EV_SYN SYN_REPORT 00000000
/dev/input/event2: EV_KEY BTN_TOUCH UP
/dev/input/event2: EV_KEY BTN_TOUCH DOWN
/dev/input/event2: EV_ABS ABS_MT_POSITION_X 000001f4
/dev/input/event2: EV_ABS ABS_MT_POSITION_Y 00000258
/dev/input/event2: EV_SYN SYN_REPORT 00000000
/dev/input/event2: EV_KEY BTN_TOUCH UP
/dev/input/event0: EV_KEY KEY_VOLUMEDOWN DOWN`

	device := InputDeviceInfo{
		DevicePath: "/dev/input/event2",
		RawMaxX:    1080,
		RawMaxY:    1920,
	}
	screen := ScreenResolution{Width: 1080, Height: 1920}
	cfg := DefaultRecorderConfig()
	// Disable debounce for this test
	cfg.DebounceWindow = 0

	scanner := bufio.NewScanner(strings.NewReader(events))
	actions, stopped := processEventStream(scanner, device, screen, cfg, "/dev/input/event2")

	if !stopped {
		t.Error("expected recording to be stopped")
	}

	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}

	// 0x64 = 100, 0xc8 = 200
	if actions[0].X != 100 || actions[0].Y != 200 {
		t.Errorf("action 1: got (%d, %d), want (100, 200)", actions[0].X, actions[0].Y)
	}

	// 0x1f4 = 500, 0x258 = 600
	if actions[1].X != 500 || actions[1].Y != 600 {
		t.Errorf("action 2: got (%d, %d), want (500, 600)", actions[1].X, actions[1].Y)
	}
}

func TestEventParser_FilterDevice(t *testing.T) {
	// Events from different devices - only process event2, stop on vol down from event0
	events := `/dev/input/event1: EV_KEY BTN_TOUCH DOWN
/dev/input/event1: EV_ABS ABS_MT_POSITION_X 00000064
/dev/input/event1: EV_ABS ABS_MT_POSITION_Y 000000c8
/dev/input/event1: EV_SYN SYN_REPORT 00000000
/dev/input/event1: EV_KEY BTN_TOUCH UP
/dev/input/event0: EV_KEY KEY_VOLUMEDOWN DOWN`

	device := InputDeviceInfo{
		DevicePath: "/dev/input/event2",
		RawMaxX:    1080,
		RawMaxY:    1920,
	}
	screen := ScreenResolution{Width: 1080, Height: 1920}
	cfg := DefaultRecorderConfig()

	scanner := bufio.NewScanner(strings.NewReader(events))
	actions, stopped := processEventStream(scanner, device, screen, cfg, "/dev/input/event2")

	if !stopped {
		t.Error("expected recording to be stopped by volume down on event0")
	}

	// event1 events should be filtered out since we target event2
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions (filtered), got %d", len(actions))
	}
}
