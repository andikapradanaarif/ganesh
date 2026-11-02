package s0

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func resultInt(n int64) *int64             { return &n }
func resultErrors(ids ...string) *[]string { v := append([]string{}, ids...); return &v }

func syntheticResult(slot EvaluationAnswer) EvaluationResult {
	h := sha256.Sum256([]byte("synthetic-slot-evidence/" + slot.TaskID + "/" + slot.Arm + "/" + slot.Corpus))
	digest := hex.EncodeToString(h[:])
	return EvaluationResult{slot, "fail", resultErrors(), resultInt(100), resultInt(10), resultInt(1), &digest}
}

func resultFixture(t *testing.T) (EvaluationSnapshot, EvaluationResults) {
	t.Helper()
	s, err := FreezeEvaluation(evaluationFixture())
	if err != nil {
		t.Fatal(err)
	}
	in := EvaluationResults{SchemaVersion: 1, SnapshotDigest: s.Digest, Results: []EvaluationResult{}}
	for _, slot := range s.Base {
		in.Results = append(in.Results, syntheticResult(slot))
	}
	return s, in
}

func changeResult(in *EvaluationResults, id, arm, corpus string, change func(*EvaluationResult)) {
	for i := range in.Results {
		if in.Results[i].Answer == (EvaluationAnswer{id, arm, corpus}) {
			change(&in.Results[i])
			return
		}
	}
	panic("synthetic result absent")
}

func outcome(in *EvaluationResults, id, arm, corpus, value string) {
	changeResult(in, id, arm, corpus, func(r *EvaluationResult) { r.Outcome = value })
}

func benefitResult(t *testing.T, a, z int) (EvaluationSnapshot, EvaluationResults) {
	t.Helper()
	s, in := resultFixture(t)
	for i, task := range s.Definition.Tasks {
		if i < a {
			outcome(&in, task.ID, "A", "none", "pass")
		}
		if i < z {
			outcome(&in, task.ID, "0", "none", "pass")
		}
	}
	return s, in
}

