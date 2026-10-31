package s0

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// These independently stated expected counts use synthetic screener records,
// not output from an origin detector or the implementation under evaluation.
func screenFixture() TargetInventory {
	in := TargetInventory{SchemaVersion: 1, State: "screened"}
	for _, id := range []string{"T0", "T1", "T2", "T3", "T4", "T5", "T6", "T7"} {
		in.Targets = append(in.Targets, ScreenTarget{NeedDefinition: NeedDefinition{id, "synthetic need " + id}, Assessed: true, ReasonRefs: []string{"fixture-origin-evidence"}})
	}
	in.Targets[0].Mixed = true
	in.Targets[0].Unknown = true
	in.Targets[0].LessonIDs = []string{"L1"}
	in.Targets[1].Mixed = true
	in.Targets[2].Unknown = true
	in.Targets[3].Assessed = false
	in.Targets[3].ReasonRefs = nil
	in.Targets[4].LessonIDs = []string{"L1", "L2"}
	in.Targets[5].LessonIDs = []string{"L1"}
	in.Targets[6].OwnerOnly = true
	for _, l := range []struct{ id, project string }{{"L1", "fixture-project-one"}, {"L2", "fixture-project-two"}, {"unused", "fixture-project-three"}} {
		in.Lessons = append(in.Lessons, ScreenLesson{ID: l.id, Project: l.project, Derivation: "automatic", Origin: "first_party_only", Assurance: "client_attested", Eligibility: "eligible", EvidenceRefs: []string{"fixture-closure-and-provenance-evidence"}})
	}
	return in
}

func screenWithFrozenNeeds(t *testing.T, in TargetInventory) ScreenReport {
	t.Helper()
	baseline, err := FreezeNeeds(in)
	if err != nil {
		t.Fatal(err)
	}
	r, err := InitialScreen(in, &baseline)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestInitialScreenCountsPartitionCoverageAndNeededLessons(t *testing.T) {
	r := screenWithFrozenNeeds(t, screenFixture())
	if r.Counts != (OriginCounts{H: 8, M: 2, U: 2, E: 2}) {
		t.Fatalf("counts: %+v", r.Counts)
	}
	if *r.Ratios != (OriginRatios{Mixed: .25, Unknown: .25, MixedUnknown: .5, Eligible: .25}) {
		t.Fatalf("ratios: %+v", r.Ratios)
	}
	if r.Route != "single_recheck_pending" || r.EligibleLessons != 2 || r.EligibleProjects != 2 {
		t.Fatalf("route/coverage: %+v", r)
	}
	if !r.Assessments[0].Mixed || !r.Assessments[0].Unknown || r.Assessments[0].Eligible {
		t.Fatal("mixed/unknown reasons lost or mixed target counted eligible")
	}
	if r.Assessments[6].Eligible {
		t.Fatal("owner-only need counted automatic")
	}
	if r.EvidenceIndependentlyVerified || r.ProductionNarrativeEnabled || r.ModelRunsAuthorized {
		t.Fatal("accounting created authority")
	}
}

func TestEarlyRoutingUsesMixedThresholdOnly(t *testing.T) {
	for _, tc := range []struct {
		name           string
		mixed, unknown int
		route          string
	}{
		{"exact half", 2, 2, "a2_3_decision"},
		{"high unknown does not add threshold", 1, 3, "single_recheck_pending"},
		{"all unknown assessed", 0, 4, "single_recheck_pending"},
		{"above half", 3, 1, "a2_3_decision"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := screenFixture()
			in.Targets = in.Targets[:4]
			in.Lessons = nil
			for i := range in.Targets {
				in.Targets[i].Assessed = true
				in.Targets[i].Mixed = i < tc.mixed
				in.Targets[i].Unknown = i >= tc.mixed && i < tc.mixed+tc.unknown
				in.Targets[i].LessonIDs = nil
				in.Targets[i].ReasonRefs = []string{"fixture-assessment"}
			}
			r := screenWithFrozenNeeds(t, in)
			if r.Route != tc.route || r.Counts.M != tc.mixed || r.Counts.U != tc.unknown {
				t.Fatalf("wrong route/counts: %+v", r)
			}
		})
	}
}

