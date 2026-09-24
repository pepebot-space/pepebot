// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package providers

import "strings"

// ModelRef is a resolved answer to the only two questions the factory has:
// which endpoint, and what model id to put on the wire.
type ModelRef struct {
	// Provider names the endpoint — a built-in key ("zai", "openai") or
	// "custom", in which case Endpoint names the entry in providers.custom.
	Provider string
	// Endpoint is the custom provider's name, empty for built-ins.
	Endpoint string
	// Model is what the upstream is actually asked for.
	Model string
}

// ParseModelRef resolves the one canonical spelling, "<provider>:<model_id>",
// against the provider field that may also be set.
//
//	"zai:glm-4.5v"                  → zai, glm-4.5v
//	"maiarouter:zai/glm-4.5v"       → maiarouter, zai/glm-4.5v
//	"custom:ollama/qwen3"           → custom, endpoint ollama, model qwen3
//	provider "zai" + "glm-4.5v"     → zai, glm-4.5v
//
// The colon form wins over the provider field, because it is the more specific
// of the two and someone who writes it means it.
//
// Splitting on the FIRST colon is what makes the form unambiguous: a vendor
// namespace inside the model id ("zai/glm-4.5v" on a router that wants it kept)
// uses a slash, never a colon, so the two can no longer be confused. That
// confusion is what shipped `maiarouter/zai/glm-5.3-flash` to an API that
// answered "no healthy deployments" (v0.5.20).
func ParseModelRef(provider, model string) ModelRef {
	model = strings.TrimSpace(model)
	provider = strings.ToLower(strings.TrimSpace(provider))

	if idx := strings.Index(model, ":"); idx > 0 {
		provider = strings.ToLower(model[:idx])
		model = model[idx+1:]
	}

	ref := ModelRef{Provider: provider, Model: model}

	// A custom endpoint is addressed as custom:<endpoint>/<model>: the first
	// path segment picks which entry in providers.custom answers, the rest is
	// the model that entry is asked for.
	if ref.Provider == "custom" {
		if slash := strings.Index(ref.Model, "/"); slash > 0 {
			ref.Endpoint = ref.Model[:slash]
			ref.Model = ref.Model[slash+1:]
		} else {
			// custom:ollama with no model — the endpoint's own default.
			ref.Endpoint, ref.Model = ref.Model, ""
		}
		return ref
	}

	// With the provider named, the model id carries no provider prefix. Strip one
	// if it is there anyway, which is what older configs and registry entries
	// look like.
	if ref.Provider != "" {
		ref.Model = modelForProvider(ref.Provider, ref.Model)
	}
	return ref
}

// String renders the canonical spelling, so what pepebot reports back is what a
// user can paste into a config.
func (r ModelRef) String() string {
	switch {
	case r.Provider == "custom" && r.Model == "":
		return "custom:" + r.Endpoint
	case r.Provider == "custom":
		return "custom:" + r.Endpoint + "/" + r.Model
	case r.Provider == "":
		return r.Model
	default:
		return r.Provider + ":" + r.Model
	}
}
