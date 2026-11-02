package devtools

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func samplePlan(study int) plan {
	if study == 34 {
		return plan{Baseline: study34Source, AttemptCap: 114, Deadline: 1793562223, Seed: 34}
	}
	return plan{Study: 36, Source: study36Source, AttemptCap: 84, Deadline: 1793606373, Seed: 36}
}

// Independent fixture construction: 30 diagnosis + 84 comparison rows for #34;
// 84 standalone rows for #36, without using the evaluator's expected-cell builder.
func sampleRows(study int) []attempt {
	var rows []attempt
	add := func(stage, arm, category string, count int) {
		for block := 0; block < count; block++ {
			exit, clock := 0, 100.0
			parent := 100.0
			if arm == "candidate" {
				parent, clock = 70, 70
			}
			rows = append(rows, attempt{Attempt: len(rows), Stage: stage, Arm: arm, Category: category, Block: block, ParentMS: parent, OSMS: &clock, Exit: &exit, Record: "present_unknown_incomplete", Success: true})
		}
	}
	stage := "standalone36"
	if study == 34 {
		stage = "comparison"
		add("diagnosis", "baseline", "new_file_first", 10)
		add("diagnosis", "baseline", "warm", 10)
		add("diagnosis", "trace", "new_file_first", 5)
		add("diagnosis", "trace", "warm", 5)
	}
	for _, arm := range []string{"baseline", "candidate"} {
		add(stage, arm, "new_file_first", 20)
		add(stage, arm, "warm", 20)
		add(stage, arm, "idle_30s", 2)
	}
	return rows
}

func changeRow(rows []attempt, arm, category string, block int, f func(*attempt)) {
	for i := range rows {
		if rows[i].Stage != "diagnosis" && rows[i].Arm == arm && rows[i].Category == category && rows[i].Block == block {
			f(&rows[i])
			return
		}
	}
	panic("fixture row absent")
}

func TestHistoricalRulesRemainDistinct(t *testing.T) {
	for _, study := range []int{34, 36} {
		r := summarize(study, samplePlan(study), sampleRows(study))
		key := "build_recommendation"
		if study == 36 {
			key = "local_signal"
			if r.Decision["headroom_probe"] != true {
				t.Fatal("complete low-latency fixture must have headroom")
			}
		}
		if r.Decision["status"] != "complete_local_comparison" || r.Decision[key] != true || r.Decision["capture_profile_enabled"] != false || math.Abs(r.Decision["paired_median_improvement"].(float64)-.3) > 1e-12 {
			t.Fatal("frozen speed fixture changed")
		}
		// #34 did not veto baseline output or a warm tail regression. #36 does.
		for _, scenario := range []string{"baseline_output", "baseline_invalid", "warm_tail"} {
			rows := sampleRows(study)
			switch scenario {
			case "baseline_output":
				changeRow(rows, "baseline", "warm", 0, func(r *attempt) { r.Output.Stderr = 1 })
			case "baseline_invalid":
				changeRow(rows, "baseline", "warm", 0, func(r *attempt) { r.Record = "invalid" })
			case "warm_tail":
				for block := 18; block < 20; block++ {
					changeRow(rows, "candidate", "warm", block, func(r *attempt) { r.ParentMS = 900 })
				}
			}
			got := summarize(study, samplePlan(study), rows)
			if got.Decision[key] != (study == 34) {
				t.Fatalf("study %d silently changed rubric for %s", study, scenario)
			}
		}
	}
}

