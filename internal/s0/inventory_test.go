package s0

import "testing"

func TestOriginPartitionAndRouting(t *testing.T) {
	in := Inventory{Targets: []Target{{ID: "mixed-and-unknown", Mixed: true, Unknown: true}, {ID: "unknown", Unknown: true}, {ID: "owner-only"}}}
	d, err := Evaluate(in, false)
	if err != nil {
		t.Fatal(err)
	}
	if d.Counts != (Counts{H: 3, M: 1, U: 1}) || d.Route != "single_recheck_pending" {
		t.Fatalf("wrong partition/route: %+v", d)
	}
	// Unknown loss is diagnostic, not a second early-routing threshold.
	if d.Ratios.MixedUnknown != 2.0/3.0 || d.Ratios.Eligible != 0 {
		t.Fatalf("wrong ratios: %+v", d.Ratios)
	}
	in.Targets = in.Targets[:2]
	d, err = Evaluate(in, false)
	if err != nil || d.Route != "a2_3_decision" {
		t.Fatalf("50%% threshold: %+v, %v", d, err)
	}
}

func TestEmptyInventoryIsInconclusive(t *testing.T) {
	d, err := Evaluate(Inventory{}, false)
	if err != nil || d.Route != "inconclusive" || d.Ratios != nil {
		t.Fatalf("empty: %+v %v", d, err)
	}
}

func eligibleInventory() Inventory {
	in := Inventory{RecheckCount: 1}
	for i, id := range []string{"a", "b", "c", "d", "e", "f"} {
		project, stratum := "fixture-project-one", "within_project"
		if i >= 3 {
			project, stratum = "fixture-project-two", "cross_project"
		}
		in.Lessons = append(in.Lessons, Lesson{ID: id, Project: project, Automatic: true, Evidence: []string{"synthetic-test-reference"}})
		in.Targets = append(in.Targets, Target{ID: id, LessonIDs: []string{id}})
		in.Tasks = append(in.Tasks, FollowOn{ID: id, TargetIDs: []string{id}, Stratum: stratum})
	}
	return in
}

func TestFinalGateRequirements(t *testing.T) {
	d, err := Evaluate(eligibleInventory(), true)
	if err != nil || d.Route != "first_party_probe" || d.ProductionNarrativeEnabled {
		t.Fatalf("gate: %+v %v", d, err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Inventory)
	}{
		{"lessons are distinct", func(in *Inventory) { in.Targets[5].LessonIDs = []string{"a"} }},
		{"two projects", func(in *Inventory) {
			for i := range in.Lessons {
				in.Lessons[i].Project = "one"
			}
		}},
		{"all applicable tasks", func(in *Inventory) { in.Tasks = in.Tasks[:5] }},
		{"eligible coverage", func(in *Inventory) {
			for _, id := range []string{"g", "h", "i", "j", "k", "l", "m"} {
				in.Targets = append(in.Targets, Target{ID: id, Unknown: true})
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := eligibleInventory()
			tc.mutate(&in)
			d, err := Evaluate(in, true)
			if err != nil || d.Route != "a2_3_decision" {
				t.Fatalf("%+v %v", d, err)
			}
		})
	}
}

func TestInvalidEligibilityEvidence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Inventory)
	}{
		{"unknown", func(in *Inventory) { in.Targets[0].Unknown = true }},
		{"owner", func(in *Inventory) { in.Lessons[0].Automatic = false }},
		{"no evidence", func(in *Inventory) { in.Lessons[0].Evidence = nil }},
		{"missing lesson", func(in *Inventory) { in.Targets[0].LessonIDs = []string{"missing"} }},
		{"duplicate task", func(in *Inventory) { in.Tasks[1].ID = in.Tasks[0].ID }},
		{"repeated recheck", func(in *Inventory) { in.RecheckCount = 2 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := eligibleInventory()
			tc.mutate(&in)
			if _, err := Evaluate(in, true); err == nil {
				t.Fatal("accepted invalid inventory")
			}
		})
	}
}
