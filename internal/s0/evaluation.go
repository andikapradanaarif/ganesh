package s0

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
)

var ErrEvaluation = errors.New("invalid or incomplete S0 evaluation plan")

var (
	evaluationID    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$`)
	evaluationRunID = regexp.MustCompile(`^s0-[a-f0-9]{32}$`)
	evaluationHash  = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// Bindings are declared hashes of private, pre-run artifacts. Their contents,
// permission, truth, independence and full-context fit are not verified here.
type EvaluationBindings struct {
	Corpus         string `json:"corpus_sha256"`
	EligibleCorpus string `json:"eligible_corpus_and_filter_sha256"`
	Baseline       string `json:"native_instructions_and_notes_sha256"`
	TypedCards     string `json:"typed_cards_sha256"`
	Variants       string `json:"arm_variants_and_renderer_sha256"`
	Permissions    string `json:"permissions_and_access_map_sha256"`
	ModelsLimits   string `json:"models_and_limits_sha256"`
	Calibration    string `json:"calibration_sha256"`
	Grading        string `json:"oracle_grader_and_spot_checks_sha256"`
	CleanSessions  string `json:"clean_session_setup_sha256"`
	FullContext    string `json:"full_context_preflight_sha256"`
}

type EvaluationTask struct {
	ID                string   `json:"id"`
	Role              string   `json:"role"`
	Stratum           string   `json:"stratum"`
	Diagnostic        bool     `json:"diagnostic"`
	Tags              []string `json:"tags"`
	Prompt            string   `json:"prompt_sha256"`
	Rubric            string   `json:"rubric_sha256"`
	Oracle            string   `json:"source_keyed_oracle_sha256"`
	Sources           string   `json:"permitted_sources_sha256"`
	BaselineCanAccess *bool    `json:"baseline_can_access_required_information"`
}

type EvaluationDefinition struct {
	SchemaVersion  int                `json:"schema_version"`
	RunID          string             `json:"run_id"`
	Seed           *uint32            `json:"schedule_seed"`
	Bindings       EvaluationBindings `json:"bindings"`
	CalibrationIDs []string           `json:"calibration_task_ids"`
	Tasks          []EvaluationTask   `json:"tasks"`
}

type EvaluationAnswer struct {
	TaskID string `json:"task_id"`
	Arm    string `json:"arm"`
	Corpus string `json:"summary_corpus"`
}

type EvaluationSnapshot struct {
	SchemaVersion int                  `json:"schema_version"`
	Algorithm     string               `json:"schedule_algorithm"`
	Definition    EvaluationDefinition `json:"definition"`
	Base          []EvaluationAnswer   `json:"base_answers"`
	Conditional   []EvaluationAnswer   `json:"conditional_eligible_origin_answers"`
	Digest        string               `json:"snapshot_digest"`
}

type EvaluationReport struct {
	State                         string `json:"state"`
	Digest                        string `json:"snapshot_digest"`
	Tasks                         int    `json:"tasks"`
	Within                        int    `json:"within_project_tasks"`
	Cross                         int    `json:"cross_project_tasks"`
	Diagnostics                   int    `json:"diagnostic_tasks"`
	BaseAnswers                   int    `json:"base_answers"`
	ConditionalAnswers            int    `json:"conditional_answers"`
	MaximumAnswers                int    `json:"maximum_answers"`
	Trigger                       string `json:"eligible_origin_trigger"`
	EvidenceIndependentlyVerified bool   `json:"evidence_independently_verified"`
	ModelRunsAuthorized           bool   `json:"model_runs_authorized"`
	ProductionNarrativeEnabled    bool   `json:"production_narrative_enabled"`
}

func NewEvaluationDefinition(runID string) EvaluationDefinition {
	return EvaluationDefinition{SchemaVersion: 1, RunID: runID, CalibrationIDs: []string{}, Tasks: []EvaluationTask{}}
}

func canonicalEvaluation(in EvaluationDefinition) (EvaluationDefinition, error) {
	if in.SchemaVersion != 1 || !evaluationRunID.MatchString(in.RunID) || in.Seed == nil || len(in.Tasks) != 30 || len(in.CalibrationIDs) == 0 || len(in.CalibrationIDs) > 30 {
		return EvaluationDefinition{}, ErrEvaluation
	}
	b := in.Bindings
	for _, value := range []string{b.Corpus, b.EligibleCorpus, b.Baseline, b.TypedCards, b.Variants, b.Permissions, b.ModelsLimits, b.Calibration, b.Grading, b.CleanSessions, b.FullContext} {
		if !evaluationHash.MatchString(value) {
			return EvaluationDefinition{}, ErrEvaluation
		}
	}
	type counts struct{ within, cross, diagnosticWithin, diagnosticCross int }
	got := map[string]counts{"coder": {}, "planner": {}, "reviewer": {}}
	seen := make(map[string]bool)
	calibration := append([]string(nil), in.CalibrationIDs...)
	for _, id := range calibration {
		if !evaluationID.MatchString(id) || seen[id] {
			return EvaluationDefinition{}, ErrEvaluation
		}
		seen[id] = true
	}
	allowedTags := map[string]bool{"changed_state": true, "workflow": true, "gotcha": true, "mistaken_premise": true, "abstention": true, "local_exception": true, "irrelevant_memory": true}
	tasks := make([]EvaluationTask, 0, 30)
	seenInputs := make(map[[3]string]bool)
	for _, task := range in.Tasks {
		c, validRole := got[task.Role]
		if !evaluationID.MatchString(task.ID) || seen[task.ID] || !validRole || task.BaselineCanAccess == nil || len(task.Tags) == 0 || len(task.Tags) > len(allowedTags) {
			return EvaluationDefinition{}, ErrEvaluation
		}
		seen[task.ID] = true
		for _, value := range []string{task.Prompt, task.Rubric, task.Oracle, task.Sources} {
			if !evaluationHash.MatchString(value) {
				return EvaluationDefinition{}, ErrEvaluation
			}
		}
		inputKey := [3]string{task.Role, task.Prompt, task.Sources}
		if seenInputs[inputKey] {
			return EvaluationDefinition{}, ErrEvaluation
		}
		seenInputs[inputKey] = true
		switch task.Stratum {
		case "within":
			c.within++
			if task.Diagnostic {
				c.diagnosticWithin++
			}
		case "cross":
			c.cross++
			if task.Diagnostic {
				c.diagnosticCross++
			}
		default:
			return EvaluationDefinition{}, ErrEvaluation
		}
		got[task.Role] = c
		tags := append([]string(nil), task.Tags...)
		sort.Strings(tags)
		for i, tag := range tags {
			if !allowedTags[tag] || (i > 0 && tags[i-1] == tag) {
				return EvaluationDefinition{}, ErrEvaluation
			}
		}
		task.Tags = tags
		baseline := *task.BaselineCanAccess
		task.BaselineCanAccess = &baseline
		tasks = append(tasks, task)
	}
	want := map[string]counts{"coder": {7, 3, 2, 2}, "planner": {7, 3, 2, 2}, "reviewer": {6, 4, 2, 2}}
	for role, counts := range want {
		if got[role] != counts {
			return EvaluationDefinition{}, ErrEvaluation
		}
	}
	sort.Strings(calibration)
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	in.CalibrationIDs, in.Tasks = calibration, tasks
	seed := *in.Seed
	in.Seed = &seed
	return in, nil
}

// scheduleRank uses length-delimited identities and a versioned purpose. Input
// array order and platform PRNG implementations cannot change the schedule.
func scheduleRank(seed uint32, purpose, taskID string) [32]byte {
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], seed)
	h := sha256.New()
	h.Write([]byte("ganesh-s0-schedule-v1\x00"))
	h.Write(encoded[:])
	for _, value := range []string{purpose, taskID} {
		binary.BigEndian.PutUint32(encoded[:], uint32(len(value)))
		h.Write(encoded[:])
		h.Write([]byte(value))
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

func rankedTasks(d EvaluationDefinition, stratum, purpose string, diagnostic bool) []EvaluationTask {
	var tasks []EvaluationTask
	for _, task := range d.Tasks {
		if task.Stratum == stratum && (!diagnostic || task.Diagnostic) {
			tasks = append(tasks, task)
		}
	}
	sort.Slice(tasks, func(i, j int) bool {
		a, b := scheduleRank(*d.Seed, purpose, tasks[i].ID), scheduleRank(*d.Seed, purpose, tasks[j].ID)
		if c := bytes.Compare(a[:], b[:]); c != 0 {
			return c < 0
		}
		return tasks[i].ID < tasks[j].ID
	})
	return tasks
}

// FreezeEvaluation creates planning evidence only. It does not inspect any
// referenced artifact or authorize even a single held-out model answer.
func FreezeEvaluation(in EvaluationDefinition) (EvaluationSnapshot, error) {
	d, err := canonicalEvaluation(in)
	if err != nil {
		return EvaluationSnapshot{}, err
	}
	s := EvaluationSnapshot{SchemaVersion: 1, Algorithm: "sha256-order-v1", Definition: d, Base: []EvaluationAnswer{}, Conditional: []EvaluationAnswer{}}
	for _, stratum := range []string{"within", "cross"} {
		for i, task := range rankedTasks(d, stratum, "primary", false) {
			arms := []string{"0", "A"}
			if i%2 == 1 {
				arms[0], arms[1] = arms[1], arms[0]
			}
			for _, arm := range arms {
				s.Base = append(s.Base, EvaluationAnswer{task.ID, arm, "none"})
			}
		}
	}
	arms := []string{"B", "C", "D", "F"}
	for stratumIndex, stratum := range []string{"within", "cross"} {
		for i, task := range rankedTasks(d, stratum, "diagnostic", true) {
			for j := range 4 {
				arm := arms[(i+2*stratumIndex+j)%4]
				corpus := "none"
				if arm == "C" {
					corpus = "reference_prose"
				}
				if arm == "D" || arm == "F" {
					corpus = "full"
				}
				s.Base = append(s.Base, EvaluationAnswer{task.ID, arm, corpus})
			}
			filteredArms := []string{"D", "F"}
			if i%2 == 1 {
				filteredArms[0], filteredArms[1] = filteredArms[1], filteredArms[0]
			}
			for _, arm := range filteredArms {
				s.Conditional = append(s.Conditional, EvaluationAnswer{task.ID, arm, "eligible_origin"})
			}
		}
	}
	body, err := json.Marshal(s) // digest field is empty while computing identity
	if err != nil {
		return EvaluationSnapshot{}, ErrEvaluation
	}
	digest := sha256.Sum256(body)
	s.Digest = hex.EncodeToString(digest[:])
	return s, nil
}

func CheckEvaluation(s EvaluationSnapshot) (EvaluationReport, error) {
	expected, err := FreezeEvaluation(s.Definition)
	if err != nil {
		return EvaluationReport{}, err
	}
	actualBody, err := json.Marshal(s)
	if err != nil {
		return EvaluationReport{}, ErrEvaluation
	}
	expectedBody, err := json.Marshal(expected)
	if err != nil || !bytes.Equal(actualBody, expectedBody) {
		return EvaluationReport{}, ErrEvaluation
	}
	return EvaluationReport{State: "evaluation_snapshot_valid", Digest: expected.Digest, Tasks: 30, Within: 20, Cross: 10, Diagnostics: 12,
		BaseAnswers: len(expected.Base), ConditionalAnswers: len(expected.Conditional), MaximumAnswers: len(expected.Base) + len(expected.Conditional), Trigger: "not_evaluated"}, nil
}

// CheckEvaluationAgainst also compares the separately retained freeze output.
// A self-consistent replacement is a different plan, not the original baseline.
func CheckEvaluationAgainst(s EvaluationSnapshot, expectedDigest string) (EvaluationReport, error) {
	if !evaluationHash.MatchString(expectedDigest) || s.Digest != expectedDigest {
		return EvaluationReport{}, ErrEvaluation
	}
	return CheckEvaluation(s)
}

func SaveEvaluationSnapshot(path string, s EvaluationSnapshot) error {
	if _, err := CheckEvaluation(s); err != nil {
		return err
	}
	return writePrivate(path, s)
}