func TestAdverseTailsControlsAndAddedFailures(t *testing.T) {
	for _, study := range []int{34, 36} {
		for _, scenario := range []string{"first_tail", "candidate_output", "candidate_invalid", "added_failure"} {
			rows := sampleRows(study)
			switch scenario {
			case "first_tail":
				for block := 18; block < 20; block++ {
					changeRow(rows, "candidate", "new_file_first", block, func(r *attempt) { r.ParentMS = 900 })
				}
			case "candidate_output":
				changeRow(rows, "candidate", "warm", 0, func(r *attempt) { r.Output.Stdout = 1 })
			case "candidate_invalid":
				changeRow(rows, "candidate", "warm", 0, func(r *attempt) { r.Record = "absent" })
			case "added_failure":
				changeRow(rows, "candidate", "idle_30s", 0, func(r *attempt) { r.Success = false })
			}
			got := summarize(study, samplePlan(study), rows)
			key := "build_recommendation"
			if study == 36 {
				key = "local_signal"
			}
			if got.Decision[key] != false {
				t.Fatalf("study %d accepted %s", study, scenario)
			}
		}
	}
}

func TestFailedTailsAndMissingClocksStayVisible(t *testing.T) {
	rows := sampleRows(36)
	for block := 18; block < 20; block++ {
		changeRow(rows, "candidate", "new_file_first", block, func(r *attempt) {
			r.ParentMS, r.OSMS, r.Exit, r.Killed, r.Success = 1000, nil, nil, true, false
		})
	}
	r := summarize(36, samplePlan(36), rows)
	group := r.Decision["groups"].(map[string]study36Group)["candidate/new_file_first"]
	if group.N != 20 || group.Failures != 2 || group.P95 != 1000 || group.P99 != 1000 || group.OSMissing != 2 || r.Decision["headroom_probe"] != false || r.Decision["local_signal"] != false {
		t.Fatal("failed/censored tails or missing clocks disappeared")
	}
	for _, c := range r.Metrics.Groups {
		if c.Arm == "candidate" && c.Category == "new_file_first" && (c.Failures != 2 || *c.ParentP95 != 1000 || c.OSMissing != 2) {
			t.Fatal("cohort report hid failed tail")
		}
	}
	for _, clock := range []*float64{nil, new(float64), ptr(601)} {
		rows := sampleRows(36)
		changeRow(rows, "candidate", "idle_30s", 0, func(r *attempt) { r.OSMS = clock })
		if summarize(36, samplePlan(36), rows).Decision["headroom_probe"] != false {
			t.Fatal("headroom passed missing/invalid/slow OS clock")
		}
	}
}

func ptr(v float64) *float64 { return &v }

func TestFewerFailuresAndBinding(t *testing.T) {
	for _, study := range []int{34, 36} {
		rows := sampleRows(study)
		// No 20% speed signal. A paired baseline-only failure still earns the
		// historically specified local signal, without enabling capture.
		for i := range rows {
			rows[i].ParentMS = 100
		}
		changeRow(rows, "baseline", "new_file_first", 0, func(r *attempt) { r.Success = false })
		r := summarize(study, samplePlan(study), rows)
		key := "build_recommendation"
		if study == 36 {
			key = "local_signal"
		}
		if r.Decision[key] != true {
			t.Fatal("fewer-failure signal lost")
		}
		for _, scenario := range []string{"stage", "duplicate", "missing", "source", "cap", "deadline", "seed", "parent"} {
			p, cases := samplePlan(study), sampleRows(study)
			switch scenario {
			case "stage":
				cases[0].Stage = "foreign"
			case "duplicate":
				cases[0] = cases[1]
			case "missing":
				cases = cases[:len(cases)-1]
			case "source":
				p.Source, p.Baseline = "foreign", "foreign"
			case "cap":
				p.AttemptCap++
			case "deadline":
				p.Deadline++
			case "seed":
				p.Seed++
			case "parent":
				cases[0].ParentMS = 0
			}
			got := summarize(study, p, cases)
			if got.Decision["status"] == "complete_local_comparison" || got.Decision[key] != false || got.Metrics != nil {
				t.Fatalf("study %d accepted contaminated %s", study, scenario)
			}
		}
	}
}

