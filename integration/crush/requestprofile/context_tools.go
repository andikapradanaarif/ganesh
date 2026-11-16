package main

import (
	"encoding/json"
	"slices"
	"strings"
)

// These are synthetic probe inputs, not an adopted S0 answering profile.
var probeTools = []string{"glob", "grep", "ls", "view"}
var probeContexts = map[string]string{
	"explicit":        "S0_PROBE_EXPLICIT_74b3e8d1",
	"native_default":  "S0_PROBE_NATIVE_95a2c6e0",
	"nested_explicit": "S0_PROBE_NESTED_a730d9e2",
	"global_explicit": "S0_PROBE_GLOBAL_c25e1f48",
}

const unlistedContext = "S0_PROBE_UNLISTED_d84e7a30"

type profileObservation struct {
	Tools    []string        `json:"tool_names"`
	Main     bool            `json:"has_tool_palette"`
	Context  map[string]bool `json:"synthetic_context_present"`
	Unlisted bool            `json:"unlisted_context_present"`
	Errors   []string        `json:"errors"`
}

// Inspect actual Responses request fields, not the config's advertised intent.
// Only names and marker booleans leave this function; bodies remain private.
func observeProfile(b []byte) profileObservation {
	o := profileObservation{Tools: []string{}, Context: map[string]bool{}, Errors: []string{}}
	var p struct {
		Instructions string          `json:"instructions"`
		Input        json.RawMessage `json:"input"`
		Tools        []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"tools"`
	}
	if !strings.HasPrefix(strings.TrimSpace(string(b)), "{") || json.Unmarshal(b, &p) != nil {
		o.Errors = append(o.Errors, "request_json")
		return o
	}
	texts := []string{p.Instructions}
	var input any
	if len(p.Input) > 0 && json.Unmarshal(p.Input, &input) != nil {
		o.Errors = append(o.Errors, "input_json")
		return o
	}
	var collect func(any)
	collect = func(v any) {
		switch x := v.(type) {
		case string:
			texts = append(texts, x)
		case []any:
			for _, e := range x {
				collect(e)
			}
		case map[string]any:
			// Ignore unrelated metadata/schema/name fields. Context is text.
			for _, k := range []string{"content", "text"} {
				if value, ok := x[k]; ok {
					collect(value)
				}
			}
		}
	}
	collect(input)
	text := strings.Join(texts, "\n")
	for name, marker := range probeContexts {
		o.Context[name] = strings.Contains(text, marker)
	}
	o.Unlisted = strings.Contains(text, unlistedContext)
	if o.Unlisted {
		o.Errors = append(o.Errors, "unlisted_context")
	}
	for _, tool := range p.Tools {
		if tool.Type != "function" || tool.Name == "" {
			o.Errors = append(o.Errors, "tool_shape")
			continue
		}
		if slices.Contains(o.Tools, tool.Name) {
			o.Errors = append(o.Errors, "duplicate_tool")
		}
		o.Tools = append(o.Tools, tool.Name)
	}
	o.Main = len(p.Tools) > 0
	slices.Sort(o.Tools)
	if o.Main {
		if !slices.Equal(o.Tools, probeTools) {
			o.Errors = append(o.Errors, "tool_palette")
		}
		for _, present := range o.Context {
			if !present {
				o.Errors = append(o.Errors, "missing_context")
				break
			}
		}
	}
	return o
}

func profilePassed(observations []profileObservation) bool {
	main := 0
	for _, o := range observations {
		if len(o.Errors) != 0 {
			return false
		}
		if o.Main {
			main++
		}
	}
	// Auxiliary requests alone cannot qualify the main prompt/tool probe.
	return main > 0
}
