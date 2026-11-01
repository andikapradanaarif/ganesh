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

// Synthetic object-params notifications matching the inspected source. The
// native hook payload has no prompt/request ID or automatic grant authority.
func permissionBody(outcome string) string {
	if outcome == "" {
		return strings.ReplaceAll(attempt, "PreToolUse", "PermissionRequest")
	}
	body := strings.ReplaceAll(attempt, "PreToolUse", "PermissionResult")
	return strings.TrimSuffix(body, "}") + `,"decision":"` + outcome + `"}`
}

func TestPermissionObservationSetAndRetries(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "spool")
	// Outcome order deliberately differs from a presumed prompt lifecycle.
	// These are independent observations, not an ordered grant history.
	for _, tc := range []struct{ outcome, event, id string }{
		{"denied", "permission_result", "cea1e76a-aebd-8ef2-8a32-8243991d835a"},
		{"", "permission_request", "4d69c81d-dc65-8594-8c7a-5be5ee3d9fa0"},
		{"cancelled", "permission_result", "2cdcab41-856e-8c74-b243-c3f32a987fb2"},
		{"granted", "permission_result", "dd1c650d-aebf-8ee1-88a2-9b0d8fe89db8"},
	} {
		want := capture.Metadata{SchemaVersion: 1, EventID: tc.id, SessionID: "00000000-0000-4000-8000-000000000002", Event: tc.event, Outcome: tc.outcome}
		if tc.outcome == "" {
			want.Outcome = "observed"
		}
		// Fixed phase hashes were computed separately with Python hashlib/uuid.
		m, err := ReduceStream(context.Background(), iotest.OneByteReader(strings.NewReader(permissionBody(tc.outcome))))
		if err != nil || m != want {
			t.Fatalf("permission reduction: %+v %v", m, err)
		}
		first, err := (capture.Spool{Directory: directory}).Append(context.Background(), m)
		if err != nil || first.Duplicate {
			t.Fatalf("first permission observation: %+v %v", first, err)
		}
		// Reopen the spool for exact retry; no in-memory receipt is needed.
		again, err := (capture.Spool{Directory: directory}).Append(context.Background(), m)
		if err != nil || !again.Duplicate || !again.CapturedAt.Equal(first.CapturedAt) {
			t.Fatalf("permission retry: %+v %v", again, err)
		}
		changed, err := Reduce([]byte(strings.ReplaceAll(permissionBody(tc.outcome), "ghp_SYNTHETIC_SECRET", "different discarded params")))
		if err != nil || changed != want {
			t.Fatal("discarded prompt content changed observation identity")
		}
		body, err := os.ReadFile(filepath.Join(directory, tc.id+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var record capture.Record
		if json.Unmarshal(body, &record) != nil || record.Metadata != want || record.State != "captured_local" || record.Origin != "unknown" || record.Coverage != "incomplete" {
			t.Fatal("permission record gained authority or lost metadata")
		}
		for _, forbidden := range []string{"SYNTHETIC_SECRET", "/synthetic", "call-1", "tool_input", "decision"} {
			if strings.Contains(string(body), forbidden) {
				t.Fatal("native permission content persisted")
			}
		}
	}
	for _, body := range []string{attempt, resultBody(false)} {
		m, err := Reduce([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		if receipt, err := (capture.Spool{Directory: directory}).Append(context.Background(), m); err != nil || receipt.Duplicate {
			t.Fatal("permission observation collided with tool execution evidence")
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			count++
		}
	}
	if count != 6 {
		t.Fatalf("permission/tool observation set has %d records", count)
	}
}

func FuzzPermissionDecision(f *testing.F) {
	for _, outcome := range []string{"granted", "denied", "cancelled", "allow", "", "ghp_SYNTHETIC_SECRET"} {
		f.Add(outcome, byte(0))
	}
	f.Fuzz(func(t *testing.T, outcome string, which byte) {
		if len(outcome) > 8*1024 {
			t.Skip()
		}
		// Use the independent stdlib encoder to keep the envelope valid while
		// varying the decision data and the event carrying it.
		event := []string{"PermissionResult", "PermissionRequest", "PreToolUse"}[which%3]
		encoded, err := json.Marshal(outcome)
		if err != nil {
			t.Fatal(err)
		}
		body := strings.TrimSuffix(strings.ReplaceAll(attempt, "PreToolUse", event), "}") + `,"decision":` + string(encoded) + "}"
		got, err := Reduce([]byte(body))
		allowed := event == "PermissionResult" && (outcome == "granted" || outcome == "denied" || outcome == "cancelled")
		if (err == nil) != allowed || allowed && (got.Event != "permission_result" || got.Outcome != outcome) || !allowed && got != (capture.Metadata{}) {
			t.Fatal("permission input disagrees with independent event/enum expectation")
		}
	})
}

func TestPermissionEnvelopeAdmission(t *testing.T) {
	result := permissionBody("denied")
	request := permissionBody("")
	for _, body := range []string{
		strings.Replace(result, `,"decision":"denied"`, "", 1),
		strings.Replace(result, `"decision":"denied"`, `"decision":null`, 1),
		strings.Replace(result, `"decision":"denied"`, `"decision":true`, 1),
		strings.Replace(result, `"decision":"denied"`, `"decision":""`, 1),
		permissionBody("allow"), permissionBody("deny"), permissionBody("Denied"), permissionBody("interrupted"), permissionBody("ghp_SYNTHETIC_SECRET"),
		strings.Replace(result, `"decision":"denied"`, `"decision":"denied","\u0064ecision":"denied"`, 1),
		strings.TrimSuffix(result, "}") + `,"tool_response":"ghp_SYNTHETIC_SECRET"}`,
		strings.TrimSuffix(result, "}") + `,"error":"ghp_SYNTHETIC_SECRET"}`,
		strings.TrimSuffix(request, "}") + `,"decision":"granted"}`,
		strings.TrimSuffix(request, "}") + `,"error":"ghp_SYNTHETIC_SECRET"}`,
		strings.Replace(result, `"PermissionResult"`, `"PermissionRequest"`, 1),
		strings.Replace(request, `"tool_input":{"file_path":"/synthetic/private","content":"ghp_SYNTHETIC_SECRET"}`, `"tool_input":null`, 1),
		strings.TrimSuffix(attempt, "}") + `,"decision":"granted"}`,
		strings.TrimSuffix(resultBody(false), "}") + `,"decision":"granted"}`,
		strings.TrimSuffix(resultBody(true), "}") + `,"decision":"denied"}`,
	} {
		if m, err := Reduce([]byte(body)); !errors.Is(err, ErrPayload) || m != (capture.Metadata{}) || strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
			t.Fatalf("unsafe permission admission: %+v %v", m, err)
		}
	}
	escaped := strings.Replace(result, `"decision":"denied"`, `"\u0064ecision":"\u0064enied"`, 1)
	got, err := Reduce([]byte(escaped))
	want, _ := Reduce([]byte(result))
	if err != nil || got != want {
		t.Fatal("escaped permission enum/key changed identity")
	}
}
