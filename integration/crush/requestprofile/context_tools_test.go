package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func validProfilePayload() obj {
	texts := []string{}
	for _, name := range []string{"explicit", "native_default", "nested_explicit", "global_explicit"} {
		texts = append(texts, probeContexts[name])
	}
	tools := []obj{}
	for _, name := range []string{"view", "ls", "grep", "glob"} {
		tools = append(tools, obj{"type": "function", "name": name})
	}
	return obj{"input": []obj{{"role": "developer", "content": []obj{{"type": "input_text", "text": strings.Join(texts, " ")}}}}, "tools": tools}
}

func TestProfileObserverRejectsMissingContextAndWrongTools(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(obj)
		valid  bool
	}{
		{"valid", func(obj) {}, true},
		{"missing_context", func(p obj) { p["input"] = "no required context" }, false},
		{"forbidden_context", func(p obj) { p["instructions"] = unlistedContext }, false},
		{"extra_mutating_tool", func(p obj) { p["tools"] = append(p["tools"].([]obj), obj{"type": "function", "name": "write"}) }, false},
		{"extra_memory_tool", func(p obj) { p["tools"] = append(p["tools"].([]obj), obj{"type": "function", "name": "recall"}) }, false},
		{"missing_tool", func(p obj) { p["tools"] = p["tools"].([]obj)[:3] }, false},
		{"duplicate_tool", func(p obj) { p["tools"] = append(p["tools"].([]obj), obj{"type": "function", "name": "view"}) }, false},
		{"unknown_tool_shape", func(p obj) { p["tools"] = []obj{{"type": "web_search_preview"}} }, false},
		{"markers_in_unrelated_metadata", func(p obj) { p["metadata"] = p["input"]; p["input"] = "none" }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validProfilePayload()
			tc.mutate(p)
			b, e := json.Marshal(p)
			if e != nil {
				t.Fatal(e)
			}
			if got := profilePassed([]profileObservation{observeProfile(b)}); got != tc.valid {
				t.Fatalf("unexpected admission %t: %+v", got, observeProfile(b))
			}
		})
	}
	for name, marker := range probeContexts {
		t.Run("missing_"+name, func(t *testing.T) {
			b, _ := json.Marshal(validProfilePayload())
			b = []byte(strings.ReplaceAll(string(b), marker, "removed"))
			if profilePassed([]profileObservation{observeProfile(b)}) {
				t.Fatal("one missing context marker admitted")
			}
		})
	}
}

func TestProfileObserverNeedsMainRequestAndChecksEveryRequest(t *testing.T) {
	b, _ := json.Marshal(validProfilePayload())
	main := observeProfile(b)
	aux := observeProfile([]byte(`{"input":"synthetic title request"}`))
	if profilePassed(nil) || profilePassed([]profileObservation{aux}) || !profilePassed([]profileObservation{aux, main}) {
		t.Fatal("auxiliary-only or absent request qualified")
	}
	bad := observeProfile([]byte(`{"input":"` + unlistedContext + `"}`))
	if profilePassed([]profileObservation{main, bad}) || profilePassed([]profileObservation{main, observeProfile([]byte(`{`))}) {
		t.Fatal("later unsafe/malformed request ignored")
	}
	if profilePassed([]profileObservation{main, observeProfile([]byte(`null`))}) {
		t.Fatal("null request ignored")
	}
	encoded, _ := json.Marshal(main)
	for _, marker := range probeContexts {
		if strings.Contains(string(encoded), marker) {
			t.Fatal("raw marker text leaked into metadata report")
		}
	}
}
