package s0

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"math/bits"
	"sort"
)

var ErrResults = errors.New("invalid or inconsistent S0 result metadata")
var ErrIncompleteResults = errors.New("S0 result metadata is incomplete or requires protocol review")

// EvaluationResult describes a per-slot grading/usage declaration, never an
// answer or a verified grade. Null fields remain unknown rather than zero.
type EvaluationResult struct {
	Answer          EvaluationAnswer `json:"answer"`
	Outcome         string           `json:"outcome"`
	CriticalErrors  *[]string        `json:"critical_error_ids"`
	AnsweringTokens *int64           `json:"answering_tokens"`
	ElapsedMS       *int64           `json:"elapsed_ms"`
	CostMicroUSD    *int64           `json:"cost_micro_usd"`
	EvidenceSHA256  *string          `json:"evidence_sha256"`
}

type EvaluationResults struct {
	SchemaVersion  int                `json:"schema_version"`
	SnapshotDigest string             `json:"snapshot_digest"`
	Results        []EvaluationResult `json:"results"`
}

type EvaluationCoverage struct {
	Expected          int   `json:"expected"`
	Present           int   `json:"present"`
	Missing           int   `json:"missing"`
	Unresolved        int   `json:"unresolved"`
	KnownTokens       int64 `json:"known_answering_tokens"`
	UnknownTokens     int   `json:"unknown_token_slots"`
	KnownElapsedMS    int64 `json:"known_elapsed_ms"`
	UnknownElapsed    int   `json:"unknown_elapsed_slots"`
	KnownCostMicroUSD int64 `json:"known_cost_micro_usd"`
	UnknownCost       int   `json:"unknown_cost_slots"`
}

func (c EvaluationCoverage) complete() bool {
	return c.Missing == 0 && c.Unresolved == 0 && c.UnknownTokens == 0 && c.UnknownElapsed == 0 && c.UnknownCost == 0
}

type EvaluationPairs struct {
	Expected           int      `json:"expected_pairs"`
	Resolved           int      `json:"resolved_pairs"`
	BothPass           int      `json:"both_pass"`
	BothFail           int      `json:"both_fail"`
	BaselineOnly       int      `json:"baseline_only"`
	CandidateOnly      int      `json:"candidate_only"`
	BaselineCompleted  int      `json:"baseline_completed_resolved_pairs"`
	CandidateCompleted int      `json:"candidate_completed_resolved_pairs"`
	Net                int      `json:"net_resolved_pairs"`
	DeltaPoints        *float64 `json:"completion_delta_percentage_points"`
	BaselineCritical   int      `json:"baseline_critical_errors_resolved_pairs"`
	CandidateCritical  int      `json:"candidate_critical_errors_resolved_pairs"`
	IntroducedCritical *int     `json:"introduced_critical_errors"`
	BaselineTokens     *int64   `json:"baseline_answering_tokens"`
	CandidateTokens    *int64   `json:"candidate_answering_tokens"`
}

type EvaluationStratum struct {
	Pairs              EvaluationPairs `json:"pairs"`
	MaterialRegression *bool           `json:"material_regression"`
}

type EvaluationPrimary struct {
	Overall                  EvaluationPairs   `json:"overall"`
	Within                   EvaluationStratum `json:"within_project"`
	Cross                    EvaluationStratum `json:"cross_project"`
	AFloor                   string            `json:"arm_a_completion_floor"`
	ZeroFloor                string            `json:"arm_0_completion_floor"`
	TokenIncreaseWithinLimit *bool             `json:"token_increase_within_25_percent"`
	NumericCriteria          string            `json:"declared_numeric_criteria"`
	AccessWins               []string          `json:"declared_cross_project_access_wins"`
}

type EvaluationDiagnostic struct {
	Pairs          EvaluationPairs `json:"pairs_against_a"`
	HistoryTrigger string          `json:"history_trigger"`
}

type EvaluationPolicyLoss struct {
	Overall EvaluationPairs `json:"overall"`
	Within  EvaluationPairs `json:"within_project"`
	Cross   EvaluationPairs `json:"cross_project"`
}

