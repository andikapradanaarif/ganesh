package crush

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/andikapradanaarif/ganesh/internal/capture"
)

// Synthetic source-shaped turn callback: no turn ID or tool binding is supplied.
const turnEnd = `{"event":"TurnEnd","hook_event_name":"TurnEnd","session_id":"00000000-0000-4000-8000-000000000002","cwd":"/synthetic/ghp_SYNTHETIC_SECRET"}`

func turnBody(kind, field string, content any) string {
	body := strings.ReplaceAll(turnEnd, "TurnEnd", kind)
	if field != "" {
		encoded, _ := json.Marshal(content)
		body = strings.TrimSuffix(body, "}") + `,"` + field + `":` + string(encoded) + "}"
	}
	return body
}

func TestTurnObservationSet(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "spool")
	for _, tc := range []struct{ kind, field, outcome, id string }{
		{"StopFailure", "error", "failed", "725d8d56-4170-8d9e-9643-f94069d2ee2d"},
		{"TurnEnd", "last_assistant_message", "observed", "4d7e68aa-4614-8ceb-b3f4-26f68b4b6b63"},
		{"Interrupt", "", "interrupted", "ad0dc494-a284-8783-90b3-5079d0e25c79"},
	} {
		want := capture.Metadata{SchemaVersion: 1, EventID: tc.id, SessionID: "00000000-0000-4000-8000-000000000002", Event: "turn_stop", Outcome: tc.outcome}
		var first capture.Receipt
		// Source optional fields may be omitted, empty or content-bearing.
		for i, body := range []string{
			turnBody(tc.kind, "", nil), turnBody(tc.kind, tc.field, ""),
			turnBody(tc.kind, tc.field, "ghp_SYNTHETIC_SECRET"),
			turnBody(tc.kind, tc.field, strings.Repeat("x", 128*1024)+"ghp_SYNTHETIC_SECRET"),
		} {
			m, err := ReduceStream(context.Background(), iotest.OneByteReader(strings.NewReader(body)))
			if err != nil || m != want {
				t.Fatalf("turn marker (independent hash fixture): %+v %v", m, err)
			}
			receipt, err := (capture.Spool{Directory: dir}).Append(context.Background(), m)
			if err != nil || receipt.Duplicate != (i > 0) {
				t.Fatalf("turn marker/reopened retry: %+v %v", receipt, err)
			}
			if i == 0 {
				first = receipt
			} else if !receipt.CapturedAt.Equal(first.CapturedAt) {
				t.Fatal("repeated turn type changed observation time")
			}
		}
		body, err := os.ReadFile(filepath.Join(dir, tc.id+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var record capture.Record
		if json.Unmarshal(body, &record) != nil || record.Metadata != want || record.State != "captured_local" || record.Origin != "unknown" || record.Coverage != "incomplete" {
			t.Fatal("turn marker gained closure/eligibility authority")
		}
		for _, forbidden := range []string{"SYNTHETIC_SECRET", "/synthetic", "last_assistant_message", `"error"`} {
			if strings.Contains(string(body), forbidden) {
				t.Fatal("native turn text persisted")
			}
		}
	}
	// Tool and permission evidence stays in its own namespace for this session.
	for _, body := range []string{attempt, resultBody(false), permissionBody(""), permissionBody("denied")} {
		m, err := Reduce([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		if r, err := (capture.Spool{Directory: dir}).Append(context.Background(), m); err != nil || r.Duplicate {
			t.Fatal("turn marker collided with tool/permission identity")
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			count++
		}
	}
	if count != 7 {
		t.Fatalf("unexpected turn/tool/permission record count: %d", count)
	}
}

func TestTurnFieldIsolation(t *testing.T) {
	for _, body := range []string{
		turnBody("TurnEnd", "last_assistant_message", nil),
		turnBody("TurnEnd", "last_assistant_message", true),
		turnBody("StopFailure", "error", 12),
		turnBody("StopFailure", "error", nil),
		turnBody("TurnEnd", "error", "ghp_SYNTHETIC_SECRET"),
		turnBody("Interrupt", "error", ""),
		turnBody("Interrupt", "last_assistant_message", ""),
		turnBody("StopFailure", "last_assistant_message", ""),
		turnBody("TurnEnd", "tool_use_id", "call-1"),
		turnBody("StopFailure", "tool_name", "write"),
		turnBody("Interrupt", "tool_input", map[string]any{}),
		turnBody("TurnEnd", "tool_response", ""),
		turnBody("Interrupt", "decision", "cancelled"),
		turnBody("TurnStart", "", nil), turnBody("SessionEnd", "", nil),
		strings.Replace(turnEnd, `"event":"TurnEnd"`, `"event":"Interrupt"`, 1),
		strings.Replace(turnEnd, `"cwd":"/synthetic/ghp_SYNTHETIC_SECRET"`, `"cwd":""`, 1),
		strings.Replace(turnEnd, `,"cwd":"/synthetic/ghp_SYNTHETIC_SECRET"`, "", 1),
		strings.Replace(turnEnd, "00000000-0000-4000-8000-000000000002", "child$$call", 1),
		strings.TrimSuffix(turnBody("TurnEnd", "last_assistant_message", ""), "}") + `,"last_assistant_\u006dessage":"ghp_SYNTHETIC_SECRET"}`,
	} {
		if m, err := Reduce([]byte(body)); !errors.Is(err, ErrPayload) || m != (capture.Metadata{}) || strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
			t.Fatalf("unsafe turn admission: %+v %v", m, err)
		}
	}
	for _, body := range []string{attempt, resultBody(false), resultBody(true), permissionBody(""), permissionBody("denied")} {
		if _, err := Reduce([]byte(strings.TrimSuffix(body, "}") + `,"last_assistant_message":""}`)); !errors.Is(err, ErrPayload) {
			t.Fatal("turn content admitted on tool/permission event")
		}
	}
	for _, body := range []string{
		strings.Replace(turnEnd, `"session_id"`, `"session_\u0069d"`, 1),
		strings.Replace(turnBody("TurnEnd", "last_assistant_message", ""), `"last_assistant_message"`, `"last_assistant_\u006dessage"`, 1),
	} {
		got, err := Reduce([]byte(body))
		want, _ := Reduce([]byte(turnEnd))
		if err != nil || got != want {
			t.Fatal("escaped turn fields changed marker identity")
		}
	}
}

func FuzzTurnOptionalFields(f *testing.F) {
	f.Add(byte(0), byte(0), "ghp_SYNTHETIC_SECRET")
	f.Add(byte(1), byte(1), "")
	f.Add(byte(2), byte(2), "text")
	f.Fuzz(func(t *testing.T, which, fieldChoice byte, content string) {
		if len(content) > 8*1024 {
			t.Skip()
		}
		kind := []string{"TurnEnd", "StopFailure", "Interrupt"}[which%3]
		field := []string{"last_assistant_message", "error", "decision"}[fieldChoice%3]
		body := turnBody(kind, field, content)
		got, err := ReduceStream(context.Background(), strings.NewReader(body))
		allowed := kind == "TurnEnd" && field == "last_assistant_message" || kind == "StopFailure" && field == "error"
		if (err == nil) != allowed || !allowed && got != (capture.Metadata{}) {
			t.Fatal("turn field isolation differs from independent source-shape expectation")
		}
		if allowed && (got.Event != "turn_stop" || got.Outcome == "succeeded") {
			t.Fatal("turn observation became task success")
		}
	})
}
