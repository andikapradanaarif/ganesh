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

func nativeMetadata(values [4]string, seen uint16) (capture.Metadata, error) {
	var empty capture.Metadata
	const required = (1 << (inputField + 1)) - 1
	if seen&required != required || values[eventField] != values[aliasField] || !callID(values[callField]) {
		return empty, ErrPayload
	}
	phase, outcome := "result", ""
	switch values[eventField] {
	case "PreToolUse":
		phase, outcome = "attempt", "unknown"
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
	default:
		return empty, ErrPayload
	}
	m := capture.Metadata{SchemaVersion: 1, EventID: eventID(values[sessionField], values[callField], phase), SessionID: values[sessionField], Event: "tool_result", Outcome: outcome}
	if phase == "attempt" {
		m.Event = "tool_attempt"
	}
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
