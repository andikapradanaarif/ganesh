package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andikapradanaarif/ganesh/internal/s0"
)

// Six synthetic needs and six distinct synthetic follow-on task definitions.
// The initial all-unknown screen permits one later recheck; no actual inputs.
func cliRecheckDefinition() (s0.RecheckDefinition, s0.RecheckInput, string) {
	d := s0.RecheckDefinition{SchemaVersion: 1, RunID: "s0-00000000000000000000000000000046", Initial: s0.RecheckInventory{SchemaVersion: 1, State: "screened", Targets: []s0.RecheckTarget{}, Lessons: []s0.ScreenLesson{}}, Tasks: []s0.RecheckTask{}}
	in := s0.RecheckInput{SchemaVersion: 1, Final: s0.RecheckInventory{SchemaVersion: 1, State: "screened", Targets: []s0.RecheckTarget{}, Lessons: []s0.ScreenLesson{}}, Tasks: []s0.RecheckTaskEvidence{}}
	needs := s0.TargetInventory{SchemaVersion: 1}
	for i := range 6 {
		id := fmt.Sprintf("need-%d", i)
		target := s0.RecheckTarget{ID: id, Need: "synthetic private ghp_SYNTHETIC_SECRET need " + id, Assessed: true, Unknown: true, ReasonRefs: []string{"synthetic-origin"}, LessonIDs: []string{}}
		d.Initial.Targets = append(d.Initial.Targets, target)
		needs.Targets = append(needs.Targets, s0.ScreenTarget{NeedDefinition: s0.NeedDefinition{ID: id, Need: target.Need}})
		target.Unknown = false
		target.LessonIDs = []string{fmt.Sprintf("lesson-%d", i)}
		in.Final.Targets = append(in.Final.Targets, target)
		in.Final.Lessons = append(in.Final.Lessons, s0.ScreenLesson{ID: target.LessonIDs[0], Project: fmt.Sprintf("synthetic-project-%d", i%2), Derivation: "automatic", Origin: "first_party_only", Assurance: "server_observed", Eligibility: "eligible", EvidenceRefs: []string{"synthetic-provenance"}})
		stratum := "within"
		if i >= 3 {
			stratum = "cross"
		}
		taskID := fmt.Sprintf("task-%d", i)
		d.Tasks = append(d.Tasks, s0.RecheckTask{ID: taskID, Stratum: stratum, NeedIDs: []string{id}, Prompt: fmt.Sprintf("%064x", i+1), Rubric: strings.Repeat("c", 64), Sources: strings.Repeat("b", 64)})
		e := fmt.Sprintf("%064x", 100+i)
		in.Tasks = append(in.Tasks, s0.RecheckTaskEvidence{ID: taskID, Status: "supported", Evidence: &e})
	}
	n, _ := s0.FreezeNeeds(needs)
	pin, _ := n.Digest()
	return d, in, pin
}

