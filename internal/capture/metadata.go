// Package capture implements the local S0 metadata spool foundation.
// It accepts only a normalized envelope, never raw client hook JSON. No client
// capture profile or production provenance eligibility is established here.
package capture

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
)

const MaxInput = 64 * 1024

type Metadata struct {
	SchemaVersion int    `json:"schema_version"`
	EventID       string `json:"event_id"`
	SessionID     string `json:"session_id"`
	Event         string `json:"event"`
	Outcome       string `json:"outcome"`
}

var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Decode uses a structural allowlist rather than storing arbitrary strings and
// hoping a secret detector finds every sensitive value. All errors are content
// free. A client adapter must reduce/scrub before emitting this envelope too.
func Decode(body []byte) (Metadata, error) {
	var m Metadata
	if len(body) > MaxInput {
		return m, errors.New("metadata input exceeds limit")
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return Metadata{}, errors.New("invalid metadata envelope")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return Metadata{}, errors.New("metadata must contain one JSON object")
	}
	if m.SchemaVersion != 1 || !uuid.MatchString(m.EventID) || !uuid.MatchString(m.SessionID) {
		return Metadata{}, errors.New("unsupported schema or invalid UUID identity")
	}
	switch m.Event {
	case "session_start", "session_end", "turn_stop", "tool_attempt", "tool_result", "subagent_start", "subagent_stop", "pre_compact", "post_compact", "instructions_observed":
	default:
		return Metadata{}, errors.New("unsupported metadata event")
	}
	switch m.Outcome {
	case "observed", "succeeded", "failed", "denied", "interrupted", "unknown":
	default:
		return Metadata{}, errors.New("unsupported metadata outcome")
	}
	return m, nil
}
