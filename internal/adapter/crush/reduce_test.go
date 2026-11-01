package crush

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andikapradanaarif/ganesh/internal/capture"
)

// Synthetic native-schema inputs, never copied from a user session.
const attempt = `{"event":"PreToolUse","hook_event_name":"PreToolUse","session_id":"00000000-0000-4000-8000-000000000002","cwd":"/synthetic/ghp_SYNTHETIC_SECRET","tool_name":"write","tool_input":{"file_path":"/synthetic/private","content":"ghp_SYNTHETIC_SECRET"},"tool_use_id":"call-1"}`

func resultBody(failed bool) string {
	event, suffix := "PostToolUse", `,"tool_response":"ghp_SYNTHETIC_SECRET"}`
	if failed {
		event, suffix = "PostToolUseFailure", `,"error":"ghp_SYNTHETIC_SECRET"}`
	}
	return strings.TrimSuffix(strings.ReplaceAll(attempt, "PreToolUse", event), "}") + suffix
}

func TestReductionAndDurableRetry(t *testing.T) {
	spool := capture.Spool{Directory: filepath.Join(t.TempDir(), "spool")}
	for _, tc := range []struct {
		body, event, outcome, id string
	}{
		{attempt, "tool_attempt", "unknown", "b337d9f5-b4f4-8afe-afb0-ddfcf33a078c"},
		{resultBody(false), "tool_result", "succeeded", "34de2926-c693-8d74-b48a-54ad575f5835"},
	} {
		m, err := Reduce([]byte(tc.body))
		if err != nil {
			t.Fatal(err)
		}
		// Fixed IDs were calculated independently with Python hashlib/uuid.
		if m.SchemaVersion != 1 || m.SessionID != "00000000-0000-4000-8000-000000000002" || m.EventID != tc.id || m.Event != tc.event || m.Outcome != tc.outcome {
			t.Fatalf("unexpected metadata: %+v", m)
		}
		first, err := spool.Append(context.Background(), m)
		if err != nil || first.Duplicate {
			t.Fatalf("first capture: %+v %v", first, err)
		}
		again, err := spool.Append(context.Background(), m)
		if err != nil || !again.Duplicate || !again.CapturedAt.Equal(first.CapturedAt) {
			t.Fatalf("retry: %+v %v", again, err)
		}
		body, err := os.ReadFile(filepath.Join(spool.Directory, tc.id+".json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"ghp_", "/synthetic", "call-1", "tool_input", "tool_response", "write"} {
			if bytes.Contains(body, []byte(forbidden)) {
				t.Fatal("native content persisted")
			}
		}
		var record capture.Record
		if json.Unmarshal(body, &record) != nil || record.State != "captured_local" || record.Origin != "unknown" || record.Coverage != "incomplete" {
			t.Fatal("incorrect evidence labels")
		}
	}
	failure, err := Reduce([]byte(resultBody(true)))
	if err != nil || failure.Outcome != "failed" || failure.EventID != "34de2926-c693-8d74-b48a-54ad575f5835" {
		t.Fatalf("failure reduction: %+v %v", failure, err)
	}
	if _, err := spool.Append(context.Background(), failure); !errors.Is(err, capture.ErrConflict) {
		t.Fatalf("contradictory result did not conflict: %v", err)
	}
}

func TestRejectedNativeShapes(t *testing.T) {
	for _, body := range []string{
		`null`, `{}`, `[]`, attempt + `{}`, attempt + ` trailing`,
		strings.Replace(attempt, `"event":"PreToolUse"`, `"event":null`, 1),
		strings.Replace(attempt, `"event":"PreToolUse"`, `"event":"PostToolUse"`, 1),
		strings.Replace(attempt, `"event":"PreToolUse"`, `"event":"PreToolUse","event":"PreToolUse"`, 1),
		strings.Replace(attempt, `"cwd":`, `"unknown":`, 1),
		strings.Replace(attempt, `"tool_use_id":"call-1"`, `"tool_use_id":"ghp_SYNTHETIC_SECRET"`, 1),
		strings.Replace(attempt, `"call-1"`, `"https://synthetic.invalid"`, 1),
		strings.Replace(attempt, `"call-1"`, `""`, 1),
		strings.Replace(attempt, `"call-1"`, `"`+strings.Repeat("a", 161)+`"`, 1),
		strings.Replace(attempt, `00000000-0000-4000-8000-000000000002`, `00000000-0000-0000-0000-000000000000`, 1),
		strings.Replace(attempt, `"tool_name":"write"`, `"tool_name":{}`, 1),
		strings.Replace(attempt, `"tool_input":{"file_path":"/synthetic/private","content":"ghp_SYNTHETIC_SECRET"}`, `"tool_input":null`, 1),
		strings.ReplaceAll(attempt, "PreToolUse", "SessionEnd"),
		attempt[:len(attempt)-1] + `,"error":"ghp_SYNTHETIC_SECRET"}`,
		strings.Replace(resultBody(false), `"tool_response":"ghp_SYNTHETIC_SECRET"`, `"tool_response":null`, 1),
		strings.Replace(resultBody(false), `,"tool_response":"ghp_SYNTHETIC_SECRET"`, ``, 1),
		strings.Replace(resultBody(true), `"error":"ghp_SYNTHETIC_SECRET"`, `"error":42`, 1),
		string([]byte{'{', '"', 0xff, '"', ':', '1', '}'}),
		strings.Repeat(" ", MaxNativeInput) + attempt,
	} {
		if m, err := Reduce([]byte(body)); !errors.Is(err, ErrPayload) || m != (capture.Metadata{}) || strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
			t.Fatalf("unsafe admission/rejection: %+v %v", m, err)
		}
	}
}

func TestIdentityScopeAndContentLoss(t *testing.T) {
	base, _ := Reduce([]byte(attempt))
	for _, body := range []string{
		strings.Replace(attempt, "call-1", "call-2", 1),
		strings.Replace(attempt, "call-1", "Call-1", 1),
		strings.Replace(attempt, "000000000002", "000000000003", 1),
	} {
		m, err := Reduce([]byte(body))
		if err != nil || m.EventID == base.EventID {
			t.Fatal("call/session identity or case lost")
		}
	}
	// Deliberately do not retain or fingerprint content. This does not prove
	// the same raw call was executed twice, or establish transcript coverage.
	changed, err := Reduce([]byte(strings.ReplaceAll(attempt, "ghp_SYNTHETIC_SECRET", "different discarded text")))
	if err != nil || changed != base {
		t.Fatal("discarded content affected reduced identity")
	}
}