func TestEmptyAndUnassessedAreInconclusive(t *testing.T) {
	var legacy TargetInventory
	if err := json.Unmarshal([]byte(`{"schema_version":1,"state":"not_screened","targets":[]}`), &legacy); err != nil {
		t.Fatal(err)
	}
	r, err := InitialScreen(legacy, nil)
	if err != nil || r.Route != "inconclusive" || r.Ratios != nil || r.Counts.H != 0 {
		t.Fatalf("legacy empty: %+v %v", r, err)
	}
	legacy.Targets = []ScreenTarget{{NeedDefinition: NeedDefinition{"pending", "synthetic pending need"}}}
	r = screenWithFrozenNeeds(t, legacy)
	if r.Route != "inconclusive" || r.Counts.U != 1 || r.Ratios.Unknown != 1 {
		t.Fatalf("unassessed treated clean: %+v", r)
	}
	if _, err := FreezeNeeds(NewTargetInventory()); err == nil {
		t.Fatal("froze empty inventory")
	}
}

func TestExplicitAssessmentFlagsDoNotRequireRedundantAssessed(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		mixed, unknown, ownerOnly bool
		want                      OriginCounts
		route                     string
	}{
		{"transcripts unavailable", false, true, false, OriginCounts{H: 2, U: 2}, "single_recheck_pending"},
		{"owner-only needs", false, false, true, OriginCounts{H: 2, U: 2}, "single_recheck_pending"},
		{"known mixed with incomplete coverage", true, false, false, OriginCounts{H: 2, M: 2}, "a2_3_decision"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := screenFixture()
			in.Targets = in.Targets[4:6]
			for i := range in.Targets {
				in.Targets[i].Assessed = false
				in.Targets[i].Mixed = tc.mixed
				in.Targets[i].Unknown = tc.unknown
				in.Targets[i].OwnerOnly = tc.ownerOnly
				in.Targets[i].ReasonRefs = []string{"fixture-explicit-assessment-evidence"}
			}
			// Both needs reference eligible lessons. An assessment declaration
			// can establish a route without clearing unknown coverage or E.
			r := screenWithFrozenNeeds(t, in)
			if r.Counts != tc.want || r.Route != tc.route {
				t.Fatalf("explicit declarations ignored: %+v", r)
			}
			for _, a := range r.Assessments {
				if !a.Unknown || a.Eligible {
					t.Fatalf("flag-only assessment upgraded coverage: %+v", a)
				}
			}
			if r.EligibleLessons != 0 || r.EvidenceIndependentlyVerified || r.ProductionNarrativeEnabled || r.ModelRunsAuthorized {
				t.Fatal("assessment flag granted trust or eligibility")
			}
			withoutRefs := in
			withoutRefs.Targets = append([]ScreenTarget(nil), in.Targets...)
			withoutRefs.Targets[0].ReasonRefs = nil
			baseline, err := FreezeNeeds(withoutRefs)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := InitialScreen(withoutRefs, &baseline); err == nil {
				t.Fatal("flag-only assessment accepted without evidence references")
			}
			in.State = "not_screened"
			baseline, err = FreezeNeeds(in)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := InitialScreen(in, &baseline); err == nil {
				t.Fatal("unscreened inventory accepted explicit assessments")
			}
		})
	}
}

func TestNeedSnapshotIsCanonicalCaseSensitiveAndNonOverwriting(t *testing.T) {
	in := TargetInventory{SchemaVersion: 1, Targets: []ScreenTarget{
		{NeedDefinition: NeedDefinition{"NeedB", "Why Y?"}},
		{NeedDefinition: NeedDefinition{"NeedA", "Why X?"}},
	}}
	snapshot, err := FreezeNeeds(in)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snapshot.Targets, []NeedDefinition{{"NeedA", "Why X?"}, {"NeedB", "Why Y?"}}) {
		t.Fatalf("changed definitions: %+v", snapshot)
	}
	// Independent fixed serialized definition, not another call to the encoder
	// or canonicalizer used by the candidate implementation.
	want := sha256.Sum256([]byte(`{"schema_version":1,"targets":[{"id":"NeedA","need":"Why X?"},{"id":"NeedB","need":"Why Y?"}]}`))
	digest, err := snapshot.Digest()
	if err != nil || digest != hex.EncodeToString(want[:]) {
		t.Fatalf("digest: %s %v", digest, err)
	}
	path := filepath.Join(t.TempDir(), "needs.json")
	if err := SaveNeedSnapshot(path, snapshot); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if err := SaveNeedSnapshot(path, snapshot); err == nil {
		t.Fatal("overwrote frozen needs")
	}
	after, _ := os.ReadFile(path)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("snapshot changed after failed overwrite")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm()&0077 != 0 {
		t.Fatal("snapshot is not private")
	}
	in.State = "screened"
	in.Targets[0].ID = "needb"
	if _, err := InitialScreen(in, &snapshot); err == nil {
		t.Fatal("case change bypassed frozen denominator")
	}
}