type EvaluationResultsReport struct {
	SchemaVersion                 int                             `json:"schema_version"`
	State                         string                          `json:"state"`
	SnapshotDigest                string                          `json:"snapshot_digest"`
	ResultsDigest                 string                          `json:"results_digest"`
	Base                          EvaluationCoverage              `json:"base"`
	Conditional                   EvaluationCoverage              `json:"conditional"`
	Primary                       EvaluationPrimary               `json:"primary"`
	Diagnostics                   map[string]EvaluationDiagnostic `json:"diagnostics"`
	HistoryTrigger                string                          `json:"eligible_origin_trigger"`
	ConditionalRequired           *bool                           `json:"conditional_required_by_declared_trigger"`
	PolicyLoss                    map[string]EvaluationPolicyLoss `json:"full_to_eligible_origin"`
	UnexpectedConditional         int                             `json:"conditional_slots_without_established_trigger"`
	EvidenceIndependentlyVerified bool                            `json:"evidence_independently_verified"`
	ExperimentAccepted            bool                            `json:"experiment_accepted"`
	ModelRunsAuthorized           bool                            `json:"model_runs_authorized"`
	ProductionNarrativeEnabled    bool                            `json:"production_narrative_enabled"`
}

func resultResolved(r EvaluationResult, exists bool) bool {
	return exists && r.Outcome != "indeterminate" && r.CriticalErrors != nil && r.EvidenceSHA256 != nil
}

func canonicalResults(s EvaluationSnapshot, in EvaluationResults) (EvaluationResults, map[EvaluationAnswer]EvaluationResult, error) {
	if in.SchemaVersion != 1 || in.SnapshotDigest != s.Digest || len(in.Results) > len(s.Base)+len(s.Conditional) {
		return EvaluationResults{}, nil, ErrResults
	}
	order := map[EvaluationAnswer]int{}
	for _, slot := range s.Base {
		order[slot] = len(order)
	}
	for _, slot := range s.Conditional {
		order[slot] = len(order)
	}
	rows := make([]EvaluationResult, 0, len(in.Results))
	bySlot := map[EvaluationAnswer]EvaluationResult{}
	evidence := map[string]bool{}
	var totals [3]int64
	for _, r := range in.Results {
		if _, ok := order[r.Answer]; !ok {
			return EvaluationResults{}, nil, ErrResults
		}
		if _, exists := bySlot[r.Answer]; exists {
			return EvaluationResults{}, nil, ErrResults
		}
		if r.Outcome != "pass" && r.Outcome != "fail" && r.Outcome != "indeterminate" {
			return EvaluationResults{}, nil, ErrResults
		}
		if r.EvidenceSHA256 != nil {
			if !evaluationHash.MatchString(*r.EvidenceSHA256) || evidence[*r.EvidenceSHA256] {
				return EvaluationResults{}, nil, ErrResults
			}
			evidence[*r.EvidenceSHA256] = true
		}
		if r.CriticalErrors != nil {
			if *r.CriticalErrors == nil || len(*r.CriticalErrors) > 64 {
				return EvaluationResults{}, nil, ErrResults
			}
			ids := append([]string{}, (*r.CriticalErrors)...)
			sort.Strings(ids)
			for i, id := range ids {
				if !evaluationID.MatchString(id) || (i > 0 && ids[i-1] == id) {
					return EvaluationResults{}, nil, ErrResults
				}
			}
			r.CriticalErrors = &ids
		}
		for i, v := range []*int64{r.AnsweringTokens, r.ElapsedMS, r.CostMicroUSD} {
			if v != nil {
				if *v < 0 || totals[i] > math.MaxInt64-*v {
					return EvaluationResults{}, nil, ErrResults
				}
				totals[i] += *v
			}
		}
		rows = append(rows, r)
		bySlot[r.Answer] = r
	}
	sort.Slice(rows, func(i, j int) bool { return order[rows[i].Answer] < order[rows[j].Answer] })
	in.Results = rows
	return in, bySlot, nil
}

func resultCoverage(slots []EvaluationAnswer, rows map[EvaluationAnswer]EvaluationResult) EvaluationCoverage {
	c := EvaluationCoverage{Expected: len(slots)}
	for _, slot := range slots {
		r, exists := rows[slot]
		if !exists {
			c.Missing++
		} else {
			c.Present++
			if !resultResolved(r, true) {
				c.Unresolved++
			}
		}
		if r.AnsweringTokens == nil {
			c.UnknownTokens++
		} else {
			c.KnownTokens += *r.AnsweringTokens
		}
		if r.ElapsedMS == nil {
			c.UnknownElapsed++
		} else {
			c.KnownElapsedMS += *r.ElapsedMS
		}
		if r.CostMicroUSD == nil {
			c.UnknownCost++
		} else {
			c.KnownCostMicroUSD += *r.CostMicroUSD
		}
	}
	return c
}

