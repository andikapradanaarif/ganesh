// Package capture implements an S0 local metadata primitive, not a client
// adapter or a source-provenance verifier. Every record remains origin-unknown.
package capture

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"unicode/utf8"
)

const MaxInput = 64 * 1024

type Metadata struct {
	SchemaVersion int    `json:"schema_version"`
	EventID       string `json:"event_id"`
	SessionID     string `json:"session_id"`
	Event         string `json:"event"`
	Outcome       string `json:"outcome"`
}

var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

var ErrMetadata = errors.New("invalid normalized capture metadata")

// Decode accepts only this flat, reduced schema. Raw client hook objects and
// arbitrary content cannot be stored by labelling them metadata. This is not a
// general-purpose secret scanner or proof that a UUID came from a client.
func Decode(body []byte) (Metadata, error) {
	var result Metadata
	if len(body) > MaxInput || !utf8.Valid(body) {
		return result, ErrMetadata
	}
	d := json.NewDecoder(bytes.NewReader(body))
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return result, ErrMetadata
	}
	seen := map[string]bool{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return Metadata{}, ErrMetadata
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return Metadata{}, ErrMetadata
		}
		seen[key] = true
		switch key {
		case "schema_version":
			err = d.Decode(&result.SchemaVersion)
		case "event_id":
			err = d.Decode(&result.EventID)
		case "session_id":
			err = d.Decode(&result.SessionID)
		case "event":
			err = d.Decode(&result.Event)
		case "outcome":
			err = d.Decode(&result.Outcome)
		default:
			return Metadata{}, ErrMetadata
		}
		if err != nil {
			return Metadata{}, ErrMetadata
		}
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') {
		return Metadata{}, ErrMetadata
	}
	if d.Decode(new(any)) != io.EOF {
		return Metadata{}, ErrMetadata
	}
	if len(seen) != 5 {
		return Metadata{}, ErrMetadata
	}
	if err := result.Validate(); err != nil {
		return Metadata{}, err
	}
	return result, nil
}

func (m Metadata) Validate() error {
	if m.SchemaVersion != 1 || !uuid.MatchString(m.EventID) || !uuid.MatchString(m.SessionID) {
		return ErrMetadata
	}
	switch m.Event {
	case "session_start", "session_end", "turn_stop", "tool_attempt", "tool_result", "subagent_start", "subagent_stop", "pre_compact", "post_compact", "instructions_observed":
	default:
		return ErrMetadata
	}
	switch m.Outcome {
	case "observed", "succeeded", "failed", "denied", "interrupted", "unknown":
	default:
		return ErrMetadata
	}
	return nil
}
