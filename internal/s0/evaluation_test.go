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

// Synthetic declarations only; these are not an enrolled corpus or real tasks.
func evaluationFixture() EvaluationDefinition {
	d := NewEvaluationDefinition("s0-" + strings.Repeat("0", 32))
	seed := uint32(17)
	d.Seed = &seed
	h := strings.Repeat("a", 64)
	d.Bindings = EvaluationBindings{h, h, h, h, h, h, h, h, h, h, h}
	d.CalibrationIDs = []string{"calibration-two", "calibration-one"}
	for _, group := range []struct {
		role, stratum string
		n             int
	}{
		{"coder", "within", 7}, {"coder", "cross", 3}, {"planner", "within", 7}, {"planner", "cross", 3}, {"reviewer", "within", 6}, {"reviewer", "cross", 4},
	} {
		for i := 0; i < group.n; i++ {
			id := fmt.Sprintf("%s-%s-%02d", group.role, group.stratum, i)
			prompt := sha256.Sum256([]byte("synthetic-prompt/" + id))
			access := group.stratum == "within"
			d.Tasks = append(d.Tasks, EvaluationTask{ID: id, Role: group.role, Stratum: group.stratum, Diagnostic: i < 2, Tags: []string{"workflow", "gotcha"},
				Prompt: hex.EncodeToString(prompt[:]), Rubric: h, Oracle: h, Sources: h, BaselineCanAccess: &access})
		}
	}
	return d
}

func TestEvaluationScheduleFixedCountsAndCounterbalance(t *testing.T) {
	d := evaluationFixture()
	s, err := FreezeEvaluation(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Base) != 108 || len(s.Conditional) != 24 {
		t.Fatal("wrong base/conditional accounting")
	}
	tasks := map[string]EvaluationTask{}
	for _, task := range d.Tasks {
		tasks[task.ID] = task
	}
	seen := map[string]map[string]int{}
	first := map[string]map[string]int{"within": {}, "cross": {}}
	for i := 0; i < 60; i += 2 {
		a, b := s.Base[i], s.Base[i+1]
		if a.TaskID != b.TaskID || a.Arm == b.Arm || (a.Arm != "0" && a.Arm != "A") || (b.Arm != "0" && b.Arm != "A") || a.Corpus != "none" || b.Corpus != "none" {
			t.Fatal("invalid primary pair")
		}
		if seen[a.TaskID] != nil {
			t.Fatal("primary task repeated")
		}
		seen[a.TaskID] = map[string]int{"0": 1, "A": 1}
		first[tasks[a.TaskID].Stratum][a.Arm]++
	}
	if len(seen) != 30 || first["within"]["0"] != 10 || first["within"]["A"] != 10 || first["cross"]["0"] != 5 || first["cross"]["A"] != 5 {
		t.Fatal("primary stratum counterbalance failed")
	}
	positions := map[string][4]int{}
	diagnostics := map[string]bool{}
	for i := 60; i < 108; i += 4 {
		id := s.Base[i].TaskID
		if !tasks[id].Diagnostic || diagnostics[id] {
			t.Fatal("wrong diagnostic subset")
		}
		diagnostics[id] = true
		for j, answer := range s.Base[i : i+4] {
			if answer.TaskID != id {
				t.Fatal("diagnostic group split")
			}
			wantCorpus := map[string]string{"B": "none", "C": "reference_prose", "D": "full", "F": "full"}
			if want, ok := wantCorpus[answer.Arm]; !ok || answer.Corpus != want {
				t.Fatal("required diagnostic missing or changed corpus")
			}
			seen[id][answer.Arm]++
			p := positions[answer.Arm]
			p[j]++
			positions[answer.Arm] = p
		}
	}
	for _, arm := range []string{"B", "C", "D", "F"} {
		if positions[arm] != [4]int{3, 3, 3, 3} {
			t.Fatal("diagnostic order not counterbalanced")
		}
	}
	if len(diagnostics) != 12 {
		t.Fatal("wrong diagnostic count")
	}
	for id, counts := range seen {
		for _, arm := range []string{"B", "C", "D", "F"} {
			want := 0
			if tasks[id].Diagnostic {
				want = 1
			}
			if counts[arm] != want {
				t.Fatal("diagnostic answer omitted or duplicated")
			}
		}
	}
	filtered := map[string]bool{}
	firstD := 0
	for i := 0; i < 24; i += 2 {
		a, b := s.Conditional[i], s.Conditional[i+1]
		if !diagnostics[a.TaskID] || filtered[a.TaskID] || a.TaskID != b.TaskID || a.Corpus != "eligible_origin" || b.Corpus != "eligible_origin" || a.Arm == b.Arm || (a.Arm != "D" && a.Arm != "F") || (b.Arm != "D" && b.Arm != "F") {
			t.Fatal("filtered D/F do not cover identical frozen subset")
		}
		filtered[a.TaskID] = true
		if a.Arm == "D" {
			firstD++
		}
	}
	if len(filtered) != 12 || firstD != 6 {
		t.Fatal("conditional D/F counterbalance failed")
	}
	r, err := CheckEvaluation(s)
	if err != nil || r.Tasks != 30 || r.Within != 20 || r.Cross != 10 || r.Diagnostics != 12 || r.BaseAnswers != 108 || r.ConditionalAnswers != 24 || r.MaximumAnswers != 132 || r.Trigger != "not_evaluated" || r.ModelRunsAuthorized || r.EvidenceIndependentlyVerified || r.ProductionNarrativeEnabled {
		t.Fatal("planning report invented authorization or changed counts")
	}
}