func compareResultPairs(tasks []EvaluationTask, baselineArm, baselineCorpus, candidateArm, candidateCorpus string, rows map[EvaluationAnswer]EvaluationResult) EvaluationPairs {
	p := EvaluationPairs{Expected: len(tasks)}
	var bTokens, cTokens int64
	baselineTokensKnown, candidateTokensKnown := true, true
	introduced := 0
	for _, task := range tasks {
		b, bok := rows[EvaluationAnswer{task.ID, baselineArm, baselineCorpus}]
		c, cok := rows[EvaluationAnswer{task.ID, candidateArm, candidateCorpus}]
		if !bok || b.AnsweringTokens == nil {
			baselineTokensKnown = false
		} else {
			bTokens += *b.AnsweringTokens
		}
		if !cok || c.AnsweringTokens == nil {
			candidateTokensKnown = false
		} else {
			cTokens += *c.AnsweringTokens
		}
		if !resultResolved(b, bok) || !resultResolved(c, cok) {
			continue
		}
		p.Resolved++
		bp, cp := b.Outcome == "pass", c.Outcome == "pass"
		if bp {
			p.BaselineCompleted++
		}
		if cp {
			p.CandidateCompleted++
		}
		switch {
		case bp && cp:
			p.BothPass++
		case !bp && !cp:
			p.BothFail++
		case bp:
			p.BaselineOnly++
		default:
			p.CandidateOnly++
		}
		known := map[string]bool{}
		for _, id := range *b.CriticalErrors {
			known[id] = true
		}
		p.BaselineCritical += len(*b.CriticalErrors)
		p.CandidateCritical += len(*c.CriticalErrors)
		for _, id := range *c.CriticalErrors {
			if !known[id] {
				introduced++
			}
		}
	}
	p.Net = p.CandidateOnly - p.BaselineOnly
	if p.Resolved == p.Expected {
		p.IntroducedCritical = &introduced
		delta := 100 * float64(p.Net) / float64(p.Expected)
		p.DeltaPoints = &delta
	}
	if baselineTokensKnown {
		p.BaselineTokens = &bTokens
	}
	if candidateTokensKnown {
		p.CandidateTokens = &cTokens
	}
	return p
}

// scaledLE compares full 128-bit products; no ratio rounding or int64 overflow.
func scaledLE(a, am, b, bm int64) bool {
	ah, al := bits.Mul64(uint64(a), uint64(am))
	bh, bl := bits.Mul64(uint64(b), uint64(bm))
	return ah < bh || ah == bh && al <= bl
}

func regression(p EvaluationPairs) *bool {
	if p.Resolved != p.Expected {
		return nil
	}
	bad := p.BaselineOnly-p.CandidateOnly >= 2
	return &bad
}

func primaryResult(tasks []EvaluationTask, rows map[EvaluationAnswer]EvaluationResult) EvaluationPrimary {
	var within, cross []EvaluationTask
	for _, task := range tasks {
		if task.Stratum == "within" {
			within = append(within, task)
		} else {
			cross = append(cross, task)
		}
	}
	o := compareResultPairs(tasks, "0", "none", "A", "none", rows)
	w, c := compareResultPairs(within, "0", "none", "A", "none", rows), compareResultPairs(cross, "0", "none", "A", "none", rows)
	p := EvaluationPrimary{Overall: o, Within: EvaluationStratum{w, regression(w)}, Cross: EvaluationStratum{c, regression(c)}, AFloor: "incomplete", ZeroFloor: "incomplete", NumericCriteria: "incomplete", AccessWins: []string{}}
	for _, task := range cross {
		b, bok := rows[EvaluationAnswer{task.ID, "0", "none"}]
		a, aok := rows[EvaluationAnswer{task.ID, "A", "none"}]
		if resultResolved(b, bok) && resultResolved(a, aok) && b.Outcome == "fail" && a.Outcome == "pass" && !*task.BaselineCanAccess {
			p.AccessWins = append(p.AccessWins, task.ID)
		}
	}
	if o.Resolved != 30 {
		return p
	}
	p.AFloor, p.ZeroFloor = "not_met", "not_met"
	if o.CandidateCompleted >= 21 {
		p.AFloor = "met"
	}
	if o.BaselineCompleted >= 21 {
		p.ZeroFloor = "met"
	}
	if o.BaselineTokens == nil || o.CandidateTokens == nil {
		return p
	}
	withinLimit := scaledLE(*o.CandidateTokens, 4, *o.BaselineTokens, 5)
	p.TokenIncreaseWithinLimit = &withinLimit
	p.NumericCriteria = "not_met"
	if p.AFloor == "met" && o.Net >= 3 && *o.IntroducedCritical == 0 && !*p.Within.MaterialRegression && !*p.Cross.MaterialRegression && withinLimit {
		p.NumericCriteria = "met"
	}
	return p
}

