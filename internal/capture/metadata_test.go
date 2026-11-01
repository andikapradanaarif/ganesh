package capture

import (
	"encoding/json"
	"strings"
	"testing"
)

func fixtureMetadata() Metadata {
	return Metadata{1, "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", "tool_result", "failed"}
}

func TestReducedMetadataAdmission(t *testing.T) {
	valid, err := json.Marshal(fixtureMetadata())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Decode(valid); err != nil || got != fixtureMetadata() {
		t.Fatalf("admission: %+v %v", got, err)
	}
	for _, body := range []string{
		`{"raw_tool_output":"ghp_SYNTHETIC_SECRET"}`,
		strings.Replace(string(valid), `"event_id":"00000000-0000-4000-8000-000000000001"`, `"event_id":"sk-SYNTHETIC_SECRET"`, 1),
		strings.Replace(string(valid), `"failed"`, `"ghp_SYNTHETIC_SECRET"`, 1),
		strings.TrimSuffix(string(valid), "}") + `,"raw":"ghp_SYNTHETIC_SECRET"}`,
		strings.TrimSuffix(string(valid), "}") + `,"event":"tool_result"}`,
		strings.TrimSuffix(string(valid), "}") + `,"schema_version":null}`,
		string(valid) + ` {"raw":"ghp_SYNTHETIC_SECRET"}`,
		`null`, `[]`, `{"schema_version":1}`, strings.Repeat(" ", MaxInput+1),
	} {
		if _, err := Decode([]byte(body)); err == nil || strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
			t.Fatal("unsafe admission/error")
		}
	}
	if _, err := Decode(append([]byte{'{'}, 0xff)); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func TestPermissionMetadataPairs(t *testing.T) {
	for _, event := range []string{"permission_request", "permission_result", "tool_attempt", "tool_result"} {
		for _, outcome := range []string{"observed", "granted", "denied", "cancelled", "failed", "succeeded", "unknown"} {
			m := fixtureMetadata()
			m.Event, m.Outcome = event, outcome
			want := false
			switch event {
			case "permission_request":
				want = outcome == "observed"
			case "permission_result":
				want = outcome == "granted" || outcome == "denied" || outcome == "cancelled"
			default:
				// Preserve the existing v1 enum grammar for old event types;
				// granted/cancelled are admitted only as permission results.
				want = outcome != "granted" && outcome != "cancelled"
			}
			body, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Decode(body)
			if (err == nil) != want || want && got != m {
				t.Fatalf("metadata pair %s/%s admitted=%v want=%v", event, outcome, err == nil, want)
			}
		}
	}
}
