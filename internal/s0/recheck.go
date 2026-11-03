package s0

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
)

var ErrRecheck = errors.New("invalid or incomplete eligibility recheck metadata")
var ErrRecheckInput = errors.New("cannot read final eligibility recheck metadata")
var ErrRecheckIncomplete = errors.New("declared first-party eligibility gate not met")

// Flat fields intentionally match the screen wire format without embedding:
// the exact-shape reader requires every key, including empty arrays.
type RecheckTarget struct {
	ID         string   `json:"id"`
	Need       string   `json:"need"`
	Assessed   bool     `json:"assessed"`
	Mixed      bool     `json:"mixed"`
	Unknown    bool     `json:"unknown"`
	OwnerOnly  bool     `json:"owner_only"`
	ReasonRefs []string `json:"reason_refs"`
	LessonIDs  []string `json:"required_lesson_ids"`
}

type RecheckInventory struct {
	SchemaVersion int             `json:"schema_version"`
	State         string          `json:"state"`
	Targets       []RecheckTarget `json:"targets"`
	Lessons       []ScreenLesson  `json:"lessons"`
}

// These six applicability tasks are separate from S0's 30-task schedule.
type RecheckTask struct {
	ID      string   `json:"id"`
	Stratum string   `json:"stratum"`
	NeedIDs []string `json:"required_need_ids"`
	Prompt  string   `json:"prompt_sha256"`
	Rubric  string   `json:"source_keyed_rubric_sha256"`
	Sources string   `json:"permitted_sources_sha256"`
}

type RecheckDefinition struct {
	SchemaVersion int              `json:"schema_version"`
	RunID         string           `json:"run_id"`
	Initial       RecheckInventory `json:"initial_inventory"`
	Tasks         []RecheckTask    `json:"applicable_tasks"`
}

type RecheckSnapshot struct {
	SchemaVersion int               `json:"schema_version"`
	Algorithm     string            `json:"algorithm"`
	NeedDigest    string            `json:"frozen_need_digest"`
	Definition    RecheckDefinition `json:"definition"`
	Digest        string            `json:"snapshot_digest"`
}

type RecheckTaskEvidence struct {
	ID       string  `json:"task_id"`
	Status   string  `json:"applicability"` // supported, unsupported, unknown
	Evidence *string `json:"evidence_sha256"`
}

type RecheckInput struct {
	SchemaVersion  int                   `json:"schema_version"`
	SnapshotDigest string                `json:"snapshot_digest"`
	Final          RecheckInventory      `json:"final_inventory"`
	Tasks          []RecheckTaskEvidence `json:"task_evidence"`
}

type RecheckClassification struct {
	Assessed  bool     `json:"assessed"`
	Mixed     bool     `json:"mixed"`
	Unknown   bool     `json:"unknown"`
	OwnerOnly bool     `json:"owner_only"`
	Eligible  bool     `json:"eligible"`
	Lessons   []string `json:"required_lesson_ids"`
}

type RecheckChange struct {
	ID     string                `json:"target_id"`
	Before RecheckClassification `json:"before"`
	After  RecheckClassification `json:"after"`
}

type RecheckOrigin struct {
	Counts   OriginCounts  `json:"counts"`
	Ratios   *OriginRatios `json:"ratios"`
	Lessons  int           `json:"distinct_needed_eligible_lessons"`
	Projects int           `json:"distinct_eligible_projects"`
}

type RecheckTaskResult struct {
	ID              string `json:"task_id"`
	Stratum         string `json:"stratum"`
	Declared        string `json:"declared_applicability"`
	EvidencePresent bool   `json:"evidence_present"`
	NeedsEligible   bool   `json:"required_needs_eligible"`
	Supported       bool   `json:"declared_support_complete"`
}