func resultReport(t *testing.T, s EvaluationSnapshot, in EvaluationResults) EvaluationResultsReport {
	t.Helper()
	r, err := ReportEvaluation(s, s.Digest, in)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func firstDiagnostic(s EvaluationSnapshot) string {
	for _, task := range s.Definition.Tasks {
		if task.Diagnostic {
			return task.ID
		}
	}
	panic("missing diagnostic")
}

func addConditional(s EvaluationSnapshot, in *EvaluationResults) {
	for _, slot := range s.Conditional {
		in.Results = append(in.Results, syntheticResult(slot))
	}
}

func TestResultPrimaryBoundaries(t *testing.T) {
	for _, tc := range []struct {
		a, z        int
		want, floor string
	}{{20, 17, "not_met", "not_met"}, {21, 18, "met", "met"}, {21, 19, "not_met", "met"}, {30, 27, "met", "met"}} {
		t.Run(fmt.Sprintf("%d-%d", tc.a, tc.z), func(t *testing.T) {
			s, in := benefitResult(t, tc.a, tc.z)
			r := resultReport(t, s, in)
			if r.Primary.NumericCriteria != tc.want || r.Primary.AFloor != tc.floor || r.Primary.Overall.CandidateCompleted != tc.a || r.Primary.Overall.BaselineCompleted != tc.z || r.Primary.Overall.Net != tc.a-tc.z || r.Primary.Overall.Resolved != 30 {
				t.Fatal("primary boundary changed")
			}
			if tc.z < 21 && r.Primary.ZeroFloor != "not_met" {
				t.Fatal("baseline floor hidden")
			}
			if r.Base.Expected != 108 || r.Base.Present != 108 || r.Conditional.Expected != 24 || r.Conditional.Missing != 24 || r.State != "metadata_complete" || r.ConditionalRequired == nil || *r.ConditionalRequired {
				t.Fatal("base/conditional accounting wrong")
			}
			if r.ExperimentAccepted || r.EvidenceIndependentlyVerified || r.ModelRunsAuthorized || r.ProductionNarrativeEnabled {
				t.Fatal("numeric criteria authorized a run or quality claim")
			}
		})
	}
	for _, extra := range []int64{0, 1} {
		s, in := benefitResult(t, 21, 18)
		for i := range in.Results {
			if in.Results[i].Answer.Arm == "0" {
				in.Results[i].AnsweringTokens = resultInt(4)
			}
			if in.Results[i].Answer.Arm == "A" {
				in.Results[i].AnsweringTokens = resultInt(5)
			}
		}
		changeResult(&in, s.Definition.Tasks[0].ID, "A", "none", func(r *EvaluationResult) { *r.AnsweringTokens += extra })
		r := resultReport(t, s, in)
		if *r.Primary.TokenIncreaseWithinLimit != (extra == 0) || (r.Primary.NumericCriteria == "met") != (extra == 0) {
			t.Fatal("25 percent token boundary changed")
		}
	}
}

func TestResultStratumRegressionCannotBePooledAway(t *testing.T) {
	for _, loss := range []int{1, 2} {
		s, in := resultFixture(t)
		var within, cross []string
		for _, task := range s.Definition.Tasks {
			if task.Stratum == "within" {
				within = append(within, task.ID)
			} else {
				cross = append(cross, task.ID)
			}
		}
		excluded := map[string]bool{}
		for _, id := range within[:loss] {
			excluded[id] = true
			outcome(&in, id, "0", "none", "pass")
		}
		for _, id := range cross[:loss+3] {
			excluded[id] = true
			outcome(&in, id, "A", "none", "pass")
		}
		common := 0
		for _, task := range s.Definition.Tasks {
			if !excluded[task.ID] && common < 18 {
				outcome(&in, task.ID, "0", "none", "pass")
				outcome(&in, task.ID, "A", "none", "pass")
				common++
			}
		}
		r := resultReport(t, s, in)
		if r.Primary.Overall.Net != 3 || *r.Primary.Within.MaterialRegression != (loss == 2) || (r.Primary.NumericCriteria == "met") != (loss == 1) {
			t.Fatal("stratum regression was pooled away or one-task loss overruled protocol")
		}
		if r.Primary.Within.Pairs.Expected != 20 || r.Primary.Cross.Pairs.Expected != 10 || r.Primary.Within.Pairs.BaselineOnly != loss || len(r.Primary.AccessWins) != loss+3 {
			t.Fatal("stratum/access accounting wrong")
		}
	}
}

func TestResultCriticalErrorsComparedPerTaskAndIdentity(t *testing.T) {
	for _, scenario := range []string{"same", "new_identity", "other_task"} {
		s, in := benefitResult(t, 21, 18)
		first, second := s.Definition.Tasks[0].ID, s.Definition.Tasks[1].ID
		changeResult(&in, first, "0", "none", func(r *EvaluationResult) { r.CriticalErrors = resultErrors("critical-a") })
		id, label := first, "critical-a"
		if scenario == "new_identity" {
			label = "critical-b"
		}
		if scenario == "other_task" {
			id = second
		}
		changeResult(&in, id, "A", "none", func(r *EvaluationResult) { r.CriticalErrors = resultErrors(label) })
		r := resultReport(t, s, in)
		want := 0
		if scenario != "same" {
			want = 1
		}
		if *r.Primary.Overall.IntroducedCritical != want || r.Primary.Overall.BaselineCritical != 1 || r.Primary.Overall.CandidateCritical != 1 || (r.Primary.NumericCriteria == "met") != (want == 0) {
			t.Fatal("aggregate critical count laundered an introduced error")
		}
	}
}

func TestResultHistoryTriggerBoundaries(t *testing.T) {
	for _, scenario := range []string{"none", "D_win", "F_win", "new_critical", "same_critical", "C_win", "B_win", "20_percent", "less_than_20", "unknown_tokens", "zero_tokens", "non_diagnostic_tokens"} {
		t.Run(scenario, func(t *testing.T) {
			s, in := resultFixture(t)
			id := firstDiagnostic(s)
			want := "not_triggered"
			switch scenario {
			case "D_win", "F_win", "new_critical", "same_critical":
				arm := "D"
				if scenario == "F_win" {
					arm = "F"
				}
				outcome(&in, id, arm, "full", "pass")
				want = "triggered"
				if scenario == "new_critical" || scenario == "same_critical" {
					changeResult(&in, id, "D", "full", func(r *EvaluationResult) { r.CriticalErrors = resultErrors("unsafe") })
					if scenario == "new_critical" {
						want = "not_triggered"
					} else {
						changeResult(&in, id, "A", "none", func(r *EvaluationResult) { r.CriticalErrors = resultErrors("unsafe") })
					}
				}
			case "C_win":
				outcome(&in, id, "C", "reference_prose", "pass")
			case "B_win":
				outcome(&in, id, "B", "none", "pass")
			case "20_percent", "less_than_20":
				for i := range in.Results {
					if in.Results[i].Answer.Arm == "D" {
						in.Results[i].AnsweringTokens = resultInt(80)
					}
				}
				want = "triggered"
				if scenario == "less_than_20" {
					changeResult(&in, id, "D", "full", func(r *EvaluationResult) { r.AnsweringTokens = resultInt(81) })
					want = "not_triggered"
				}
			case "unknown_tokens":
				changeResult(&in, id, "D", "full", func(r *EvaluationResult) { r.AnsweringTokens = nil })
				want = "incomplete"
			case "zero_tokens":
				for i := range in.Results {
					if in.Results[i].Answer.Arm == "D" || in.Results[i].Answer.Arm == "A" {
						in.Results[i].AnsweringTokens = resultInt(0)
					}
				}
			case "non_diagnostic_tokens":
				for _, task := range s.Definition.Tasks {
					if !task.Diagnostic {
						changeResult(&in, task.ID, "A", "none", func(r *EvaluationResult) { r.AnsweringTokens = resultInt(100000) })
					}
				}
			}
			r := resultReport(t, s, in)
			if r.HistoryTrigger != want || r.Diagnostics["D"].Pairs.Expected != 12 || r.Diagnostics["F"].Pairs.Expected != 12 || r.Diagnostics["C"].HistoryTrigger != "not_applicable" || r.Diagnostics["B"].HistoryTrigger != "not_applicable" {
				t.Fatal("wrong matched-subset trigger")
			}
			if want == "triggered" && (r.State != "conditional_metadata_required" || r.Conditional.Missing != 24) {
				t.Fatal("required rerun omitted")
			}
		})
	}
	// A known completion win triggers the OR rule despite unknown token usage;
	// missing usage still makes the full metadata report incomplete.
	s, in := resultFixture(t)
	id := firstDiagnostic(s)
	outcome(&in, id, "D", "full", "pass")
	changeResult(&in, id, "D", "full", func(r *EvaluationResult) { r.AnsweringTokens = nil })
	changeResult(&in, id, "F", "full", func(r *EvaluationResult) { r.CriticalErrors = nil })
	r := resultReport(t, s, in)
	if r.HistoryTrigger != "triggered" || r.State != "incomplete_base_metadata" || r.Base.UnknownTokens != 1 {
		t.Fatal("OR trigger or unknown usage misclassified")
	}
}

func TestResultConditionalAccountingAndPolicyLoss(t *testing.T) {
	s, in := resultFixture(t)
	id := firstDiagnostic(s)
	outcome(&in, id, "D", "full", "pass")
	addConditional(s, &in)
	r := resultReport(t, s, in)
	if r.State != "metadata_complete" || r.Conditional.Present != 24 || r.PolicyLoss["D"].Overall.BaselineOnly != 1 || r.PolicyLoss["D"].Overall.Net != -1 || r.PolicyLoss["D"].Within.Expected != 6 || r.PolicyLoss["D"].Cross.Expected != 6 || r.Base.Present+r.Conditional.Present != 132 {
		t.Fatal("conditional results not paired with full corpus")
	}
	in.Results = in.Results[:len(in.Results)-1]
	if r := resultReport(t, s, in); r.State != "conditional_metadata_required" || r.Conditional.Missing != 1 {
		t.Fatal("partial rerun treated as complete")
	}
	s, in = resultFixture(t)
	addConditional(s, &in)
	r = resultReport(t, s, in)
	if r.State != "unexpected_conditional_metadata" || r.UnexpectedConditional != 24 || *r.ConditionalRequired {
		t.Fatal("unjustified conditional rows hidden")
	}
}

func TestResultUnknownsNeverBecomeFailuresOrZero(t *testing.T) {
	s, in := benefitResult(t, 21, 18)
	id := s.Definition.Tasks[0].ID
	for _, field := range []string{"completion", "critical", "evidence", "tokens", "latency", "cost"} {
		copy := in
		copy.Results = append([]EvaluationResult{}, in.Results...)
		changeResult(&copy, id, "A", "none", func(r *EvaluationResult) {
			switch field {
			case "completion":
				r.Outcome = "indeterminate"
			case "critical":
				r.CriticalErrors = nil
			case "evidence":
				r.EvidenceSHA256 = nil
			case "tokens":
				r.AnsweringTokens = nil
			case "latency":
				r.ElapsedMS = nil
			case "cost":
				r.CostMicroUSD = nil
			}
		})
		r := resultReport(t, s, copy)
		if r.State != "incomplete_base_metadata" {
			t.Fatal("unknown metadata disappeared")
		}
		if field == "completion" || field == "critical" || field == "evidence" {
			if r.Primary.Overall.Resolved != 29 || r.Primary.NumericCriteria != "incomplete" || r.Primary.Overall.IntroducedCritical != nil {
				t.Fatal("unresolved grade treated as a failure/known critical set")
			}
		}
		if field == "tokens" && (r.Primary.Overall.CandidateTokens != nil || r.Primary.Overall.BaselineTokens == nil || r.Primary.NumericCriteria != "incomplete") {
			t.Fatal("unknown tokens became zero or hid known baseline")
		}
	}
	empty := EvaluationResults{1, s.Digest, []EvaluationResult{}}
	r := resultReport(t, s, empty)
	if r.Base.Missing != 108 || r.Primary.Overall.Resolved != 0 || r.Primary.Overall.BothFail != 0 || r.Primary.Within.MaterialRegression != nil || r.Primary.NumericCriteria != "incomplete" {
		t.Fatal("missing rows fabricated outcomes")
	}
}

func TestResultIdentityValidationAndOverflow(t *testing.T) {
	for _, scenario := range []string{"schema", "plan", "duplicate", "unknown_slot", "reused_evidence", "bad_outcome", "bad_evidence", "bad_critical", "duplicate_critical", "negative_tokens", "negative_latency", "negative_cost", "overflow"} {
		t.Run(scenario, func(t *testing.T) {
			s, in := resultFixture(t)
			switch scenario {
			case "schema":
				in.SchemaVersion = 2
			case "plan":
				in.SnapshotDigest = strings.Repeat("b", 64)
			case "duplicate":
				in.Results = append(in.Results, in.Results[0])
			case "unknown_slot":
				in.Results[0].Answer.TaskID = "ghp_SYNTHETIC_SECRET"
			case "reused_evidence":
				in.Results[1].EvidenceSHA256 = in.Results[0].EvidenceSHA256
			case "bad_outcome":
				in.Results[0].Outcome = "ghp_SYNTHETIC_SECRET"
			case "bad_evidence":
				v := "ghp_SYNTHETIC_SECRET"
				in.Results[0].EvidenceSHA256 = &v
			case "bad_critical":
				in.Results[0].CriticalErrors = resultErrors("invalid/path")
			case "duplicate_critical":
				in.Results[0].CriticalErrors = resultErrors("unsafe", "unsafe")
			case "negative_tokens":
				in.Results[0].AnsweringTokens = resultInt(-1)
			case "negative_latency":
				in.Results[0].ElapsedMS = resultInt(-1)
			case "negative_cost":
				in.Results[0].CostMicroUSD = resultInt(-1)
			case "overflow":
				in.Results[0].AnsweringTokens = resultInt(math.MaxInt64)
			}
			if _, err := ReportEvaluation(s, s.Digest, in); err != ErrResults || strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
				t.Fatal("invalid metadata passed or leaked")
			}
		})
	}
	s, in := benefitResult(t, 21, 18)
	for i := range in.Results {
		in.Results[i].AnsweringTokens = resultInt(0)
	}
	id := s.Definition.Tasks[0].ID
	changeResult(&in, id, "0", "none", func(r *EvaluationResult) { r.AnsweringTokens = resultInt(math.MaxInt64 / 2) })
	changeResult(&in, id, "A", "none", func(r *EvaluationResult) { r.AnsweringTokens = resultInt(math.MaxInt64 / 2) })
	if r := resultReport(t, s, in); r.Primary.NumericCriteria != "met" {
		t.Fatal("large ratio products overflowed")
	}
	a := resultReport(t, s, in)
	slices.Reverse(in.Results)
	changeResult(&in, id, "A", "none", func(r *EvaluationResult) { r.CriticalErrors = resultErrors("z", "a") })
	b := resultReport(t, s, in)
	changeResult(&in, id, "A", "none", func(r *EvaluationResult) { r.CriticalErrors = resultErrors("a", "z") })
	c := resultReport(t, s, in)
	if b.ResultsDigest != c.ResultsDigest || a.ResultsDigest == b.ResultsDigest {
		t.Fatal("canonical critical-set identity changed or ignored values")
	}
	// Restore the declaration, retaining reversed input order.
	changeResult(&in, id, "A", "none", func(r *EvaluationResult) { r.CriticalErrors = resultErrors() })
	if d := resultReport(t, s, in); !reflect.DeepEqual(a, d) {
		t.Fatal("row ordering changed the report")
	}
	if _, err := ReportEvaluation(s, strings.Repeat("f", 64), in); err != ErrEvaluation {
		t.Fatal("report accepted an unpinned snapshot")
	}
}