func cliRecheckFreeze(t *testing.T, d s0.RecheckDefinition, needPin string) (s0.RecheckSnapshot, string, string, string) {
	t.Helper()
	dir := t.TempDir()
	definition, plan, input := filepath.Join(dir, "definition.json"), filepath.Join(dir, "snapshot.json"), filepath.Join(dir, "final.json")
	body, _ := json.Marshal(d)
	if os.WriteFile(definition, body, 0600) != nil {
		t.Fatal("write")
	}
	var output bytes.Buffer
	if err := run(context.Background(), []string{"s0", "freeze-recheck", "--input", definition, "--expected-need-digest", needPin, "--output", plan}, &output); err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	if json.Unmarshal(output.Bytes(), &report) != nil || report["state"] != "recheck_snapshot_frozen" {
		t.Fatal("missing freeze report")
	}
	assertRecheckAuthorityFalse(t, report)
	if bytes.Contains(output.Bytes(), []byte("SYNTHETIC_SECRET")) {
		t.Fatal("freeze echoed need prose")
	}
	s, err := s0.ReadRecheckSnapshot(plan)
	if err != nil || report["snapshot_digest"] != s.Digest || report["frozen_need_digest"] != needPin {
		t.Fatal("freeze pin mismatch")
	}
	// Independently hash the serialized snapshot bytes, without calling the
	// candidate's freeze/canonicalization code to obtain the expected digest.
	stored, err := os.ReadFile(plan)
	if err != nil {
		t.Fatal("read snapshot")
	}
	var compact bytes.Buffer
	if json.Compact(&compact, stored) != nil {
		t.Fatal("compact snapshot")
	}
	needle := []byte(`"snapshot_digest":"` + s.Digest + `"`)
	if bytes.Count(compact.Bytes(), needle) != 1 {
		t.Fatal("ambiguous digest field")
	}
	unsigned := bytes.Replace(compact.Bytes(), needle, []byte(`"snapshot_digest":""`), 1)
	if got := fmt.Sprintf("%x", sha256.Sum256(unsigned)); got != s.Digest {
		t.Fatal("independent serialized digest mismatch")
	}
	after, _ := os.ReadFile(definition)
	if !bytes.Equal(body, after) {
		t.Fatal("freeze changed input")
	}
	return s, plan, input, definition
}

func assertRecheckAuthorityFalse(t *testing.T, r map[string]any) {
	t.Helper()
	for _, key := range []string{"evidence_independently_verified", "experiment_accepted", "model_runs_authorized", "production_narrative_enabled"} {
		if v, ok := r[key]; !ok || v != false {
			t.Fatal("recheck granted authority")
		}
	}
}

func TestRecheckCLIFreezeRoutesAndImmutability(t *testing.T) {
	for _, name := range []string{"met", "missing_tasks", "initial_early_route", "initial_inconclusive"} {
		t.Run(name, func(t *testing.T) {
			d, in, pin := cliRecheckDefinition()
			route := "first_party_probe_candidate"
			switch name {
			case "missing_tasks":
				in.Tasks = []s0.RecheckTaskEvidence{}
				route = "a2_3_decision"
			case "initial_early_route":
				for i := range 3 {
					d.Initial.Targets[i].Mixed = true
				}
				route = "a2_3_decision"
			case "initial_inconclusive":
				for i := range d.Initial.Targets {
					d.Initial.Targets[i].Assessed = false
					d.Initial.Targets[i].Unknown = false
					d.Initial.Targets[i].ReasonRefs = []string{}
				}
				route = "inconclusive"
			}
			s, plan, input, definition := cliRecheckFreeze(t, d, pin)
			in.SnapshotDigest = s.Digest
			before, _ := os.ReadFile(plan)
			body, _ := json.Marshal(in)
			if os.WriteFile(input, body, 0600) != nil {
				t.Fatal("write")
			}
			var output bytes.Buffer
			err := run(context.Background(), []string{"s0", "recheck-eligibility", "--plan", plan, "--expected-digest", s.Digest, "--input", input}, &output)
			if name == "met" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err != s0.ErrRecheckIncomplete {
				t.Fatal("nonpassing declaration did not emit and exit incomplete")
			}
			var r map[string]any
			if json.Unmarshal(output.Bytes(), &r) != nil || r["route"] != route {
				t.Fatal("report route absent/wrong")
			}
			assertRecheckAuthorityFalse(t, r)
			if bytes.Contains(output.Bytes(), []byte("SYNTHETIC_SECRET")) {
				t.Fatal("report exposed need prose")
			}
			after, _ := os.ReadFile(input)
			if !bytes.Equal(body, after) {
				t.Fatal("final input mutated")
			}
			after, _ = os.ReadFile(plan)
			if !bytes.Equal(before, after) {
				t.Fatal("snapshot mutated")
			}
			output.Reset()
			if err := run(context.Background(), []string{"s0", "freeze-recheck", "--input", definition, "--expected-need-digest", pin, "--output", plan}, &output); err != s0.ErrRecheck || output.Len() != 0 {
				t.Fatal("freeze overwrote or reported uncommitted pin")
			}
		})
	}
}