func TestEvaluationRankAndCanonicalIdentity(t *testing.T) {
	// Independently computed with a literal byte sequence piped to shasum -a 256.
	rank := scheduleRank(17, "primary", "coder-within-00")
	if hex.EncodeToString(rank[:]) != "6af535903c96c92d066578a2020eaeb3b0e62373ef901fb581d45049f7e00ecc" {
		t.Fatal("rank byte contract changed")
	}
	d := evaluationFixture()
	a, err := FreezeEvaluation(d)
	if err != nil {
		t.Fatal(err)
	}
	// Serialized synthetic snapshot independently rehashed with jq/shasum.
	if a.Digest != "ec0a141b713efd83fe47a249e29d0184cbd02fd5dc7bd4cea45f7f6d751d3154" {
		t.Fatal("snapshot byte contract changed")
	}
	slices.Reverse(d.Tasks)
	slices.Reverse(d.CalibrationIDs)
	for i := range d.Tasks {
		slices.Reverse(d.Tasks[i].Tags)
	}
	b, err := FreezeEvaluation(d)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("set order changed canonical identity")
	}
	*d.Seed = 18
	c, err := FreezeEvaluation(d)
	if err != nil || c.Digest == a.Digest || reflect.DeepEqual(c.Base, a.Base) {
		t.Fatal("seed does not bind order and identity")
	}
	if _, err := CheckEvaluationAgainst(c, a.Digest); err != ErrEvaluation {
		t.Fatal("valid replacement silently changed frozen baseline")
	}
	if _, err := CheckEvaluationAgainst(a, a.Digest); err != nil {
		t.Fatal("retained baseline rejected")
	}
	for _, expected := range []string{"", strings.Repeat("A", 64), "ghp_SYNTHETIC_SECRET"} {
		if _, err := CheckEvaluationAgainst(a, expected); err != ErrEvaluation {
			t.Fatal("invalid expected digest admitted")
		}
	}
	// Later caller mutation must not rewrite the already returned snapshot.
	d = evaluationFixture()
	frozen, _ := FreezeEvaluation(d)
	*d.Seed = 99
	*d.Tasks[0].BaselineCanAccess = false
	d.Tasks[0].Tags[0] = "irrelevant_memory"
	d.CalibrationIDs[0] = "changed"
	if _, err := CheckEvaluation(frozen); err != nil {
		t.Fatal("snapshot aliases mutable caller input")
	}
}

func TestEvaluationRejectsIncompleteAndWrongStrata(t *testing.T) {
	for _, scenario := range []string{"empty", "schema", "run", "seed", "binding", "calibration_empty", "calibration_duplicate", "calibration_overlap", "role", "stratum", "role_mix", "diagnostic", "duplicate", "duplicate_inputs", "added", "removed", "prompt", "rubric", "oracle", "source", "access", "tag", "duplicate_tag"} {
		t.Run(scenario, func(t *testing.T) {
			d := evaluationFixture()
			switch scenario {
			case "empty":
				d = NewEvaluationDefinition(d.RunID)
			case "schema":
				d.SchemaVersion = 2
			case "run":
				d.RunID = "ghp_SYNTHETIC_SECRET"
			case "seed":
				d.Seed = nil
			case "binding":
				d.Bindings.EligibleCorpus = ""
			case "calibration_empty":
				d.CalibrationIDs = nil
			case "calibration_duplicate":
				d.CalibrationIDs = append(d.CalibrationIDs, d.CalibrationIDs[0])
			case "calibration_overlap":
				d.CalibrationIDs[0] = d.Tasks[0].ID
			case "role":
				d.Tasks[0].Role = "owner"
			case "stratum":
				d.Tasks[0].Stratum = "portfolio"
			case "role_mix":
				d.Tasks[0].Role = "planner"
			case "diagnostic":
				d.Tasks[0].Diagnostic = false
			case "duplicate":
				d.Tasks[1].ID = d.Tasks[0].ID
			case "duplicate_inputs":
				d.Tasks[1].Prompt = d.Tasks[0].Prompt
			case "added":
				d.Tasks = append(d.Tasks, d.Tasks[0])
			case "removed":
				d.Tasks = d.Tasks[:29]
			case "prompt":
				d.Tasks[0].Prompt = ""
			case "rubric":
				d.Tasks[0].Rubric = "ghp_SYNTHETIC_SECRET"
			case "oracle":
				d.Tasks[0].Oracle = strings.Repeat("A", 64)
			case "source":
				d.Tasks[0].Sources = ""
			case "access":
				d.Tasks[0].BaselineCanAccess = nil
			case "tag":
				d.Tasks[0].Tags = []string{"arbitrary"}
			case "duplicate_tag":
				d.Tasks[0].Tags = []string{"workflow", "workflow"}
			}
			if _, err := FreezeEvaluation(d); err != ErrEvaluation || strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
				t.Fatal("invalid declaration admitted or leaked")
			}
		})
	}
}

