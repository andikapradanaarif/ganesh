package s0

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func recheckHash(label string) string {
	h := sha256.Sum256([]byte("synthetic-recheck/" + label))
	return hex.EncodeToString(h[:])
}

// Twelve frozen needs: initially M=2/U=10/E=0. Later six separate lessons
// cover six needs, on two projects. Independent expected gate: 6/12, 6, 2, 3+3.
func recheckFixture() (RecheckDefinition, RecheckInput) {
	d := RecheckDefinition{SchemaVersion: 1, RunID: "s0-00000000000000000000000000000001", Initial: RecheckInventory{SchemaVersion: 1, State: "screened", Targets: []RecheckTarget{}, Lessons: []ScreenLesson{}}, Tasks: []RecheckTask{}}
	in := RecheckInput{SchemaVersion: 1, Final: RecheckInventory{SchemaVersion: 1, State: "screened", Targets: []RecheckTarget{}, Lessons: []ScreenLesson{}}, Tasks: []RecheckTaskEvidence{}}
	for i := range 12 {
		target := RecheckTarget{ID: fmt.Sprintf("N%d", i), Need: fmt.Sprintf("synthetic need %d", i), Assessed: true, Mixed: i < 2, Unknown: true, ReasonRefs: []string{"synthetic-origin"}, LessonIDs: []string{}}
		d.Initial.Targets = append(d.Initial.Targets, target)
		target.Mixed = false
		if i < 6 {
			target.Unknown = false
			target.LessonIDs = []string{fmt.Sprintf("L%d", i)}
			in.Final.Lessons = append(in.Final.Lessons, ScreenLesson{ID: fmt.Sprintf("L%d", i), Project: fmt.Sprintf("synthetic-project-%d", i%2), Derivation: "automatic", Origin: "first_party_only", Assurance: "client_attested", Eligibility: "eligible", EvidenceRefs: []string{"synthetic-reconciled-provenance"}})
			stratum := "within"
			if i >= 3 {
				stratum = "cross"
			}
			id := fmt.Sprintf("task-%d", i)
			d.Tasks = append(d.Tasks, RecheckTask{id, stratum, []string{target.ID}, recheckHash("prompt/" + id), recheckHash("rubric/" + id), recheckHash("sources/" + id)})
			e := recheckHash("applicability/" + id)
			in.Tasks = append(in.Tasks, RecheckTaskEvidence{id, "supported", &e})
		}
		in.Final.Targets = append(in.Final.Targets, target)
	}
	return d, in
}

