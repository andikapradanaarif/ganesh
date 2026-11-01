// Package crush reduces a pinned subset of native Crush hook input. It does
// not authenticate the client, establish source provenance or claim coverage.
package crush

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/andikapradanaarif/ganesh/internal/capture"
)

// Profile names the inspected source schema, not an installed-client approval.
const Profile = "crush-tool-hooks-v1-aed2bae"

var ErrPayload = errors.New("invalid or unsupported Crush tool-hook input")

// Reduce is the byte-slice convenience entry point for synthetic fixtures.
// The hook CLI calls ReduceStream directly and never buffers the native body.
func Reduce(body []byte) (capture.Metadata, error) {
	return ReduceStream(context.Background(), bytes.NewReader(body))
}

func nativeMetadata(values [6]string, seen uint16) (capture.Metadata, error) {
	var empty capture.Metadata
	const base = (1 << eventField) | (1 << aliasField) | (1 << sessionField) | (1 << cwdField)
	if seen&base != base || values[eventField] != values[aliasField] {
		return empty, ErrPayload
	}
	switch values[eventField] {
	case "PreCompact", "PostCompact":
		return compactMetadata(values, seen, base)
	case "TurnEnd", "StopFailure", "Interrupt":
		return turnMetadata(values, seen, base)
	}
	if seen&((1<<assistantField)|(1<<triggerField)) != 0 {
		return empty, ErrPayload
	}
	const required = base | (1 << callField) | (1 << nameField) | (1 << inputField)
	if seen&required != required || !callID(values[callField]) {
		return empty, ErrPayload
	}
	if values[eventField] != "PermissionResult" && seen&(1<<decisionField) != 0 {
		return empty, ErrPayload
	}
	phase, outcome, event := "result", "", "tool_result"
	switch values[eventField] {
	case "PreToolUse":
		phase, outcome, event = "attempt", "unknown", "tool_attempt"
		if seen&((1<<responseField)|(1<<errorField)) != 0 {
			return empty, ErrPayload
		}
	case "PostToolUse":
		outcome = "succeeded"
		if seen&(1<<responseField) == 0 || seen&(1<<errorField) != 0 {
			return empty, ErrPayload
		}
	case "PostToolUseFailure":
		outcome = "failed"
		if seen&(1<<responseField) != 0 {
			return empty, ErrPayload
		}
	case "PermissionRequest":
		phase, outcome, event = "permission_request", "observed", "permission_request"
		if seen&((1<<responseField)|(1<<errorField)) != 0 {
			return empty, ErrPayload
		}
	case "PermissionResult":
		if seen&(1<<decisionField) == 0 || seen&((1<<responseField)|(1<<errorField)) != 0 {
			return empty, ErrPayload
		}
		switch values[decisionField] {
		case "granted", "denied", "cancelled":
			outcome = values[decisionField]
		default:
			return empty, ErrPayload
		}
		event = "permission_result"
		// The native payload omits the prompt ID. Distinct outcomes may be
		// from different prompts for one call. Preserve the observed set;
		// never claim a prompt count, order or resolved contradiction.
		phase = "permission_result/" + outcome
	default:
		return empty, ErrPayload
	}
	m := capture.Metadata{SchemaVersion: 1, EventID: eventID(values[sessionField], values[callField], phase), SessionID: values[sessionField], Event: event, Outcome: outcome}
	if m.Validate() != nil {
		return empty, ErrPayload
	}
	return m, nil
}

func compactMetadata(values [6]string, seen, base uint16) (capture.Metadata, error) {
	var empty capture.Metadata
	const trigger = 1 << triggerField
	if seen != base|trigger || (values[triggerField] != "auto" && values[triggerField] != "manual") {
		return empty, ErrPayload
	}
	event := "pre_compact"
	if values[eventField] == "PostCompact" {
		event = "post_compact"
	}
	// No compaction ID is supplied. Trigger is validated then discarded;
	// both triggers and repeated invocations collapse to one phase marker.
	m := capture.Metadata{SchemaVersion: 1, EventID: eventID(values[sessionField], "", "compact/"+values[eventField]), SessionID: values[sessionField], Event: event, Outcome: "observed"}
	if m.Validate() != nil {
		return empty, ErrPayload
	}
	return m, nil
}

func turnMetadata(values [6]string, seen, allowed uint16) (capture.Metadata, error) {
	var empty capture.Metadata
	outcome := ""
	switch values[eventField] {
	case "TurnEnd":
		allowed |= 1 << assistantField
		outcome = "observed" // A normal ending is not proof of task success.
	case "StopFailure":
		allowed |= 1 << errorField
		outcome = "failed"
	case "Interrupt":
		outcome = "interrupted"
	default:
		return empty, ErrPayload
	}
	if seen & ^allowed != 0 {
		return empty, ErrPayload
	}
	// The native notification has no turn/call ID. An empty call component
	// plus a distinct phase identifies only a per-session ending-type marker.
	// Repeated turns collapse; neither count nor order can be reconstructed.
	m := capture.Metadata{SchemaVersion: 1, EventID: eventID(values[sessionField], "", "turn/"+values[eventField]), SessionID: values[sessionField], Event: "turn_stop", Outcome: outcome}
	if m.Validate() != nil {
		return empty, ErrPayload
	}
	return m, nil
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