func copyEvaluation(t *testing.T, s EvaluationSnapshot) EvaluationSnapshot {
	t.Helper()
	body, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var result EvaluationSnapshot
	if json.Unmarshal(body, &result) != nil {
		t.Fatal("cannot copy fixture")
	}
	return result
}

func TestEvaluationSnapshotTamperingAndPrivateNonOverwrite(t *testing.T) {
	s, err := FreezeEvaluation(evaluationFixture())
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"schema", "algorithm", "digest", "recased_task", "changed_task", "added_task", "removed_task", "seed", "binding", "base_arm", "base_omission", "conditional", "definition_order"} {
		t.Run(scenario, func(t *testing.T) {
			changed := copyEvaluation(t, s)
			switch scenario {
			case "schema":
				changed.SchemaVersion = 2
			case "algorithm":
				changed.Algorithm = "other"
			case "digest":
				changed.Digest = strings.Repeat("b", 64)
			case "recased_task":
				changed.Definition.Tasks[0].ID = strings.ToUpper(changed.Definition.Tasks[0].ID)
			case "changed_task":
				value := !*changed.Definition.Tasks[0].BaselineCanAccess
				changed.Definition.Tasks[0].BaselineCanAccess = &value
			case "added_task":
				changed.Definition.Tasks = append(changed.Definition.Tasks, changed.Definition.Tasks[0])
			case "removed_task":
				changed.Definition.Tasks = changed.Definition.Tasks[:29]
			case "seed":
				*changed.Definition.Seed = 91
			case "binding":
				changed.Definition.Bindings.Grading = strings.Repeat("b", 64)
			case "base_arm":
				changed.Base[0].Arm = "F"
			case "base_omission":
				changed.Base = changed.Base[:107]
			case "conditional":
				changed.Conditional[0].Corpus = "full"
			case "definition_order":
				slices.Reverse(changed.Definition.Tasks)
			}
			if _, err := CheckEvaluation(changed); err != ErrEvaluation {
				t.Fatal("tampering accepted")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "snapshot.json")
	if err := SaveEvaluationSnapshot(path, s); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if SaveEvaluationSnapshot(path, s) == nil {
		t.Fatal("existing snapshot overwritten")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("existing snapshot changed")
	}
	loaded, err := ReadEvaluationSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CheckEvaluation(loaded); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("snapshot is not private")
	}
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadEvaluationSnapshot(path); err != nil {
		t.Fatal("private read-only snapshot rejected")
	}
}

func TestEvaluationInputBoundaries(t *testing.T) {
	body, _ := json.Marshal(evaluationFixture())
	for _, scenario := range []string{"unknown", "duplicate", "escaped_duplicate", "case_alias", "missing", "null_bool", "null_array", "trailing", "utf8", "oversize", "mode", "symlink", "directory", "deep"} {
		t.Run(scenario, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ghp_SYNTHETIC_SECRET.json")
			text := string(body)
			switch scenario {
			case "unknown":
				text = strings.Replace(text, "{", `{"raw":"ghp_SYNTHETIC_SECRET",`, 1)
			case "duplicate":
				text = strings.Replace(text, "{", `{"schema_version":1,`, 1)
			case "escaped_duplicate":
				text = strings.Replace(text, "{", `{"schema_\u0076ersion":1,`, 1)
			case "case_alias":
				text = strings.Replace(text, `"run_id"`, `"Run_ID"`, 1)
			case "missing":
				text = strings.Replace(text, `"diagnostic":true,`, "", 1)
			case "null_bool":
				text = strings.Replace(text, `"diagnostic":true`, `"diagnostic":null`, 1)
			case "null_array":
				text = strings.Replace(text, `"tags":["workflow","gotcha"]`, `"tags":null`, 1)
			case "trailing":
				text += ` {"raw":"ghp_SYNTHETIC_SECRET"}`
			case "utf8":
				text += string([]byte{255})
			case "oversize":
				text = strings.Repeat(" ", (1<<20)+1)
			case "deep":
				text = strings.Repeat("[", 34) + "0" + strings.Repeat("]", 34)
			}
			if os.WriteFile(path, []byte(text), 0o600) != nil {
				t.Fatal("cannot write fixture")
			}
			switch scenario {
			case "mode":
				if os.Chmod(path, 0o644) != nil {
					t.Fatal("chmod")
				}
			case "symlink":
				if os.Rename(path, path+".source") != nil || os.Symlink(path+".source", path) != nil {
					t.Fatal("symlink")
				}
			case "directory":
				path = filepath.Dir(path)
			}
			if _, err := ReadEvaluationDefinition(path); err != ErrEvaluation || strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
				t.Fatal("unsafe record admitted or error leaked content")
			}
		})
	}
}