func TestRecheckCLIExternalPinBeforeFinalRead(t *testing.T) {
	d, in, pin := cliRecheckDefinition()
	s, plan, input, definition := cliRecheckFreeze(t, d, pin)
	var output bytes.Buffer
	args := []string{"s0", "recheck-eligibility", "--plan", plan, "--expected-digest", strings.Repeat("f", 64), "--input", input}
	if err := run(context.Background(), args, &output); err != s0.ErrRecheck || output.Len() != 0 {
		t.Fatal("final metadata read before external pin")
	}
	args[5] = s.Digest
	if err := run(context.Background(), args, &output); err != s0.ErrRecheckInput || output.Len() != 0 {
		t.Fatal("bad-pin probe did not distinguish final-read failure")
	}
	// A replacement can be internally valid but must fail against the old pin.
	d.RunID = "s0-00000000000000000000000000000047"
	body, _ := json.Marshal(d)
	if os.WriteFile(definition, body, 0600) != nil {
		t.Fatal("write")
	}
	replacement := plan + "-replacement"
	if err := run(context.Background(), []string{"s0", "freeze-recheck", "--input", definition, "--expected-need-digest", pin, "--output", replacement}, &output); err != nil {
		t.Fatal(err)
	}
	in.SnapshotDigest = s.Digest
	body, _ = json.Marshal(in)
	if os.WriteFile(input, body, 0600) != nil {
		t.Fatal("write")
	}
	output.Reset()
	args[3] = replacement
	if err := run(context.Background(), args, &output); err != s0.ErrRecheck || output.Len() != 0 {
		t.Fatal("self-consistent replacement escaped retained pin")
	}
}

func TestRecheckCLIRejectsMalformedMetadata(t *testing.T) {
	d, in, pin := cliRecheckDefinition()
	s, plan, input, _ := cliRecheckFreeze(t, d, pin)
	in.SnapshotDigest = s.Digest
	good, _ := json.Marshal(in)
	for _, name := range []string{"unknown", "duplicate", "case_alias", "missing", "null", "wrong_need", "unsupported_claim", "empty_pin"} {
		t.Run(name, func(t *testing.T) {
			body := string(good)
			expected := s.Digest
			switch name {
			case "unknown":
				body = strings.Replace(body, "{", `{"raw":"ghp_SYNTHETIC_SECRET",`, 1)
			case "duplicate":
				body = strings.Replace(body, "{", `{"schema_version":1,`, 1)
			case "case_alias":
				body = strings.Replace(body, `"final_inventory"`, `"Final_Inventory"`, 1)
			case "missing":
				body = strings.Replace(body, `"schema_version":1,`, "", 1)
			case "null":
				v := in
				v.Tasks = nil
				b, _ := json.Marshal(v)
				body = string(b)
			case "wrong_need":
				body = strings.Replace(body, "synthetic private ghp_SYNTHETIC_SECRET need need-0", "changed", 1)
			case "unsupported_claim":
				body = strings.Replace(body, `"applicability":"supported"`, `"applicability":"ghp_SYNTHETIC_SECRET"`, 1)
			case "empty_pin":
				expected = ""
			}
			if os.WriteFile(input, []byte(body), 0600) != nil {
				t.Fatal("write")
			}
			var output bytes.Buffer
			err := run(context.Background(), []string{"s0", "recheck-eligibility", "--plan", plan, "--expected-digest", expected, "--input", input}, &output)
			if err == nil || err == s0.ErrRecheckIncomplete || output.Len() != 0 || strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
				t.Fatal("bad metadata accepted/echoed")
			}
		})
	}
}