func writeStudy(t *testing.T, study int) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(samplePlan(study))
	if err := os.WriteFile(filepath.Join(dir, "plan.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	var lines []byte
	for _, row := range sampleRows(study) {
		body, _ := json.Marshal(row)
		lines = append(lines, append(body, '\n')...)
	}
	if err := os.WriteFile(filepath.Join(dir, "attempts.jsonl"), lines, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestStudyReadOnlyAndPrivate(t *testing.T) {
	for _, study := range []int{34, 36} {
		dir := writeStudy(t, study)
		before, _ := os.ReadFile(filepath.Join(dir, "attempts.jsonl"))
		got, err := SummarizeStudy(study, dir)
		if err != nil || got.Decision["status"] != "complete_local_comparison" {
			t.Fatal("valid private study rejected")
		}
		after, _ := os.ReadFile(filepath.Join(dir, "attempts.jsonl"))
		if !reflect.DeepEqual(before, after) {
			t.Fatal("summary mutated input")
		}
	}
	for _, scenario := range []string{"dir_mode", "file_mode", "file_symlink", "dir_symlink", "dir_symlink_slash", "oversize", "line_limit", "unknown", "duplicate", "case_alias", "null", "missing", "trailing", "output_null", "output_missing", "invalid_utf8"} {
		t.Run(scenario, func(t *testing.T) {
			dir := writeStudy(t, 36)
			file := filepath.Join(dir, "attempts.jsonl")
			body, _ := json.Marshal(sampleRows(36)[0])
			var err error
			switch scenario {
			case "dir_mode":
				err = os.Chmod(dir, 0o755)
			case "file_mode":
				err = os.Chmod(file, 0o644)
			case "dir_symlink", "dir_symlink_slash":
				link := filepath.Join(t.TempDir(), "alias")
				err = os.Symlink(dir, link)
				dir = link
				if scenario == "dir_symlink_slash" {
					dir += string(os.PathSeparator)
				}
			case "file_symlink":
				if err = os.Rename(file, file+".original"); err == nil {
					err = os.Symlink(file+".original", file)
				}
			case "oversize":
				err = os.WriteFile(file, []byte(strings.Repeat("x", (1<<20)+1)), 0o600)
			case "line_limit":
				err = os.WriteFile(file, []byte(strings.Repeat(" ", 64<<10)+string(body)), 0o600)
			default:
				s := string(body)
				switch scenario {
				case "unknown":
					s = strings.Replace(s, "{", `{"ghp_SYNTHETIC_SECRET":1,`, 1)
				case "duplicate":
					s = strings.Replace(s, "{", `{"success":true,`, 1)
				case "case_alias":
					s = strings.Replace(s, "{", `{"Success":false,`, 1)
				case "null":
					s = strings.Replace(s, `"success":true`, `"success":null`, 1)
				case "missing":
					s = strings.Replace(s, `"success":true,`, "", 1)
				case "trailing":
					s += ` {"ghp_SYNTHETIC_SECRET":1}`
				case "output_null":
					s = strings.Replace(s, `"stdout":0`, `"stdout":null`, 1)
				case "output_missing":
					s = strings.Replace(s, `"stdout":0,`, "", 1)
				case "invalid_utf8":
					s += string([]byte{255})
				}
				err = os.WriteFile(file, []byte(s+"\n"), 0o600)
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = SummarizeStudy(36, dir)
			if err != ErrStudy || strings.Contains(err.Error(), "ghp_") {
				t.Fatal("unsafe input accepted or echoed")
			}
		})
	}
}

func TestJSONDuplicatesAndMissingAttempts(t *testing.T) {
	for _, body := range []string{`{"key":0,"k\u0065y":1}`, `{"nested":{"x":0,"x":1}}`, `[[[[`, `{}` + `{}`, `{"x": [1,]}`} {
		if uniqueJSON([]byte(body)) == nil {
			t.Fatal("invalid JSON admitted")
		}
	}
	dir := writeStudy(t, 36)
	if err := os.Remove(filepath.Join(dir, "attempts.jsonl")); err != nil {
		t.Fatal(err)
	}
	r, err := SummarizeStudy(36, dir)
	if err != nil || r.Decision["status"] != "incomplete" || r.Decision["attempts"] != 0 {
		t.Fatal("unstarted study must stay incomplete")
	}
}
