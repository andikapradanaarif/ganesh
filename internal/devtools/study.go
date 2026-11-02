package devtools

import (
	"math"
	"sort"
)

const (
	study34Source = "6efdbce26bdcb879cb2c7bb4bdaaed8ea11ea075"
	study36Source = "5eb607f1fc9271ca18dfae161a7769e8f398c3f3"
)

type cell struct {
	stage, arm, category string
	block                int
}

type pair struct {
	Category string `json:"category"`
	Block    int    `json:"block"`
}

type cohort struct {
	Stage        string   `json:"stage"`
	Arm          string   `json:"arm"`
	Category     string   `json:"category"`
	N            int      `json:"n"`
	Failures     int      `json:"failures"`
	ParentMedian float64  `json:"parent_median_ms"`
	ParentP95    *float64 `json:"parent_p95_ms"`
	ParentP99    *float64 `json:"parent_p99_ms"`
	OSP95        *float64 `json:"os_p95_ms"`
	OSP99        *float64 `json:"os_p99_ms"`
	OSMissing    int      `json:"os_missing"`
}

type metrics struct {
	Attempts int      `json:"attempts"`
	Groups   []cohort `json:"groups"`
}

type StudyReport struct {
	SchemaVersion int            `json:"schema_version"`
	Study         int            `json:"study"`
	Metrics       *metrics       `json:"metrics"`
	Decision      map[string]any `json:"decision"`
}

// SummarizeStudy reads existing records only. No producer, subprocess or clock
// measurement is invoked, and neither historical rule can enable a profile.
func SummarizeStudy(study int, directory string) (StudyReport, error) {
	if study != 34 && study != 36 {
		return StudyReport{}, ErrCommand
	}
	p, rows, err := readStudy(directory)
	if err != nil {
		return StudyReport{}, err
	}
	return summarize(study, p, rows), nil
}

func summarize(study int, p plan, rows []attempt) StudyReport {
	r := StudyReport{SchemaVersion: 1, Study: study, Decision: incomplete(study, "invalid_study_binding", len(rows))}
	bound := p.Study == 0 && p.Baseline == study34Source && p.AttemptCap == 114 && p.Deadline == 1793562223 && p.Seed == 34
	if study == 36 {
		bound = p.Study == 36 && p.Source == study36Source && p.AttemptCap == 84 && p.Deadline == 1793606373 && p.Seed == 36
	}
	if !bound {
		return r
	}
	if !completeCells(study, rows) {
		r.Decision = incomplete(study, "incomplete", len(rows))
		return r
	}
	m := summarizeMetrics(rows)
	r.Metrics = &m
	if study == 34 {
		r.Decision = evaluate34(rows)
	} else {
		r.Decision = evaluate36(rows)
	}
	return r
}

func incomplete(study int, status string, n int) map[string]any {
	r := map[string]any{"status": status, "attempts": n, "capture_profile_enabled": false}
	if study == 34 {
		r["build_recommendation"] = false
	} else {
		r["local_signal"], r["headroom_probe"] = false, false
	}
	return r
}

func completeCells(study int, rows []attempt) bool {
	expected := make(map[cell]bool)
	add := func(stage, arm, category string, n int) {
		for block := range n {
			expected[cell{stage, arm, category, block}] = true
		}
	}
	stage := "standalone36"
	if study == 34 {
		stage = "comparison"
		for _, c := range []string{"new_file_first", "warm"} {
			add("diagnosis", "baseline", c, 10)
			add("diagnosis", "trace", c, 5)
		}
	}
	for _, arm := range []string{"baseline", "candidate"} {
		add(stage, arm, "new_file_first", 20)
		add(stage, arm, "warm", 20)
		add(stage, arm, "idle_30s", 2)
	}
	if len(rows) != len(expected) {
		return false
	}
	for _, row := range rows {
		key := cell{row.Stage, row.Arm, row.Category, row.Block}
		if !expected[key] || !positive(row.ParentMS) {
			return false
		}
		delete(expected, key) // duplicates cannot stand in for a missing attempt
	}
	return len(expected) == 0
}

func positive(value float64) bool { return value > 0 && !math.IsInf(value, 0) && !math.IsNaN(value) }

func percentile(values []float64, q float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	value := ordered[int(math.Ceil(float64(len(ordered))*q))-1]
	return &value
}

func median(values []float64) float64 {
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	n := len(ordered)
	if n%2 == 1 {
		return ordered[n/2]
	}
	return (ordered[n/2-1] + ordered[n/2]) / 2
}

func summarizeMetrics(rows []attempt) metrics {
	groups := make(map[cell][]attempt)
	for _, row := range rows {
		key := cell{row.Stage, row.Arm, row.Category, 0}
		groups[key] = append(groups[key], row)
	}
	keys := make([]cell, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.stage != b.stage {
			return a.stage < b.stage
		}
		if a.arm != b.arm {
			return a.arm < b.arm
		}
		return a.category < b.category
	})
	r := metrics{Attempts: len(rows), Groups: make([]cohort, 0, len(keys))}
	for _, key := range keys {
		g := groups[key]
		times, spans, failures := groupNumbers(g, false)
		r.Groups = append(r.Groups, cohort{key.stage, key.arm, key.category, len(g), failures, median(times), percentile(times, .95), percentile(times, .99), percentile(spans, .95), percentile(spans, .99), len(g) - len(spans)})
	}
	return r
}

func groupNumbers(rows []attempt, positiveClocks bool) (times, spans []float64, failures int) {
	for _, row := range rows {
		times = append(times, row.ParentMS) // failed and censored attempts stay in tails
		if row.OSMS != nil && (!positiveClocks || positive(*row.OSMS)) {
			spans = append(spans, *row.OSMS)
		}
		if !row.Success {
			failures++
		}
	}
	return
}