type RecheckReport struct {
	SchemaVersion                 int                 `json:"schema_version"`
	State                         string              `json:"state"`
	Route                         string              `json:"route"`
	SnapshotDigest                string              `json:"snapshot_digest"`
	InputDigest                   string              `json:"input_digest"`
	InitialRoute                  string              `json:"initial_route"`
	Initial                       RecheckOrigin       `json:"initial"`
	Final                         RecheckOrigin       `json:"final"`
	Changes                       []RecheckChange     `json:"classification_changes"`
	Tasks                         []RecheckTaskResult `json:"applicable_tasks"`
	Within                        int                 `json:"supported_within_project_tasks"`
	Cross                         int                 `json:"supported_cross_project_tasks"`
	SixLessons                    bool                `json:"six_needed_eligible_lessons"`
	TwoProjects                   bool                `json:"two_eligible_projects"`
	HalfNeeds                     bool                `json:"eligible_need_coverage_at_least_half"`
	SixTasks                      bool                `json:"six_applicable_tasks_supported"`
	Reasons                       []string            `json:"reasons"`
	EvidenceIndependentlyVerified bool                `json:"evidence_independently_verified"`
	ExperimentAccepted            bool                `json:"experiment_accepted"`
	ModelRunsAuthorized           bool                `json:"model_runs_authorized"`
	ProductionNarrativeEnabled    bool                `json:"production_narrative_enabled"`
}

func (in RecheckInventory) screenInventory() TargetInventory {
	targets := make([]ScreenTarget, 0, len(in.Targets))
	for _, t := range in.Targets {
		targets = append(targets, ScreenTarget{NeedDefinition{t.ID, t.Need}, t.Assessed, t.Mixed, t.Unknown, t.OwnerOnly, t.ReasonRefs, t.LessonIDs})
	}
	return TargetInventory{in.SchemaVersion, in.State, targets, in.Lessons}
}

func recheckStrings(in []string, limit, width int, ids bool) ([]string, error) {
	if len(in) > limit {
		return nil, ErrRecheck
	}
	out := append([]string{}, in...)
	sort.Strings(out)
	for i, v := range out {
		if !nonblank(v) || len(v) > width || ids && !evaluationID.MatchString(v) || i > 0 && v == out[i-1] {
			return nil, ErrRecheck
		}
	}
	return out, nil
}

func canonicalRecheckInventory(in RecheckInventory) (RecheckInventory, error) {
	if in.SchemaVersion != 1 || len(in.Targets) == 0 || len(in.Targets) > 1024 || len(in.Lessons) > 2048 {
		return RecheckInventory{}, ErrRecheck
	}
	in.Targets = append([]RecheckTarget{}, in.Targets...)
	in.Lessons = append([]ScreenLesson{}, in.Lessons...)
	for i, t := range in.Targets {
		if !evaluationID.MatchString(t.ID) || !nonblank(t.Need) || len(t.Need) > 8192 {
			return RecheckInventory{}, ErrRecheck
		}
		var err error
		t.ReasonRefs, err = recheckStrings(t.ReasonRefs, 32, 1024, false)
		if err != nil {
			return RecheckInventory{}, err
		}
		t.LessonIDs, err = recheckStrings(t.LessonIDs, 64, 80, true)
		if err != nil {
			return RecheckInventory{}, err
		}
		in.Targets[i] = t
	}
	for i, l := range in.Lessons {
		if !evaluationID.MatchString(l.ID) || !nonblank(l.Project) || len(l.Project) > 160 {
			return RecheckInventory{}, ErrRecheck
		}
		for _, value := range []string{l.Derivation, l.Origin, l.Assurance} {
			if !nonblank(value) || len(value) > 80 {
				return RecheckInventory{}, ErrRecheck
			}
		}
		var err error
		l.EvidenceRefs, err = recheckStrings(l.EvidenceRefs, 32, 1024, false)
		if err != nil {
			return RecheckInventory{}, err
		}
		in.Lessons[i] = l
	}
	sort.Slice(in.Targets, func(i, j int) bool { return in.Targets[i].ID < in.Targets[j].ID })
	sort.Slice(in.Lessons, func(i, j int) bool { return in.Lessons[i].ID < in.Lessons[j].ID })
	// Reuse declaration-consistency rules, not a source/origin verifier.
	inv := in.screenInventory()
	needs, err := FreezeNeeds(inv)
	if err != nil {
		return RecheckInventory{}, ErrRecheck
	}
	if _, err := InitialScreen(inv, &needs); err != nil {
		return RecheckInventory{}, ErrRecheck
	}
	return in, nil
}