func historyResult(p EvaluationPairs) string {
	if p.Resolved != p.Expected {
		return "incomplete"
	}
	if *p.IntroducedCritical != 0 {
		return "not_triggered"
	}
	if p.Net >= 1 {
		return "triggered"
	}
	if p.Net != 0 {
		return "not_triggered"
	}
	if p.BaselineTokens == nil || p.CandidateTokens == nil {
		return "incomplete"
	}
	if *p.BaselineTokens > 0 && scaledLE(*p.CandidateTokens, 5, *p.BaselineTokens, 4) {
		return "triggered"
	}
	return "not_triggered"
}

// ReportEvaluation accounts for declarations only. Its numeric criteria and
// rerun trigger neither grade an answer nor accept a run or authorize execution.
func ReportEvaluation(s EvaluationSnapshot, expectedDigest string, in EvaluationResults) (EvaluationResultsReport, error) {
	if _, err := CheckEvaluationAgainst(s, expectedDigest); err != nil {
		return EvaluationResultsReport{}, err
	}
	canonical, rows, err := canonicalResults(s, in)
	if err != nil {
		return EvaluationResultsReport{}, err
	}
	body, err := json.Marshal(canonical)
	if err != nil {
		return EvaluationResultsReport{}, ErrResults
	}
	hash := sha256.Sum256(body)
	r := EvaluationResultsReport{SchemaVersion: 1, State: "incomplete_base_metadata", SnapshotDigest: s.Digest, ResultsDigest: hex.EncodeToString(hash[:]),
		Base: resultCoverage(s.Base, rows), Conditional: resultCoverage(s.Conditional, rows), Primary: primaryResult(s.Definition.Tasks, rows), Diagnostics: map[string]EvaluationDiagnostic{}, PolicyLoss: map[string]EvaluationPolicyLoss{}}
	var diagnostic, within, cross []EvaluationTask
	for _, task := range s.Definition.Tasks {
		if task.Diagnostic {
			diagnostic = append(diagnostic, task)
			if task.Stratum == "within" {
				within = append(within, task)
			} else {
				cross = append(cross, task)
			}
		}
	}
	for _, arm := range []string{"B", "C", "D", "F"} {
		corpus := "full"
		if arm == "B" {
			corpus = "none"
		}
		if arm == "C" {
			corpus = "reference_prose"
		}
		p := compareResultPairs(diagnostic, "A", "none", arm, corpus, rows)
		trigger := "not_applicable"
		if arm == "D" || arm == "F" {
			trigger = historyResult(p)
		}
		r.Diagnostics[arm] = EvaluationDiagnostic{p, trigger}
	}
	for _, arm := range []string{"D", "F"} {
		r.PolicyLoss[arm] = EvaluationPolicyLoss{compareResultPairs(diagnostic, arm, "full", arm, "eligible_origin", rows), compareResultPairs(within, arm, "full", arm, "eligible_origin", rows), compareResultPairs(cross, arm, "full", arm, "eligible_origin", rows)}
	}
	d, f := r.Diagnostics["D"].HistoryTrigger, r.Diagnostics["F"].HistoryTrigger
	r.HistoryTrigger = "not_triggered"
	if d == "triggered" || f == "triggered" {
		r.HistoryTrigger = "triggered"
	} else if d == "incomplete" || f == "incomplete" {
		r.HistoryTrigger = "incomplete"
	}
	if r.HistoryTrigger != "incomplete" {
		required := r.HistoryTrigger == "triggered"
		r.ConditionalRequired = &required
	}
	if r.HistoryTrigger != "triggered" {
		r.UnexpectedConditional = r.Conditional.Present
	}
	if r.Base.complete() {
		switch {
		case r.HistoryTrigger == "incomplete": // missing usage cannot count as zero
		case r.HistoryTrigger == "triggered" && !r.Conditional.complete():
			r.State = "conditional_metadata_required"
		case r.UnexpectedConditional > 0:
			r.State = "unexpected_conditional_metadata"
		default:
			r.State = "metadata_complete"
		}
	}
	return r, nil
}

func ReadEvaluationResults(path string) (EvaluationResults, error) {
	var in EvaluationResults
	if readEvaluation(path, &in) != nil {
		return in, ErrResults
	}
	return in, nil
}
