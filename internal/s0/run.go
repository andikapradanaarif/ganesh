// Package s0 contains preparation records for the bounded S0 experiment.
// It does not capture content or establish production eligibility.
package s0

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"time"
)

type Project struct {
	Path                string `json:"path"`
	LocalReadPermission string `json:"local_read_permission"`
}

type Capture struct {
	Client           *string `json:"selected_client"`
	Build            *string `json:"selected_client_build"`
	Installation     *string `json:"installation"`
	TranscriptAccess *string `json:"permitted_transcript_access_evidence"`
	ConfigRevision   *string `json:"config_revision"`
	Measurements     *string `json:"measurement_record_ref"`
	HookInstalled    bool    `json:"hook_installed"`
}

type Models struct {
	Answering     *string `json:"answering_exact_id"`
	Grading       *string `json:"grading_exact_id"`
	GradingFamily *string `json:"grading_family"`
}

type Limits struct {
	SpendUSD           *float64 `json:"total_api_spend_cap_usd"`
	AnswerSeconds      *int     `json:"per_answer_timeout_seconds"`
	OutputTokens       *int     `json:"output_token_cap"`
	Reasoning          *string  `json:"reasoning_cap"`
	CalibrationRetries *string  `json:"calibration_and_retry_allowance"`
}

type A23 struct {
	Days     *float64 `json:"focused_day_cap"`
	SpendUSD *float64 `json:"spend_cap_usd"`
	Deadline *string  `json:"decision_deadline"`
	Decision string   `json:"decision"`
}

type Run struct {
	SchemaVersion   int           `json:"schema_version"`
	ID              string        `json:"run_id"`
	CreatedAt       time.Time     `json:"created_at"`
	ScreenStartedAt *time.Time    `json:"screen_started_at"`
	Projects        []Project     `json:"projects"`
	Discovery       []Observation `json:"local_discovery"`
	Capture         Capture       `json:"capture"`
	Models          Models        `json:"models"`
	Limits          Limits        `json:"limits"`
	FrozenEvidence  *string       `json:"frozen_corpus_tasks_rubrics_permissions_variants_order_seeds_digest_ref"`
	A23             A23           `json:"optional_a2_3"`
}

func NewRun(now time.Time, observations []Observation) (Run, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return Run{}, err
	}
	if observations == nil {
		observations = []Observation{}
	}
	return Run{SchemaVersion: 1, ID: "s0-" + hex.EncodeToString(id[:]), CreatedAt: now.UTC(), Projects: []Project{}, Discovery: observations, A23: A23{Decision: "pending"}}, nil
}

type Readiness struct {
	Phase                               string   `json:"phase"`
	InputsPresent                       bool     `json:"inputs_present"`
	Missing                             []string `json:"missing"`
	Warnings                            []string `json:"warnings"`
	PermissionsIndependentlyVerified    bool     `json:"permissions_independently_verified"`
	FrozenEvidenceIndependentlyVerified bool     `json:"frozen_evidence_independently_verified"`
	ProductionNarrativeEnabled          bool     `json:"production_narrative_enabled"`
}

// Check checks declared input presence only. It never reads the named project
// or evidence paths, proves permission, verifies freezing, or starts a run.
func Check(r Run, phase string) (Readiness, error) {
	result := Readiness{Phase: phase, Missing: []string{}, Warnings: []string{}}
	if r.SchemaVersion != 1 {
		return result, errors.New("unsupported run schema")
	}
	if phase != "inventory" && phase != "heldout" {
		return result, errors.New("phase must be inventory or heldout")
	}
	if len(r.Projects) < 3 || len(r.Projects) > 5 {
		result.Missing = append(result.Missing, "3–5 selected personal projects")
	}
	seen := map[string]bool{}
	for _, p := range r.Projects {
		clean := filepath.Clean(p.Path)
		if !filepath.IsAbs(p.Path) || seen[clean] {
			result.Missing = append(result.Missing, "distinct absolute project paths")
		}
		seen[clean] = true
		if p.LocalReadPermission != "allowed" {
			result.Missing = append(result.Missing, "explicit local-read permission for every selected project")
		}
	}
	required := []struct {
		name  string
		value *string
	}{
		{"selected capture client", r.Capture.Client}, {"observed selected client build", r.Capture.Build},
		{"selected installation", r.Capture.Installation}, {"permitted transcript access evidence", r.Capture.TranscriptAccess},
	}
	if phase == "heldout" {
		required = append(required, []struct {
			name  string
			value *string
		}{
			{"capture configuration revision", r.Capture.ConfigRevision}, {"capture measurements and explicit gaps", r.Capture.Measurements},
			{"answering exact model ID", r.Models.Answering}, {"grading exact model ID", r.Models.Grading},
			{"grading model family", r.Models.GradingFamily}, {"reasoning cap", r.Limits.Reasoning},
			{"calibration/retry allowance", r.Limits.CalibrationRetries}, {"frozen experiment evidence reference", r.FrozenEvidence},
		}...)
		if r.Limits.SpendUSD == nil || *r.Limits.SpendUSD < 0 {
			result.Missing = append(result.Missing, "nonnegative API spend cap")
		}
		if r.Limits.AnswerSeconds == nil || *r.Limits.AnswerSeconds <= 0 {
			result.Missing = append(result.Missing, "positive per-answer time cap")
		}
		if r.Limits.OutputTokens == nil || *r.Limits.OutputTokens <= 0 {
			result.Missing = append(result.Missing, "positive output token cap")
		}
	}
	for _, item := range required {
		if item.value == nil || strings.TrimSpace(*item.value) == "" {
			result.Missing = append(result.Missing, item.name)
		}
	}
	// Keep diagnostics concise without changing which checks fail or their
	// first-occurrence order. Several projects can fail the same predicate.
	seenReasons := map[string]bool{}
	unique := result.Missing[:0]
	for _, reason := range result.Missing {
		if !seenReasons[reason] {
			seenReasons[reason] = true
			unique = append(unique, reason)
		}
	}
	result.Missing = unique
	result.InputsPresent = len(result.Missing) == 0
	return result, nil
}
