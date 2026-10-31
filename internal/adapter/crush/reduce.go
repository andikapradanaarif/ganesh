// Package crush reduces a pinned subset of native Crush hook input. It does
// not authenticate the client, establish source provenance or claim coverage.
package crush

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/andikapradanaarif/ganesh/internal/capture"
)

// Profile names the inspected source schema, not an installed-client approval.
const Profile = "crush-tool-hooks-v1-aed2bae"

var ErrPayload = errors.New("invalid or unsupported Crush tool-hook input")

// Reduce drops every content-bearing field before calling the capture core.
// Only the native session UUID and a derived call/phase UUID survive. Repeated
// bodies with the same reduced metadata are duplicates, not proof of equal raw
// input. Neither a reported success nor an ID proves a file mutation occurred.
func Reduce(body []byte) (capture.Metadata, error) {
	var empty capture.Metadata
	if len(body) > capture.MaxInput || !utf8.Valid(body) {
		return empty, ErrPayload
	}
	d := json.NewDecoder(bytes.NewReader(body))
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return empty, ErrPayload
	}
	fields := make(map[string]json.RawMessage)
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return empty, ErrPayload
		}
		key, ok := token.(string)
		if !ok || fields[key] != nil {
			return empty, ErrPayload
		}
		switch key {
		case "event", "hook_event_name", "session_id", "cwd", "tool_name", "tool_input", "tool_use_id", "tool_response", "error":
		default:
			return empty, ErrPayload
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return empty, ErrPayload
		}
		fields[key] = value
	}
	last, err := d.Token()
	if err != nil || last != json.Delim('}') || d.Decode(new(any)) != io.EOF {
		return empty, ErrPayload
	}
	var event, alias, session, call, cwd, name string
	for key, target := range map[string]*string{
		"event": &event, "hook_event_name": &alias, "session_id": &session,
		"tool_use_id": &call, "cwd": &cwd, "tool_name": &name,
	} {
		if !readString(fields[key], target) || *target == "" {
			return empty, ErrPayload
		}
	}
	input := bytes.TrimSpace(fields["tool_input"])
	if len(input) == 0 || input[0] != '{' || event != alias || !callID(call) {
		return empty, ErrPayload
	}
	phase, outcome := "result", ""
	switch event {
	case "PreToolUse":
		phase, outcome = "attempt", "unknown"
		if fields["tool_response"] != nil || fields["error"] != nil {
			return empty, ErrPayload
		}
	case "PostToolUse":
		outcome = "succeeded"
		var ignored string
		if !readString(fields["tool_response"], &ignored) || fields["error"] != nil {
			return empty, ErrPayload
		}
	case "PostToolUseFailure":
		outcome = "failed"
		var ignored string
		if fields["tool_response"] != nil || (fields["error"] != nil && !readString(fields["error"], &ignored)) {
			return empty, ErrPayload
		}
	default:
		return empty, ErrPayload
	}
	m := capture.Metadata{SchemaVersion: 1, EventID: eventID(session, call, phase), SessionID: session, Event: "tool_result", Outcome: outcome}
	if phase == "attempt" {
		m.Event = "tool_attempt"
	}
	if m.Validate() != nil {
		return empty, ErrPayload
	}
	return m, nil
}

func readString(raw json.RawMessage, target *string) bool {
	return len(raw) > 0 && raw[0] == '"' && json.Unmarshal(raw, target) == nil
}

func callID(value string) bool {
	if len(value) == 0 || len(value) > 160 {
		return false
	}
	// Reject known credential prefixes even in this declared ID path. This
	// narrow check is not a general secret scanner; other raw fields are dropped.
	for _, prefix := range []string{"ghp_", "gho_", "ghu_", "ghs_", "ghr_", "github_pat_", "sk-", "AKIA", "ASIA"} {
		if strings.HasPrefix(value, prefix) {
			return false
		}
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func eventID(session, call, phase string) string {
	// NUL cannot appear in the admitted components. Success and failure share
	// the result identity so contradictory outcomes conflict in the spool.
	sum := sha256.Sum256([]byte(Profile + "\x00" + session + "\x00" + call + "\x00" + phase))
	id := sum[:16]
	id[6] = id[6]&0x0f | 0x80 // UUID version 8, application-defined identity.
	id[8] = id[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])
}