func FreezeRecheck(d RecheckDefinition, expectedNeedDigest string) (RecheckSnapshot, error) {
	if d.SchemaVersion != 1 || !evaluationRunID.MatchString(d.RunID) || !evaluationHash.MatchString(expectedNeedDigest) || len(d.Tasks) != 6 {
		return RecheckSnapshot{}, ErrRecheck
	}
	var err error
	d.Initial, err = canonicalRecheckInventory(d.Initial)
	if err != nil {
		return RecheckSnapshot{}, err
	}
	needs, err := FreezeNeeds(d.Initial.screenInventory())
	if err != nil {
		return RecheckSnapshot{}, ErrRecheck
	}
	digest, err := needs.Digest()
	if err != nil {
		return RecheckSnapshot{}, ErrRecheck
	}
	if digest != expectedNeedDigest {
		return RecheckSnapshot{}, ErrRecheck
	}
	known := map[string]bool{}
	for _, t := range d.Initial.Targets {
		known[t.ID] = true
	}
	seen, inputs := map[string]bool{}, map[[2]string]bool{}
	d.Tasks = append([]RecheckTask{}, d.Tasks...)
	within, cross := 0, 0
	for i, task := range d.Tasks {
		key := [2]string{task.Prompt, task.Sources}
		if !evaluationID.MatchString(task.ID) || seen[task.ID] || inputs[key] || !evaluationHash.MatchString(task.Prompt) || !evaluationHash.MatchString(task.Rubric) || !evaluationHash.MatchString(task.Sources) {
			return RecheckSnapshot{}, ErrRecheck
		}
		seen[task.ID], inputs[key] = true, true
		task.NeedIDs, err = recheckStrings(task.NeedIDs, 1024, 80, true)
		if err != nil || len(task.NeedIDs) == 0 {
			return RecheckSnapshot{}, ErrRecheck
		}
		for _, id := range task.NeedIDs {
			if !known[id] {
				return RecheckSnapshot{}, ErrRecheck
			}
		}
		switch task.Stratum {
		case "within":
			within++
		case "cross":
			cross++
		default:
			return RecheckSnapshot{}, ErrRecheck
		}
		d.Tasks[i] = task
	}
	if within != 3 || cross != 3 {
		return RecheckSnapshot{}, ErrRecheck
	}
	sort.Slice(d.Tasks, func(i, j int) bool { return d.Tasks[i].ID < d.Tasks[j].ID })
	s := RecheckSnapshot{SchemaVersion: 1, Algorithm: "ganesh-s0-recheck-v1", NeedDigest: digest, Definition: d}
	body, err := json.Marshal(s)
	if err != nil {
		return RecheckSnapshot{}, ErrRecheck
	}
	hash := sha256.Sum256(body)
	s.Digest = hex.EncodeToString(hash[:])
	return s, nil
}

func CheckRecheckAgainst(s RecheckSnapshot, expectedDigest string) error {
	if !evaluationHash.MatchString(expectedDigest) || s.Digest != expectedDigest {
		return ErrRecheck
	}
	want, err := FreezeRecheck(s.Definition, s.NeedDigest)
	if err != nil {
		return ErrRecheck
	}
	a, err := json.Marshal(s)
	if err != nil {
		return ErrRecheck
	}
	b, err := json.Marshal(want)
	if err != nil || !bytes.Equal(a, b) {
		return ErrRecheck
	}
	return nil
}