func TestChangedDefinitionCannotChangeDenominator(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*TargetInventory)
	}{
		{"need rewritten", func(in *TargetInventory) { in.Targets[0].Need += " changed" }},
		{"need removed", func(in *TargetInventory) { in.Targets = in.Targets[1:] }},
		{"need added", func(in *TargetInventory) {
			in.Targets = append(in.Targets, ScreenTarget{NeedDefinition: NeedDefinition{"new", "new synthetic need"}})
		}},
		{"all removed", func(in *TargetInventory) { in.Targets = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := screenFixture()
			base, _ := FreezeNeeds(in)
			tc.change(&in)
			if _, err := InitialScreen(in, &base); err == nil {
				t.Fatal("changed frozen need inventory accepted")
			}
		})
	}
	in := screenFixture()
	baseline, _ := FreezeNeeds(in)
	for left, right := 0, len(in.Targets)-1; left < right; left, right = left+1, right-1 {
		in.Targets[left], in.Targets[right] = in.Targets[right], in.Targets[left]
	}
	if _, err := InitialScreen(in, &baseline); err != nil {
		t.Fatalf("ordering changed frozen identity: %v", err)
	}
	if _, err := InitialScreen(in, nil); err == nil {
		t.Fatal("counted origins without frozen needs")
	}
}

func TestPartialOrUnknownLessonsCannotFillCoverage(t *testing.T) {
	in := screenFixture()
	in.Lessons[1].Eligibility = "unknown"
	in.Lessons[1].Origin = "unknown"
	in.Lessons[1].EvidenceRefs = nil
	r := screenWithFrozenNeeds(t, in)
	if r.Counts.E != 1 || r.EligibleLessons != 1 || r.EligibleProjects != 1 {
		t.Fatalf("partial lesson coverage counted: %+v", r)
	}
	in.Lessons[0].Eligibility = "ineligible"
	in.Lessons[0].Derivation = "owner"
	in.Lessons[0].Assurance = "owner_attested"
	r = screenWithFrozenNeeds(t, in)
	if r.Counts.H != 8 || r.Counts.E != 0 {
		t.Fatal("owner-only lesson removed need or filled automatic coverage")
	}
}

func TestInvalidAssessmentsFailWithoutEchoingTheirContents(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*TargetInventory)
	}{
		{"duplicate need", func(in *TargetInventory) { in.Targets[1].ID = in.Targets[0].ID }},
		{"duplicate lesson", func(in *TargetInventory) { in.Lessons[1].ID = in.Lessons[0].ID }},
		{"missing lesson", func(in *TargetInventory) { in.Targets[4].LessonIDs = []string{"ghp_SYNTHETIC_SECRET"} }},
		{"repeated required lesson", func(in *TargetInventory) { in.Targets[4].LessonIDs = []string{"L1", "L1"} }},
		{"mixed eligible lesson", func(in *TargetInventory) { in.Lessons[0].Origin = "mixed" }},
		{"unknown eligible lesson", func(in *TargetInventory) { in.Lessons[0].Origin = "unknown" }},
		{"owner substitute", func(in *TargetInventory) { in.Lessons[0].Derivation = "owner" }},
		{"missing evidence", func(in *TargetInventory) { in.Lessons[0].EvidenceRefs = nil }},
		{"blank evidence", func(in *TargetInventory) { in.Lessons[0].EvidenceRefs = []string{" "} }},
		{"missing reason", func(in *TargetInventory) { in.Targets[0].ReasonRefs = nil }},
		{"unsupported schema", func(in *TargetInventory) { in.SchemaVersion = 99 }},
		{"unscreened assessments", func(in *TargetInventory) { in.State = "not_screened" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := screenFixture()
			baseline, _ := FreezeNeeds(in)
			tc.change(&in)
			_, err := InitialScreen(in, &baseline)
			if err == nil {
				t.Fatal("invalid assessment accepted")
			}
			if strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
				t.Fatal("assessment content echoed")
			}
		})
	}
}