func freezeRecheckFixture(t *testing.T, d RecheckDefinition) RecheckSnapshot {
	t.Helper()
	n, err := FreezeNeeds(d.Initial.screenInventory())
	if err != nil {
		t.Fatal(err)
	}
	pin, err := n.Digest()
	if err != nil {
		t.Fatal(err)
	}
	s, err := FreezeRecheck(d, pin)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func reportRecheckFixture(t *testing.T, d RecheckDefinition, in RecheckInput) RecheckReport {
	t.Helper()
	s := freezeRecheckFixture(t, d)
	in.SnapshotDigest = s.Digest
	r, err := ReportRecheck(s, s.Digest, in)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRecheckExactGateAndPrivacy(t *testing.T) {
	d, in := recheckFixture()
	for i := range d.Initial.Targets {
		d.Initial.Targets[i].Need += " ghp_SYNTHETIC_SECRET"
		in.Final.Targets[i].Need = d.Initial.Targets[i].Need
	}
	in.Final.Targets[0].ReasonRefs = []string{"ghp_SYNTHETIC_SECRET"}
	in.Final.Lessons[0].EvidenceRefs = []string{"ghp_SYNTHETIC_SECRET"}
	r := reportRecheckFixture(t, d, in)
	if r.State != "declared_criteria_met" || r.Route != "first_party_probe_candidate" || r.InitialRoute != "single_recheck_pending" || r.Initial.Counts != (OriginCounts{H: 12, M: 2, U: 10}) || r.Final.Counts != (OriginCounts{H: 12, U: 6, E: 6}) || r.Final.Lessons != 6 || r.Final.Projects != 2 || r.Within != 3 || r.Cross != 3 || !r.SixTasks || !r.SixLessons || !r.TwoProjects || !r.HalfNeeds {
		t.Fatalf("incorrect independent gate counts: %+v", r)
	}
	if len(r.Changes) != 6 || len(r.Tasks) != 6 || len(r.Reasons) != 0 {
		t.Fatal("changes/tasks lost")
	}
	for _, change := range r.Changes {
		if !change.Before.Unknown || change.Before.Eligible || change.After.Unknown || !change.After.Eligible {
			t.Fatal("classification transition lost")
		}
	}
	body, _ := json.Marshal(r)
	if strings.Contains(string(body), "SYNTHETIC_SECRET") {
		t.Fatal("report exposed private prose/reference")
	}
	var fields map[string]any
	_ = json.Unmarshal(body, &fields)
	for _, key := range []string{"evidence_independently_verified", "experiment_accepted", "model_runs_authorized", "production_narrative_enabled"} {
		if v, ok := fields[key]; !ok || v != false {
			t.Fatal("report granted authority")
		}
	}
}

func TestRecheckClassificationsByID(t *testing.T) {
	targets := []RecheckTarget{
		{ID: "need-b", Assessed: true, OwnerOnly: true, LessonIDs: []string{"lesson-b"}},
		{ID: "need-a", Assessed: true, LessonIDs: []string{"lesson-a"}},
	}
	assessments := []TargetAssessment{
		{ID: "need-a", Eligible: true},
		{ID: "need-b", Mixed: true, Unknown: true},
	}
	want := map[string]RecheckClassification{
		"need-a": {Assessed: true, Eligible: true, Lessons: []string{"lesson-a"}},
		"need-b": {Assessed: true, Mixed: true, Unknown: true, OwnerOnly: true, Lessons: []string{"lesson-b"}},
	}
	for range 2 {
		got, err := recheckClassifications(targets, assessments)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("assessment attributed to wrong need: %+v, %v", got, err)
		}
		slices.Reverse(assessments)
	}
	for _, name := range []string{"missing", "extra", "foreign", "case_changed", "duplicate_assessment", "duplicate_target"} {
		t.Run(name, func(t *testing.T) {
			targets, assessments := slices.Clone(targets), slices.Clone(assessments)
			switch name {
			case "missing":
				assessments = assessments[:1]
			case "extra":
				assessments = append(assessments, TargetAssessment{ID: "extra"})
			case "foreign":
				assessments[0].ID = "foreign"
			case "case_changed":
				assessments[0].ID = "Need-a"
			case "duplicate_assessment":
				assessments[1].ID = assessments[0].ID
			case "duplicate_target":
				targets[1].ID = targets[0].ID
			}
			if got, err := recheckClassifications(targets, assessments); err != ErrRecheck || got != nil {
				t.Fatal("mismatched identities produced classifications")
			}
		})
	}
}

func TestRecheckReferenceOnlyChangesRemainDigestBound(t *testing.T) {
	d, in := recheckFixture()
	d.Initial = in.Final
	before := reportRecheckFixture(t, d, in)
	if len(before.Changes) != 0 || before.State != "declared_criteria_met" {
		t.Fatal("fixture must start with identical eligible inventories")
	}
	// The fixture inventories share slices; replace them before changing final
	// references so the frozen initial declarations remain intact.
	in.Final.Targets = slices.Clone(in.Final.Targets)
	in.Final.Lessons = slices.Clone(in.Final.Lessons)
	in.Final.Targets[0].ReasonRefs = []string{"synthetic-new-reason"}
	in.Final.Lessons[0].EvidenceRefs = []string{"synthetic-new-evidence"}
	after := reportRecheckFixture(t, d, in)
	if len(after.Changes) != 0 || after.SnapshotDigest != before.SnapshotDigest || after.InputDigest == before.InputDigest || !reflect.DeepEqual(after.Final, before.Final) || after.State != "declared_criteria_met" {
		t.Fatal("reference-only edit changed classification or escaped digest binding")
	}
}

func TestRecheckDistinctGateFailures(t *testing.T) {
	for _, name := range []string{"five_lessons", "one_project", "below_half", "one_missing_task", "unknown_applicability", "unsupported_applicability", "one_lesson_six_needs", "six_lessons_one_need", "partial_need", "mixed", "unknown", "owner_only", "unassessed", "owner_lesson", "synthetic_lesson"} {
		t.Run(name, func(t *testing.T) {
			d, in := recheckFixture()
			switch name {
			case "five_lessons":
				in.Final.Targets[5].LessonIDs = []string{"L4"}
			case "one_project":
				for i := range in.Final.Lessons {
					in.Final.Lessons[i].Project = "synthetic-one-project"
				}
			case "below_half":
				a := RecheckTarget{ID: "extra", Need: "synthetic extra need", Assessed: true, Unknown: true, ReasonRefs: []string{"synthetic-origin"}, LessonIDs: []string{}}
				d.Initial.Targets = append(d.Initial.Targets, a)
				in.Final.Targets = append(in.Final.Targets, a)
			case "one_missing_task":
				in.Tasks = in.Tasks[:5]
			case "unknown_applicability":
				in.Tasks[0].Status = "unknown"
				in.Tasks[0].Evidence = nil
			case "unsupported_applicability":
				in.Tasks[0].Status = "unsupported"
			case "one_lesson_six_needs":
				for i := range 6 {
					in.Final.Targets[i].LessonIDs = []string{"L0"}
				}
			case "six_lessons_one_need":
				for i := range 6 {
					in.Final.Targets[i].Unknown = i > 0
					in.Final.Targets[i].LessonIDs = []string{}
					d.Tasks[i].NeedIDs = []string{"N0"}
					in.Final.Targets[0].LessonIDs = append(in.Final.Targets[0].LessonIDs, fmt.Sprintf("L%d", i))
				}
			case "partial_need":
				in.Final.Lessons[1].Eligibility = "ineligible"
				in.Final.Targets[0].LessonIDs = []string{"L0", "L1"}
			case "mixed":
				in.Final.Targets[0].Mixed = true
			case "unknown":
				in.Final.Targets[0].Unknown = true
			case "owner_only":
				in.Final.Targets[0].OwnerOnly = true
			case "unassessed":
				in.Final.Targets[0].Assessed = false
			case "owner_lesson", "synthetic_lesson":
				in.Final.Lessons[0].Eligibility = "ineligible"
				in.Final.Lessons[0].Derivation = strings.TrimSuffix(name, "_lesson")
			}
			r := reportRecheckFixture(t, d, in)
			if r.State != "declared_gate_not_met" || r.Route != "a2_3_decision" || len(r.Reasons) == 0 {
				t.Fatalf("failed requirement did not route: %+v", r)
			}
			switch name {
			case "five_lessons":
				if r.Final.Lessons != 5 || r.Final.Counts.E != 6 || !r.SixTasks {
					t.Fatal("unused lesson inflated eligible count")
				}
			case "below_half":
				if r.Final.Counts.H != 13 || r.Final.Counts.E != 6 || r.HalfNeeds || !r.SixTasks {
					t.Fatal("incorrect half threshold")
				}
			case "one_lesson_six_needs":
				if r.Final.Lessons != 1 || r.Final.Projects != 1 || r.Final.Counts.E != 6 {
					t.Fatal("lessons/targets conflated")
				}
			case "six_lessons_one_need":
				if r.Final.Lessons != 6 || r.Final.Counts.E != 1 || !r.SixTasks {
					t.Fatal("target/lesson counts conflated")
				}
			case "partial_need":
				if r.Final.Counts.E != 4 || r.Final.Lessons != 4 {
					t.Fatal("partial coverage counted")
				}
			}
		})
	}
}

func TestRecheckInitialRoutingCannotBeLaundered(t *testing.T) {
	for _, name := range []string{"early_half", "early_above_half", "initial_inconclusive", "final_inconclusive", "final_mixed_threshold"} {
		t.Run(name, func(t *testing.T) {
			d, in := recheckFixture()
			want := "a2_3_decision"
			switch name {
			case "early_half", "early_above_half":
				n := 6
				if name == "early_above_half" {
					n = 7
				}
				for i := range d.Initial.Targets {
					d.Initial.Targets[i].Mixed = i < n
				}
			case "initial_inconclusive":
				for i := range d.Initial.Targets {
					d.Initial.Targets[i].Assessed = false
					d.Initial.Targets[i].Mixed = false
					d.Initial.Targets[i].Unknown = false
					d.Initial.Targets[i].ReasonRefs = []string{}
				}
				want = "inconclusive"
			case "final_inconclusive":
				for i := range in.Final.Targets {
					in.Final.Targets[i].Assessed = false
					in.Final.Targets[i].Mixed = false
					in.Final.Targets[i].Unknown = false
					in.Final.Targets[i].ReasonRefs = []string{}
					in.Final.Targets[i].LessonIDs = []string{}
				}
				in.Final.Lessons = []ScreenLesson{}
				want = "inconclusive"
			case "final_mixed_threshold":
				for i := range 6 {
					in.Final.Targets[i+6].Mixed = true
				}
			}
			r := reportRecheckFixture(t, d, in)
			if r.Route != want {
				t.Fatalf("route: %s want %s", r.Route, want)
			}
			if name == "final_inconclusive" && r.InitialRoute != "single_recheck_pending" {
				t.Fatal("fixture lost assessable initial screen")
			}
			if strings.HasPrefix(name, "early_") && (!r.SixLessons || !r.TwoProjects || !r.HalfNeeds || !r.SixTasks) {
				t.Fatal("fixture did not isolate locked initial route")
			}
		})
	}
}

func TestRecheckFreezeAndInputValidation(t *testing.T) {
	baseline, _ := recheckFixture()
	needs, _ := FreezeNeeds(baseline.Initial.screenInventory())
	needPin, _ := needs.Digest()
	for _, name := range []string{"few_tasks", "wrong_stratum", "unknown_need", "duplicate_task", "renamed_task", "missing_hash", "duplicate_need", "empty_needs", "bad_label", "duplicate_lesson", "forged_owner_eligibility"} {
		t.Run(name, func(t *testing.T) {
			d, _ := recheckFixture()
			switch name {
			case "few_tasks":
				d.Tasks = d.Tasks[:5]
			case "wrong_stratum":
				d.Tasks[0].Stratum = "cross"
			case "unknown_need":
				d.Tasks[0].NeedIDs = []string{"absent"}
			case "duplicate_task":
				d.Tasks[1].ID = d.Tasks[0].ID
			case "renamed_task":
				d.Tasks[1].Prompt = d.Tasks[0].Prompt
				d.Tasks[1].Sources = d.Tasks[0].Sources
			case "missing_hash":
				d.Tasks[0].Rubric = ""
			case "duplicate_need":
				d.Initial.Targets[1].ID = d.Initial.Targets[0].ID
			case "empty_needs":
				d.Initial.Targets = []RecheckTarget{}
			case "bad_label":
				d.Tasks[0].ID = "ghp_SYNTHETIC_SECRET/prose"
			case "duplicate_lesson":
				l := ScreenLesson{ID: "L", Project: "synthetic", Derivation: "automatic", Origin: "unknown", Assurance: "client_attested", Eligibility: "unknown", EvidenceRefs: []string{}}
				d.Initial.Lessons = []ScreenLesson{l, l}
			case "forged_owner_eligibility":
				d.Initial.Lessons = []ScreenLesson{{ID: "L", Project: "synthetic", Derivation: "owner", Origin: "first_party_only", Assurance: "client_attested", Eligibility: "eligible", EvidenceRefs: []string{"synthetic"}}}
			}
			if _, err := FreezeRecheck(d, needPin); err != ErrRecheck || strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
				t.Fatal("bad definition admitted/echoed")
			}
		})
	}
	d, in := recheckFixture()
	s := freezeRecheckFixture(t, d)
	in.SnapshotDigest = s.Digest
	for _, name := range []string{"changed_need", "added_need", "removed_need", "re_cased_need", "wrong_snapshot", "duplicate_task", "unknown_task", "missing_evidence", "reused_evidence", "bad_evidence", "forged_synthetic_eligibility"} {
		t.Run(name, func(t *testing.T) {
			body, _ := json.Marshal(in)
			var v RecheckInput
			_ = json.Unmarshal(body, &v)
			switch name {
			case "changed_need":
				v.Final.Targets[0].Need = "changed"
			case "added_need":
				v.Final.Targets = append(v.Final.Targets, RecheckTarget{ID: "extra", Need: "changed"})
			case "removed_need":
				v.Final.Targets = v.Final.Targets[:11]
			case "re_cased_need":
				v.Final.Targets[0].ID = "n0"
			case "wrong_snapshot":
				v.SnapshotDigest = strings.Repeat("a", 64)
			case "duplicate_task":
				v.Tasks[1].ID = v.Tasks[0].ID
			case "unknown_task":
				v.Tasks[0].ID = "absent"
			case "missing_evidence":
				v.Tasks[0].Evidence = nil
			case "reused_evidence":
				v.Tasks[1].Evidence = v.Tasks[0].Evidence
			case "bad_evidence":
				x := "ghp_SYNTHETIC_SECRET"
				v.Tasks[0].Evidence = &x
			case "forged_synthetic_eligibility":
				v.Final.Lessons[0].Derivation = "synthetic"
			}
			if _, err := ReportRecheck(s, s.Digest, v); err != ErrRecheck || strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
				t.Fatal("bad input admitted/echoed")
			}
		})
	}
}