func SaveRecheckSnapshot(path string, s RecheckSnapshot) error {
	if CheckRecheckAgainst(s, s.Digest) != nil {
		return ErrRecheck
	}
	body, err := json.MarshalIndent(s, "", "  ")
	if err != nil || len(body)+1 > 1<<20 {
		return ErrRecheck
	}
	if writePrivate(path, s) != nil {
		return ErrRecheck
	}
	return nil
}

func ReadRecheckDefinition(path string) (RecheckDefinition, error) {
	var in RecheckDefinition
	if readEvaluation(path, &in) != nil {
		return in, ErrRecheck
	}
	return in, nil
}
func ReadRecheckSnapshot(path string) (RecheckSnapshot, error) {
	var in RecheckSnapshot
	if readEvaluation(path, &in) != nil {
		return in, ErrRecheck
	}
	return in, nil
}
func ReadRecheckInput(path string) (RecheckInput, error) {
	var in RecheckInput
	if readEvaluation(path, &in) != nil {
		return in, ErrRecheckInput
	}
	return in, nil
}

// Join by identity: screen assessment order is independent of inventory order.
// Missing, foreign or duplicate identities cannot produce a classification.
func recheckClassifications(targets []RecheckTarget, assessments []TargetAssessment) (map[string]RecheckClassification, error) {
	if len(targets) != len(assessments) {
		return nil, ErrRecheck
	}
	byID := make(map[string]TargetAssessment, len(assessments))
	for _, a := range assessments {
		if _, exists := byID[a.ID]; exists {
			return nil, ErrRecheck
		}
		byID[a.ID] = a
	}
	classes := make(map[string]RecheckClassification, len(targets))
	for _, t := range targets {
		a, exists := byID[t.ID]
		if !exists {
			return nil, ErrRecheck
		}
		if _, duplicate := classes[t.ID]; duplicate {
			return nil, ErrRecheck
		}
		classes[t.ID] = RecheckClassification{t.Assessed, a.Mixed, a.Unknown, t.OwnerOnly, a.Eligible, append([]string{}, t.LessonIDs...)}
	}
	return classes, nil
}

