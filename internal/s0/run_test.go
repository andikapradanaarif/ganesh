package s0

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func text(value string) *string { return &value }

func inventoryRun(t *testing.T) Run {
	t.Helper()
	r, err := NewRun(time.Now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"synthetic-one", "synthetic-two", "synthetic-three"} {
		r.Projects = append(r.Projects, Project{filepath.Join(t.TempDir(), name), "allowed"})
	}
	r.Capture.Client = text("fixture-client")
	r.Capture.Build = text("fixture-build")
	r.Capture.Installation = text("fixture-installation")
	r.Capture.TranscriptAccess = text("fixture-permission-evidence")
	return r
}

func TestUnknownsAndPhaseSeparation(t *testing.T) {
	r, err := NewRun(time.Now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Projects) != 0 || r.Capture.Client != nil || r.ScreenStartedAt != nil || r.Limits.SpendUSD != nil || r.A23.Decision != "pending" {
		t.Fatal("invented run inputs")
	}
	check, err := Check(r, "inventory")
	if err != nil || check.InputsPresent {
		t.Fatalf("unknown inputs passed: %+v %v", check, err)
	}
	r = inventoryRun(t)
	check, err = Check(r, "inventory")
	if err != nil || !check.InputsPresent {
		t.Fatalf("held-out settings blocked inventory: %+v %v", check, err)
	}
	check, err = Check(r, "heldout")
	if err != nil || check.InputsPresent || len(check.Missing) == 0 {
		t.Fatalf("missing heldout settings passed: %+v %v", check, err)
	}
	if check.PermissionsIndependentlyVerified || check.FrozenEvidenceIndependentlyVerified || check.ProductionNarrativeEnabled {
		t.Fatal("presence checks created trust")
	}
}

func TestSelectionAndPermissionCannotBeAssumed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Run)
	}{
		{"duplicate project", func(r *Run) { r.Projects[1].Path = r.Projects[0].Path + "/../" + filepath.Base(r.Projects[0].Path) }},
		{"relative project", func(r *Run) { r.Projects[0].Path = "relative" }},
		{"permission missing", func(r *Run) { r.Projects[0].LocalReadPermission = "pending" }},
		{"client not selected", func(r *Run) { r.Capture.Client = nil }},
		{"build blank", func(r *Run) { r.Capture.Build = text("  ") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := inventoryRun(t)
			tc.change(&r)
			result, err := Check(r, "inventory")
			if err != nil || result.InputsPresent {
				t.Fatalf("unready inputs passed: %+v %v", result, err)
			}
		})
	}
}

func TestRepeatedMissingReasonsAreUniqueAndStable(t *testing.T) {
	r := inventoryRun(t)
	for i := range r.Projects {
		r.Projects[i].Path = "fixture-relative-duplicate"
		r.Projects[i].LocalReadPermission = "pending"
	}
	result, err := Check(r, "inventory")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"distinct absolute project paths", "explicit local-read permission for every selected project"}
	if !reflect.DeepEqual(result.Missing, want) || result.InputsPresent {
		t.Fatalf("duplicate diagnostics or changed readiness: %+v", result)
	}
	if result.Warnings == nil || len(result.Warnings) != 0 {
		t.Fatal("presence-only checks invented file permission warnings")
	}
	// The first project now has only the permission failure. Later projects
	// repeat both reasons, so sorting or adjacent-only compaction is wrong.
	r.Projects[0].Path = filepath.Join(t.TempDir(), "fixture-absolute")
	result, err = Check(r, "inventory")
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"explicit local-read permission for every selected project", "distinct absolute project paths"}
	if !reflect.DeepEqual(result.Missing, want) {
		t.Fatalf("first-occurrence order changed: %v", result.Missing)
	}
}

func TestHeldoutInputsDoNotGrantPermissionsOrA23Funding(t *testing.T) {
	r := inventoryRun(t)
	r.Capture.ConfigRevision = text("fixture-config")
	r.Capture.Measurements = text("fixture-measurements")
	r.Models = Models{text("fixture-answer"), text("fixture-grade"), text("different-fixture-family")}
	zero, seconds, tokens := 0.0, 60, 1000
	r.Limits = Limits{&zero, &seconds, &tokens, text("none"), text("no retries")}
	r.FrozenEvidence = text("fixture-manifest")
	result, err := Check(r, "heldout")
	if err != nil || !result.InputsPresent || r.A23.SpendUSD != nil || result.ProductionNarrativeEnabled {
		t.Fatalf("unexpected result: %+v %v", result, err)
	}
	negative := -1.0
	r.Limits.SpendUSD = &negative
	if result, _ = Check(r, "heldout"); result.InputsPresent {
		t.Fatal("negative spend cap passed")
	}
}

func TestWorkspaceIsPrivateAndNeverOverwritten(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new-run")
	r, err := NewRun(time.Now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := Prepare(dir, r); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Prepare(dir, r); err == nil {
		t.Fatal("reused existing workspace")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "run.json"))
	if !reflect.DeepEqual(before, after) {
		t.Fatal("changed existing run")
	}
	for _, name := range []string{dir, filepath.Join(dir, "run.json"), filepath.Join(dir, "targets.json")} {
		info, err := os.Stat(name)
		if err != nil || info.Mode().Perm()&0077 != 0 {
			t.Fatalf("private permissions: %s %v", name, err)
		}
	}
	if !strings.Contains(string(before), `"screen_started_at": null`) {
		t.Fatal("preparation started a screen")
	}
}