func TestRecheckCanonicalIdentityAndExternalPin(t *testing.T) {
	d, in := recheckFixture()
	s := freezeRecheckFixture(t, d)
	// Canonicalization must not mutate any caller arrays or pointed evidence.
	db, _ := json.Marshal(d)
	ib, _ := json.Marshal(in)
	r := reportRecheckFixture(t, d, in)
	if after, _ := json.Marshal(d); !bytes.Equal(db, after) {
		t.Fatal("definition mutated")
	}
	if after, _ := json.Marshal(in); !bytes.Equal(ib, after) {
		t.Fatal("input mutated")
	}
	slices.Reverse(d.Initial.Targets)
	slices.Reverse(d.Tasks)
	s2 := freezeRecheckFixture(t, d)
	if s.Digest != s2.Digest {
		t.Fatal("definition order changed pin")
	}
	slices.Reverse(in.Final.Targets)
	slices.Reverse(in.Final.Lessons)
	slices.Reverse(in.Tasks)
	r2 := reportRecheckFixture(t, d, in)
	if !reflect.DeepEqual(r, r2) {
		t.Fatal("input order changed identity/output")
	}
	d.RunID = "s0-00000000000000000000000000000002"
	replacement := freezeRecheckFixture(t, d)
	if CheckRecheckAgainst(replacement, replacement.Digest) != nil || CheckRecheckAgainst(replacement, s.Digest) != ErrRecheck {
		t.Fatal("self-consistent replacement escaped external pin")
	}
	s.Algorithm = "other"
	if CheckRecheckAgainst(s, s.Digest) != ErrRecheck {
		t.Fatal("snapshot shape tamper admitted")
	}
}