// ReportRecheck computes declared eligibility only. The caller still needs the
// actual collection window, independent evidence and relevant safety gates.
func ReportRecheck(s RecheckSnapshot, expectedDigest string, in RecheckInput) (RecheckReport, error) {
	if CheckRecheckAgainst(s, expectedDigest) != nil || in.SchemaVersion != 1 || in.SnapshotDigest != s.Digest || len(in.Tasks) > 6 {
		return RecheckReport{}, ErrRecheck
	}
	var err error
	in.Final, err = canonicalRecheckInventory(in.Final)
	if err != nil {
		return RecheckReport{}, err
	}
	needs, err := FreezeNeeds(s.Definition.Initial.screenInventory())
	if err != nil {
		return RecheckReport{}, ErrRecheck
	}
	before, err := InitialScreen(s.Definition.Initial.screenInventory(), &needs)
	if err != nil {
		return RecheckReport{}, ErrRecheck
	}
	after, err := InitialScreen(in.Final.screenInventory(), &needs)
	if err != nil {
		return RecheckReport{}, ErrRecheck
	}
	beforeClasses, err := recheckClassifications(s.Definition.Initial.Targets, before.Assessments)
	if err != nil {
		return RecheckReport{}, err
	}
	afterClasses, err := recheckClassifications(in.Final.Targets, after.Assessments)
	if err != nil {
		return RecheckReport{}, err
	}
	known := map[string]bool{}
	for _, task := range s.Definition.Tasks {
		known[task.ID] = true
	}
	evidence := map[string]RecheckTaskEvidence{}
	seenEvidence := map[string]bool{}
	in.Tasks = append([]RecheckTaskEvidence{}, in.Tasks...)
	for i, t := range in.Tasks {
		if !known[t.ID] {
			return RecheckReport{}, ErrRecheck
		}
		if _, exists := evidence[t.ID]; exists {
			return RecheckReport{}, ErrRecheck
		}
		if t.Status != "supported" && t.Status != "unsupported" && t.Status != "unknown" {
			return RecheckReport{}, ErrRecheck
		}
		if t.Evidence != nil {
			if !evaluationHash.MatchString(*t.Evidence) || seenEvidence[*t.Evidence] {
				return RecheckReport{}, ErrRecheck
			}
			seenEvidence[*t.Evidence] = true
			copy := *t.Evidence
			t.Evidence = &copy
		}
		if t.Status == "supported" && t.Evidence == nil {
			return RecheckReport{}, ErrRecheck
		}
		evidence[t.ID], in.Tasks[i] = t, t
	}
	sort.Slice(in.Tasks, func(i, j int) bool { return in.Tasks[i].ID < in.Tasks[j].ID })
	body, err := json.Marshal(in)
	if err != nil {
		return RecheckReport{}, ErrRecheck
	}
	hash := sha256.Sum256(body)
	origin := func(r ScreenReport) RecheckOrigin {
		return RecheckOrigin{r.Counts, r.Ratios, r.EligibleLessons, r.EligibleProjects}
	}
	r := RecheckReport{SchemaVersion: 1, State: "inconclusive", Route: "inconclusive", SnapshotDigest: s.Digest, InputDigest: hex.EncodeToString(hash[:]), InitialRoute: before.Route, Initial: origin(before), Final: origin(after), Changes: []RecheckChange{}, Tasks: []RecheckTaskResult{}, Reasons: []string{}}
	eligible := map[string]bool{}
	for _, t := range in.Final.Targets {
		b, a := beforeClasses[t.ID], afterClasses[t.ID]
		eligible[t.ID] = a.Eligible
		if !reflect.DeepEqual(b, a) {
			r.Changes = append(r.Changes, RecheckChange{t.ID, b, a})
		}
	}
	for _, task := range s.Definition.Tasks {
		e, exists := evidence[task.ID]
		if !exists {
			e.Status = "unknown"
		}
		covered := true
		for _, id := range task.NeedIDs {
			covered = covered && eligible[id]
		}
		supported := e.Status == "supported" && e.Evidence != nil && covered
		r.Tasks = append(r.Tasks, RecheckTaskResult{task.ID, task.Stratum, e.Status, e.Evidence != nil, covered, supported})
		if supported {
			if task.Stratum == "within" {
				r.Within++
			} else {
				r.Cross++
			}
		}
	}
	r.SixLessons = after.EligibleLessons >= 6
	r.TwoProjects = after.EligibleProjects >= 2
	r.HalfNeeds = 2*after.Counts.E >= after.Counts.H
	r.SixTasks = r.Within == 3 && r.Cross == 3
	// The day-one route is binding; later clean declarations cannot undo it.
	if before.Route == "a2_3_decision" {
		r.Reasons = append(r.Reasons, "initial_mixed_origin_route_locked")
	} else if before.Route == "inconclusive" {
		r.Reasons = append(r.Reasons, "initial_screen_inconclusive")
		return r, nil
	} else if after.Route == "inconclusive" {
		r.Reasons = append(r.Reasons, "final_screen_inconclusive")
		return r, nil
	}
	if after.Route == "a2_3_decision" {
		r.Reasons = append(r.Reasons, "final_mixed_origin_threshold")
	}
	if !r.SixLessons {
		r.Reasons = append(r.Reasons, "fewer_than_six_needed_eligible_lessons")
	}
	if !r.TwoProjects {
		r.Reasons = append(r.Reasons, "fewer_than_two_eligible_projects")
	}
	if !r.HalfNeeds {
		r.Reasons = append(r.Reasons, "eligible_need_coverage_below_half")
	}
	if !r.SixTasks {
		r.Reasons = append(r.Reasons, "applicable_tasks_unsupported")
	}
	if len(r.Reasons) > 0 {
		r.State = "declared_gate_not_met"
		r.Route = "a2_3_decision"
	} else {
		r.State = "declared_criteria_met"
		r.Route = "first_party_probe_candidate"
	}
	return r, nil
}
