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

func compactBody(kind, trigger string) string {
	encoded, _ := json.Marshal(trigger)
	return strings.TrimSuffix(strings.ReplaceAll(turnEnd, "TurnEnd", kind), "}") + `,"trigger":` + string(encoded) + "}"
}

func TestCompactionObservationSet(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "spool")
	for _, tc := range []struct{ kind, event, id string }{
		{"PreCompact", "pre_compact", "c9a31509-0b92-8b5f-a647-73917ce8a4fb"},
		{"PostCompact", "post_compact", "cac173d5-4073-8a86-8372-d6d677fb96c8"},
	} {
		// IDs were calculated separately with Python hashlib, not eventID.
		want := capture.Metadata{SchemaVersion: 1, EventID: tc.id, SessionID: "00000000-0000-4000-8000-000000000002", Event: tc.event, Outcome: "observed"}
		var first capture.Receipt
		for i, trigger := range []string{"auto", "manual", "auto"} {
			m, err := ReduceStream(context.Background(), iotest.OneByteReader(strings.NewReader(compactBody(tc.kind, trigger))))
			if err != nil || m != want {
				t.Fatalf("compaction marker differs from independent fixture: %+v %v", m, err)
			}
			r, err := (capture.Spool{Directory: dir}).Append(context.Background(), m)
			if err != nil || r.Duplicate != (i > 0) {
				t.Fatalf("compaction retry: %+v %v", r, err)
			}
			if i == 0 {
				first = r
			} else if !r.CapturedAt.Equal(first.CapturedAt) {
				t.Fatal("trigger change or repeat renewed a phase marker")
			}
		}
		body, err := os.ReadFile(filepath.Join(dir, tc.id+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var record capture.Record
		if json.Unmarshal(body, &record) != nil || record.Metadata != want || record.State != "captured_local" || record.Origin != "unknown" || record.Coverage != "incomplete" {
			t.Fatal("compaction marker gained coverage or outcome authority")
		}
		for _, forbidden := range []string{"SYNTHETIC_SECRET", "/synthetic", `"trigger"`, `"auto"`, `"manual"`} {
			if strings.Contains(string(body), forbidden) {
				t.Fatal("discarded compaction content persisted")
			}
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
	if count != 2 {
		t.Fatal("compaction markers collided or counted invocations/triggers")
	}
	base := compactBody("PreCompact", "auto")
	want, _ := Reduce([]byte(base))
	for _, body := range []string{
		strings.Replace(base, `"trigger"`, `"tri\u0067ger"`, 1),
		strings.Replace(base, `"auto"`, `"\u0061uto"`, 1),
	} {
		if got, err := Reduce([]byte(body)); err != nil || got != want {
			t.Fatal("escaped compaction field/value changed identity")
		}
	}
	other, err := Reduce([]byte(strings.Replace(base, "00000000-0000-4000-8000-000000000002", "00000000-0000-4000-8000-000000000003", 1)))
	if err != nil || other.EventID == want.EventID {
		t.Fatal("compaction identity crossed session binding")
	}
}

func TestCompactionFieldIsolation(t *testing.T) {
	for _, kind := range []string{"PreCompact", "PostCompact"} {
		base := compactBody(kind, "auto")
		bodies := []string{
			strings.Replace(base, `,"trigger":"auto"`, "", 1),
			strings.Replace(base, `"auto"`, `null`, 1),
			strings.Replace(base, `"auto"`, `true`, 1),
			strings.Replace(base, `"auto"`, `{}`, 1),
			strings.Replace(base, `"auto"`, `[]`, 1),
			strings.Replace(base, `"auto"`, `12`, 1),
			strings.TrimSuffix(base, "}") + `,"tri\u0067ger":"manual"}`,
			strings.Replace(base, `"hook_event_name":"`+kind+`"`, `"hook_event_name":"TurnEnd"`, 1),
			strings.Replace(base, `"session_id":`, `"session_\u0069d":"00000000-0000-4000-8000-000000000002","session_id":`, 1),
			strings.Replace(base, "00000000-0000-4000-8000-000000000002", "msg$$child", 1),
			strings.Replace(base, `,"cwd":"/synthetic/ghp_SYNTHETIC_SECRET"`, "", 1),
			base + `{}`,
		}
		for _, trigger := range []string{"", "AUTO", "Manual", "unknown", "ghp_SYNTHETIC_SECRET", strings.Repeat("x", maxControl)} {
			bodies = append(bodies, compactBody(kind, trigger))
		}
		for key, value := range map[string]string{
			"tool_use_id": `"call-1"`, "tool_name": `"write"`, "tool_input": `{}`,
			"tool_response": `""`, "error": `""`, "last_assistant_message": `""`,
			"decision": `"denied"`, "source": `"startup"`, "reason": `"exit"`,
		} {
			bodies = append(bodies, strings.TrimSuffix(base, "}")+`,"`+key+`":`+value+"}")
		}
		for _, body := range bodies {
			got, err := Reduce([]byte(body))
			if !errors.Is(err, ErrPayload) || got != (capture.Metadata{}) || strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
				t.Fatalf("unsafe compaction admitted: %+v %v", got, err)
			}
		}
	}
	for _, body := range []string{attempt, resultBody(false), resultBody(true), permissionBody(""), permissionBody("denied"), turnEnd, turnBody("StopFailure", "", nil), turnBody("Interrupt", "", nil)} {
		if _, err := Reduce([]byte(strings.TrimSuffix(body, "}") + `,"trigger":"auto"}`)); !errors.Is(err, ErrPayload) {
			t.Fatal("compaction trigger admitted on another event family")
		}
	}
}

func FuzzCompactionFields(f *testing.F) {
	f.Add(byte(0), byte(0), "auto")
	f.Add(byte(1), byte(0), "manual")
	f.Add(byte(1), byte(1), "ghp_SYNTHETIC_SECRET")
	f.Fuzz(func(t *testing.T, event, extra byte, trigger string) {
		if len(trigger) > 4096 {
			t.Skip()
		}
		kind := []string{"PreCompact", "PostCompact", "TurnEnd", "PermissionRequest"}[event%4]
		body := compactBody(kind, trigger)
		if extra%2 == 1 {
			body = strings.TrimSuffix(body, "}") + `,"last_assistant_message":""}`
		}
		got, err := Reduce([]byte(body))
		allowed := event%4 < 2 && extra%2 == 0 && (trigger == "auto" || trigger == "manual")
		if (err == nil) != allowed || !allowed && got != (capture.Metadata{}) {
			t.Fatal("compaction source-shape expectation differs from admission")
		}
		if allowed && got.Outcome != "observed" {
			t.Fatal("compaction phase became a verified completion")
		}
	})
}