func TestRecheckPrivateSnapshotAndStrictReader(t *testing.T) {
	d, in := recheckFixture()
	s := freezeRecheckFixture(t, d)
	path := filepath.Join(t.TempDir(), "snapshot.json")
	if SaveRecheckSnapshot(path, s) != nil {
		t.Fatal("save failed")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("not private")
	}
	if SaveRecheckSnapshot(path, s) != ErrRecheck {
		t.Fatal("snapshot overwritten")
	}
	if got, err := ReadRecheckSnapshot(path); err != nil || CheckRecheckAgainst(got, s.Digest) != nil {
		t.Fatal("snapshot read/pin")
	}
	in.SnapshotDigest = s.Digest
	good, _ := json.Marshal(in)
	for _, name := range []string{"unknown", "duplicate", "case_alias", "missing_key", "null_slice", "bad_utf8", "oversize", "mode", "symlink"} {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "input.json")
			body := good
			switch name {
			case "unknown":
				body = []byte(strings.Replace(string(good), "{", `{"raw":"ghp_SYNTHETIC_SECRET",`, 1))
			case "duplicate":
				body = []byte(strings.Replace(string(good), "{", `{"schema_version":1,`, 1))
			case "case_alias":
				body = []byte(strings.Replace(string(good), `"task_evidence"`, `"Task_Evidence"`, 1))
			case "missing_key":
				body = []byte(strings.Replace(string(good), `"schema_version":1,`, "", 1))
			case "null_slice":
				body = []byte(strings.Replace(string(good), `"reason_refs":["synthetic-origin"]`, `"reason_refs":null`, 1))
			case "bad_utf8":
				body = []byte{0xff}
			case "oversize":
				body = []byte(strings.Repeat("x", (1<<20)+1))
			}
			if os.WriteFile(file, body, 0600) != nil {
				t.Fatal("write")
			}
			if name == "mode" {
				_ = os.Chmod(file, 0644)
			}
			if name == "symlink" {
				link := file + "-link"
				if os.Symlink(file, link) != nil {
					t.Fatal("symlink")
				}
				file = link
			}
			if _, err := ReadRecheckInput(file); err != ErrRecheckInput || strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
				t.Fatal("unsafe input admitted/echoed")
			}
		})
	}
}
