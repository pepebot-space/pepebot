// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package config

import (
	"fmt"
	"strings"
)

// Validate reports configuration that will not work, or will work in a way
// nobody intended.
//
// Every check here is something that actually went wrong on a live deployment
// and cost an afternoon to find, because the symptom appeared far from the
// cause: a model id with the provider name glued to its front came back as "no
// healthy deployments", a custom endpoint that was never defined came back as a
// dial error, and an output budget doubling as the context window compacted
// every session at a fraction of the real limit — silently, forever.
//
// It returns problems rather than failing: a bot that starts with a warning is
// more useful than one that refuses to start, and nothing here is unrecoverable.
func (c *Config) Validate() []string {
	var problems []string

	d := c.Agents.Defaults

	if strings.TrimSpace(d.Model) == "" {
		problems = append(problems, "agents.defaults.model is empty — nothing will answer")
	}

	// "<provider>/<provider>/<model>": the provider name repeated inside the
	// model id. Endpoints reject the whole string, and the message names the
	// model, not the config.
	if provider := strings.ToLower(strings.TrimSpace(d.Provider)); provider != "" {
		if strings.HasPrefix(strings.ToLower(d.Model), provider+"/") {
			problems = append(problems, fmt.Sprintf(
				"agents.defaults.model %q repeats the provider name; write %q, or use the canonical %q",
				d.Model, strings.TrimPrefix(d.Model, provider+"/"), provider+":"+strings.TrimPrefix(d.Model, provider+"/")))
		}
	}

	// A custom endpoint has to exist before a model can point at it.
	if name, _, ok := customTarget(d.Model); ok {
		entry, defined := c.Providers.Custom[name]
		switch {
		case !defined:
			problems = append(problems, fmt.Sprintf(
				"model %q names custom provider %q, which is not defined under providers.custom", d.Model, name))
		case strings.TrimSpace(entry.APIBase) == "":
			problems = append(problems, fmt.Sprintf("providers.custom.%s has no api_base", name))
		}
	}

	for name, entry := range c.Providers.Custom {
		if strings.TrimSpace(entry.APIBase) == "" {
			problems = append(problems, fmt.Sprintf("providers.custom.%s has no api_base", name))
		}
	}

	// One number cannot be both the reply budget and the input window. When it
	// was, sessions on a 128k model were compacted at 75% of 8k.
	if d.ContextWindow > 0 && d.MaxTokens > 0 && d.ContextWindow < d.MaxTokens {
		problems = append(problems, fmt.Sprintf(
			"agents.defaults.context_window (%d) is smaller than max_tokens (%d); the context window is the input limit, max_tokens the reply budget",
			d.ContextWindow, d.MaxTokens))
	}

	if d.MaxToolIterations < 0 {
		problems = append(problems, "agents.defaults.max_tool_iterations is negative; tools will never run")
	}

	if c.Memory.Enabled {
		if c.Memory.CharLimit < 0 || c.Memory.UserCharLimit < 0 {
			problems = append(problems, "memory character limits cannot be negative")
		}
		if c.Memory.Review && c.Memory.ReviewInterval <= 0 {
			problems = append(problems, "memory.review is on but memory.review_interval is not positive, so the review never runs")
		}
	}

	if c.Live.Enabled && strings.TrimSpace(c.Live.Provider) == "" {
		problems = append(problems, "live.enabled is true but live.provider is empty")
	}

	return problems
}

// customTarget splits a "custom:<endpoint>/<model>" reference. It is kept here
// rather than imported so config stays free of provider dependencies.
func customTarget(model string) (endpoint, rest string, ok bool) {
	const prefix = "custom:"
	if !strings.HasPrefix(strings.ToLower(model), prefix) {
		return "", "", false
	}
	target := model[len(prefix):]
	if slash := strings.Index(target, "/"); slash > 0 {
		return target[:slash], target[slash+1:], true
	}
	return target, "", true
}
