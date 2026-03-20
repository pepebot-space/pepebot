package live

import "strings"

func withVideoResponseModalities(generationConfig map[string]interface{}, enableVideo bool) map[string]interface{} {
	if generationConfig == nil {
		return nil
	}

	copied := make(map[string]interface{}, len(generationConfig))
	for k, v := range generationConfig {
		copied[k] = v
	}

	if !enableVideo {
		return copied
	}

	modalities := extractResponseModalities(copied["responseModalities"])
	if len(modalities) == 0 {
		modalities = []string{"AUDIO"}
	}

	hasVideo := false
	for _, m := range modalities {
		if strings.EqualFold(m, "VIDEO") {
			hasVideo = true
			break
		}
	}

	if !hasVideo {
		modalities = append(modalities, "VIDEO")
	}

	copied["responseModalities"] = modalities
	return copied
}

func extractResponseModalities(raw interface{}) []string {
	switch v := raw.(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, m := range v {
			m = strings.TrimSpace(m)
			if m != "" {
				out = append(out, m)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, it := range v {
			if s, ok := it.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	default:
		return nil
	}
}