func invalidRecord(row attempt) bool {
	return row.Record == "invalid" || (row.Exit != nil && *row.Exit == 0 && row.Record != "present_unknown_incomplete")
}

// evaluate34 intentionally preserves the original candidate-only control veto
// and first-execution tail gate. Do not silently substitute study 36's rubric.
func evaluate34(rows []attempt) map[string]any {
	byCell := make(map[cell]attempt)
	failures := map[string]int{"baseline": 0, "candidate": 0}
	unexpected, invalid := 0, 0
	for _, row := range rows {
		if row.Stage != "comparison" {
			continue
		}
		byCell[cell{"", row.Arm, row.Category, row.Block}] = row
		if !row.Success {
			failures[row.Arm]++
		}
		if row.Arm == "candidate" {
			if row.Output.Stdout != 0 || row.Output.Stderr != 0 {
				unexpected++
			}
			if invalidRecord(row) {
				invalid++
			}
		}
	}
	added := make([]pair, 0)
	improvements, firstB, firstC := []float64{}, []float64{}, []float64{}
	for _, category := range []string{"idle_30s", "new_file_first", "warm"} { // original sorted pair order
		n := 20
		if category == "idle_30s" {
			n = 2
		}
		for block := range n {
			b, c := byCell[cell{"", "baseline", category, block}], byCell[cell{"", "candidate", category, block}]
			if b.Success && !c.Success {
				added = append(added, pair{category, block})
			}
			if category == "new_file_first" {
				improvements = append(improvements, (b.ParentMS-c.ParentMS)/b.ParentMS)
				firstB, firstC = append(firstB, b.ParentMS), append(firstC, c.ParentMS)
			}
		}
	}
	med, b95, c95 := median(improvements), *percentile(firstB, .95), *percentile(firstC, .95)
	fewer, speed := failures["candidate"] < failures["baseline"], med >= .20 && c95 <= b95
	return map[string]any{"status": "complete_local_comparison", "attempts": len(rows), "failures": failures,
		"added_failure_pairs": added, "unexpected_output_attempts": unexpected, "invalid_candidate_records": invalid,
		"paired_first_improvements": improvements, "paired_median_improvement": med,
		"first_p95_ms": map[string]float64{"baseline": b95, "candidate": c95}, "fewer_failures": fewer, "speed_signal": speed,
		"build_recommendation": len(added) == 0 && unexpected == 0 && invalid == 0 && (fewer || speed), "capture_profile_enabled": false}
}

type study36Group struct {
	N         int      `json:"n"`
	Failures  int      `json:"failures"`
	Median    float64  `json:"median_ms"`
	P95       float64  `json:"p95_ms"`
	P99       float64  `json:"p99_ms"`
	OSP99     *float64 `json:"os_p99_ms"`
	OSMissing int      `json:"os_missing"`
}

func evaluate36(rows []attempt) map[string]any {
	groups := make(map[string]study36Group)
	controls := map[string]map[string]int{"baseline": {"unexpected_output": 0, "invalid_records": 0}, "candidate": {"unexpected_output": 0, "invalid_records": 0}}
	byCell := make(map[cell]attempt)
	for _, row := range rows {
		byCell[cell{"", row.Arm, row.Category, row.Block}] = row
		if row.Output.Stdout != 0 || row.Output.Stderr != 0 {
			controls[row.Arm]["unexpected_output"]++
		}
		if invalidRecord(row) {
			controls[row.Arm]["invalid_records"]++
		}
	}
	controlsOK := true
	for _, arm := range []string{"baseline", "candidate"} {
		for _, count := range controls[arm] {
			controlsOK = controlsOK && count == 0
		}
		for _, category := range []string{"new_file_first", "warm", "idle_30s"} {
			var g []attempt
			for _, row := range rows {
				if row.Arm == arm && row.Category == category {
					g = append(g, row)
				}
			}
			times, spans, failures := groupNumbers(g, true)
			groups[arm+"/"+category] = study36Group{len(g), failures, median(times), *percentile(times, .95), *percentile(times, .99), percentile(spans, .99), len(g) - len(spans)}
		}
	}
	added, improvements := make([]pair, 0), []float64{}
	noIncrease, tailsOK, headroom := true, true, controlsOK
	for _, category := range []string{"new_file_first", "warm", "idle_30s"} {
		b, c := groups["baseline/"+category], groups["candidate/"+category]
		noIncrease = noIncrease && c.Failures <= b.Failures
		if category != "idle_30s" {
			tailsOK = tailsOK && c.P95 <= b.P95
		}
		headroom = headroom && c.Failures == 0 && c.OSMissing == 0 && c.OSP99 != nil && *c.OSP99 <= 600
		for block := range b.N {
			br, cr := byCell[cell{"", "baseline", category, block}], byCell[cell{"", "candidate", category, block}]
			if br.Success && !cr.Success {
				added = append(added, pair{category, block})
			}
			if category == "new_file_first" {
				improvements = append(improvements, (br.ParentMS-cr.ParentMS)/br.ParentMS)
			}
		}
	}
	med := median(improvements)
	signal := controlsOK && len(added) == 0 && noIncrease && tailsOK && (groups["candidate/new_file_first"].Failures < groups["baseline/new_file_first"].Failures || med >= .20)
	return map[string]any{"status": "complete_local_comparison", "attempts": len(rows), "controls": controls, "groups": groups,
		"added_failure_pairs": added, "paired_first_improvements": improvements, "paired_median_improvement": med,
		"tail_gates_passed": tailsOK, "local_signal": signal, "headroom_probe": headroom, "capture_profile_enabled": false}
}
