package live

import (
	"encoding/json"
	"testing"

	"github.com/pepebot-space/pepebot/pkg/config"
)

// setupSystemInstructionText extracts setup.systemInstruction.parts[0].text from a
// BidiGenerateContentSetup payload, returning ("", false) when absent.
func setupSystemInstructionText(t *testing.T, data []byte) (string, bool) {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("invalid setup JSON: %v", err)
	}
	inner, ok := m["setup"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing setup object")
	}
	si, ok := inner["systemInstruction"].(map[string]interface{})
	if !ok {
		return "", false
	}
	parts, ok := si["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		t.Fatalf("systemInstruction has no parts")
	}
	p0, _ := parts[0].(map[string]interface{})
	txt, _ := p0["text"].(string)
	return txt, true
}

func TestSetupMessageSystemInstructionFromConfig(t *testing.T) {
	const prompt = "You are LEXA, an autonomous rover."
	p := &GeminiLiveProvider{liveConfig: config.LiveConfig{SystemPrompt: prompt}}

	txt, present := setupSystemInstructionText(t, p.SetupMessage("gemini-live-2.5-flash-native-audio"))
	if !present {
		t.Fatal("expected systemInstruction when live.system_prompt is set")
	}
	if txt != prompt {
		t.Fatalf("systemInstruction text = %q, want %q", txt, prompt)
	}
}

func TestSetupMessageNoSystemInstructionWhenUnset(t *testing.T) {
	p := &GeminiLiveProvider{liveConfig: config.LiveConfig{}}

	if _, present := setupSystemInstructionText(t, p.SetupMessage("gemini-live-2.5-flash-native-audio")); present {
		t.Fatal("expected no systemInstruction when no prompt configured (byte-identical/no regression)")
	}
}

func TestInjectGeminiSystemInstruction(t *testing.T) {
	base := []byte(`{"setup":{"model":"m","generationConfig":{}}}`)

	// Empty prompt is a no-op.
	if got := injectGeminiSystemInstruction(base, ""); string(got) != string(base) {
		t.Fatalf("empty prompt should not modify setup; got %s", got)
	}

	// Non-empty prompt is injected/replaced.
	out := injectGeminiSystemInstruction(base, "be a rover")
	txt, present := setupSystemInstructionText(t, out)
	if !present || txt != "be a rover" {
		t.Fatalf("inject failed: present=%v text=%q", present, txt)
	}

	// Override replaces an existing instruction.
	out2 := injectGeminiSystemInstruction(out, "be a drone")
	if txt2, _ := setupSystemInstructionText(t, out2); txt2 != "be a drone" {
		t.Fatalf("override failed: text=%q want %q", txt2, "be a drone")
	}
}
