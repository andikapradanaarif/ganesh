package s0

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// TargetInventory holds screener declarations, not verified source facts.
// Definitions are frozen separately from assessments so classification can
// evolve without changing which high-priority needs form the denominator.
type TargetInventory struct {
	SchemaVersion int            `json:"schema_version"`
	State         string         `json:"state"`
	Targets       []ScreenTarget `json:"targets"`
	Lessons       []ScreenLesson `json:"lessons,omitempty"`
}

type NeedDefinition struct {
	ID   string `json:"id"`
	Need string `json:"need"`
}

type ScreenTarget struct {
	NeedDefinition
	Assessed   bool     `json:"assessed"`
	Mixed      bool     `json:"mixed"`
	Unknown    bool     `json:"unknown"`
	OwnerOnly  bool     `json:"owner_only"`
	ReasonRefs []string `json:"reason_refs"`
	// All listed lessons must qualify to declare this need completely covered.
	LessonIDs []string `json:"required_lesson_ids"`
}

type ScreenLesson struct {
	ID           string   `json:"id"`
	Project      string   `json:"project"`
	Derivation   string   `json:"derivation"`
	Origin       string   `json:"origin"`
	Assurance    string   `json:"assurance"`
	Eligibility  string   `json:"eligibility"`
	EvidenceRefs []string `json:"eligibility_evidence_refs"`
}

type NeedSnapshot struct {
	SchemaVersion int              `json:"schema_version"`
	Targets       []NeedDefinition `json:"targets"`
}

type OriginCounts struct {
	H int `json:"H"`
	M int `json:"M"`
	U int `json:"U"`
	E int `json:"E"`
}
type OriginRatios struct {
	Mixed        float64 `json:"M_over_H"`
	Unknown      float64 `json:"U_over_H"`
	MixedUnknown float64 `json:"M_plus_U_over_H"`
	Eligible     float64 `json:"E_over_H"`
}

type TargetAssessment struct {
	ID             string   `json:"id"`
	Mixed          bool     `json:"mixed"`
	Unknown        bool     `json:"unknown"`
	Eligible       bool     `json:"eligible"`
	ReasonRefCount int      `json:"reason_ref_count"`
	Reasons        []string `json:"reasons"`
}

type ScreenReport struct {
	Counts                        OriginCounts       `json:"counts"`
	Ratios                        *OriginRatios      `json:"ratios"`
	NeedDigest                    *string            `json:"frozen_need_digest"`
	Route                         string             `json:"route"`
	Reasons                       []string           `json:"reasons"`
	Assessments                   []TargetAssessment `json:"target_assessments"`
	EligibleLessons               int                `json:"distinct_needed_eligible_lessons"`
	EligibleProjects              int                `json:"distinct_eligible_projects"`
	EvidenceIndependentlyVerified bool               `json:"evidence_independently_verified"`
	ProductionNarrativeEnabled    bool               `json:"production_narrative_enabled"`
	ModelRunsAuthorized           bool               `json:"model_runs_authorized"`
}

func NewTargetInventory() TargetInventory {
	return TargetInventory{SchemaVersion: 1, State: "not_screened", Targets: []ScreenTarget{}}
}

func definitions(targets []ScreenTarget) ([]NeedDefinition, error) {
	result := make([]NeedDefinition, 0, len(targets))
	seen := map[string]bool{}
	for _, target := range targets {
		if !nonblank(target.ID) || !nonblank(target.Need) || seen[target.ID] {
			return nil, errors.New("need definitions require distinct nonempty IDs and descriptions")
		}
		seen[target.ID] = true
		result = append(result, target.NeedDefinition)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

// FreezeNeeds preserves exact identifiers/descriptions; ordering alone is
// canonicalized. It neither examines nor approves origin assessments.
func FreezeNeeds(in TargetInventory) (NeedSnapshot, error) {
	if in.SchemaVersion != 1 {
		return NeedSnapshot{}, errors.New("unsupported target inventory schema")
	}
	needs, err := definitions(in.Targets)
	if err != nil {
		return NeedSnapshot{}, err
	}
	if len(needs) == 0 {
		return NeedSnapshot{}, errors.New("cannot freeze an empty need inventory")
	}
	return NeedSnapshot{1, needs}, nil
}

func (s NeedSnapshot) Digest() (string, error) {
	if s.SchemaVersion != 1 {
		return "", errors.New("unsupported need snapshot schema")
	}
	// Validate snapshot identity too, rather than trusting a supplied digest.
	targets := make([]ScreenTarget, 0, len(s.Targets))
	for _, need := range s.Targets {
		targets = append(targets, ScreenTarget{NeedDefinition: need})
	}
	canonical, err := FreezeNeeds(TargetInventory{SchemaVersion: 1, Targets: targets})
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(canonical)
	if err != nil {
		return "", errors.New("cannot encode frozen need definitions")
	}
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:]), nil
}

// SaveNeedSnapshot never overwrites a previous snapshot. The digest binds
// definitions only, not the truth or completeness of source assessments.
func SaveNeedSnapshot(path string, snapshot NeedSnapshot) error {
	if _, err := snapshot.Digest(); err != nil {
		return err
	}
	return writePrivate(path, snapshot)
}

func InitialScreen(in TargetInventory, baseline *NeedSnapshot) (ScreenReport, error) {
	r := ScreenReport{Route: "inconclusive", Reasons: []string{}, Assessments: []TargetAssessment{}}
	if in.SchemaVersion != 1 {
		return r, errors.New("unsupported target inventory schema")
	}
	if in.State != "not_screened" && in.State != "screened" {
		return r, errors.New("inventory state must be not_screened or screened")
	}
	needs, err := definitions(in.Targets)
	if err != nil {
		return r, err
	}
	if len(needs) > 0 {
		if baseline == nil {
			return r, errors.New("freeze need definitions and supply the snapshot before counting origins")
		}
		expected, err := baseline.Digest()
		if err != nil {
			return r, err
		}
		current := NeedSnapshot{1, needs}
		actual, err := current.Digest()
		if err != nil {
			return r, err
		}
		if actual != expected {
			return r, errors.New("need definitions differ from the supplied frozen inventory")
		}
		r.NeedDigest = &actual
	} else if baseline != nil {
		return r, errors.New("empty inventory cannot replace supplied frozen needs")
	}
	lessons := map[string]ScreenLesson{}
	for _, lesson := range in.Lessons {
		if !nonblank(lesson.ID) || !nonblank(lesson.Project) {
			return r, errors.New("lesson IDs and projects are required")
		}
		if _, exists := lessons[lesson.ID]; exists {
			return r, errors.New("duplicate lesson identity")
		}
		switch lesson.Eligibility {
		case "eligible":
			if lesson.Derivation != "automatic" || lesson.Origin != "first_party_only" || !validRefs(lesson.EvidenceRefs) || (lesson.Assurance != "server_observed" && lesson.Assurance != "client_attested") {
				return r, errors.New("eligible automatic lessons need first-party origin, admitted assurance and explicit evidence references")
			}
		case "ineligible", "unknown":
		default:
			return r, errors.New("lesson eligibility must be eligible, ineligible or unknown")
		}
		lessons[lesson.ID] = lesson
	}
	if in.State == "not_screened" && len(lessons) > 0 {
		return r, errors.New("unscreened inventory cannot contain lesson assessments")
	}
	used := map[string]bool{}
	projects := map[string]bool{}
	assessed := 0
	for _, target := range in.Targets {
		hasAssessment := target.Assessed || target.Mixed || target.Unknown || target.OwnerOnly
		if hasAssessment && !validRefs(target.ReasonRefs) {
			return r, errors.New("origin assessments need explicit reason references")
		}
		if in.State == "not_screened" && (hasAssessment || len(target.LessonIDs) > 0) {
			return r, errors.New("unscreened inventory cannot contain origin assessments")
		}
		a := TargetAssessment{ID: target.ID, Mixed: target.Mixed, Unknown: !target.Assessed || target.Unknown, ReasonRefCount: len(target.ReasonRefs), Reasons: []string{}}
		covered := len(target.LessonIDs) > 0
		seen := map[string]bool{}
		for _, id := range target.LessonIDs {
			lesson, exists := lessons[id]
			if !exists {
				return r, errors.New("target references an absent lesson")
			}
			if seen[id] {
				return r, errors.New("target repeats a required lesson identity")
			}
			seen[id] = true
			covered = covered && lesson.Eligibility == "eligible"
		}
		a.Eligible = covered && !a.Mixed && !a.Unknown && !target.OwnerOnly
		r.Counts.H++
		if a.Mixed {
			r.Counts.M++
			a.Reasons = append(a.Reasons, "known_mixed_origin")
		} else if a.Unknown {
			r.Counts.U++
		}
		if a.Unknown {
			a.Reasons = append(a.Reasons, "unknown_or_unassessed_origin_coverage")
		}
		if target.OwnerOnly {
			a.Reasons = append(a.Reasons, "owner_only_need")
		}
		if !covered {
			a.Reasons = append(a.Reasons, "eligible_automatic_coverage_missing")
		}
		if a.Eligible {
			r.Counts.E++
			a.Reasons = append(a.Reasons, "declared_eligible_automatic_coverage")
			for _, id := range target.LessonIDs {
				used[id] = true
				projects[lessons[id].Project] = true
			}
		}
		if hasAssessment {
			assessed++
		}
		r.Assessments = append(r.Assessments, a)
	}
	sort.Slice(r.Assessments, func(i, j int) bool { return r.Assessments[i].ID < r.Assessments[j].ID })
	r.EligibleLessons = len(used)
	r.EligibleProjects = len(projects)
	if r.Counts.H == 0 {
		r.Reasons = append(r.Reasons, "empty_need_inventory")
		return r, nil
	}
	h := float64(r.Counts.H)
	r.Ratios = &OriginRatios{float64(r.Counts.M) / h, float64(r.Counts.U) / h, float64(r.Counts.M+r.Counts.U) / h, float64(r.Counts.E) / h}
	if 2*r.Counts.M >= r.Counts.H {
		r.Route = "a2_3_decision"
		r.Reasons = append(r.Reasons, "M/H >= 50%; separate funding/scope/budget decision required; S0 continues")
	} else if assessed == 0 {
		r.Reasons = append(r.Reasons, "no_assessable_origin_evidence")
	} else {
		r.Route = "single_recheck_pending"
		r.Reasons = append(r.Reasons, "below early mixed-origin threshold; final first-party eligibility recheck is still required")
	}
	return r, nil
}

func nonblank(value string) bool { return strings.TrimSpace(value) != "" }
func validRefs(refs []string) bool {
	if len(refs) == 0 {
		return false
	}
	for _, ref := range refs {
		if !nonblank(ref) {
			return false
		}
	}
	return true
}
