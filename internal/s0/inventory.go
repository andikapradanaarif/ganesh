// Package s0 provides local preparation checks for the frozen S0 protocol.
// It does not establish provenance or authorize a production content lane.
package s0

import (
	"errors"
	"fmt"
)

type Target struct {
	ID        string   `json:"id"`
	Mixed     bool     `json:"mixed"`
	Unknown   bool     `json:"unknown"`
	LessonIDs []string `json:"eligible_lesson_ids"`
}

// Lesson records the screener's evidence references, not a machine-verified
// assertion of provenance. Production capture requires separate runtime gates.
type Lesson struct {
	ID        string   `json:"id"`
	Project   string   `json:"project"`
	Automatic bool     `json:"automatic"`
	Evidence  []string `json:"eligibility_evidence_refs"`
}

type FollowOn struct {
	ID        string   `json:"id"`
	TargetIDs []string `json:"target_ids"`
	Stratum   string   `json:"stratum"`
}

type Inventory struct {
	Targets      []Target   `json:"targets"`
	Lessons      []Lesson   `json:"lessons"`
	Tasks        []FollowOn `json:"applicable_tasks"`
	RecheckCount int        `json:"recheck_count"`
}

type Counts struct {
	H int `json:"H"`
	M int `json:"M"`
	U int `json:"U"`
	E int `json:"E"`
}

type Ratios struct {
	Mixed        float64 `json:"M_over_H"`
	Unknown      float64 `json:"U_over_H"`
	MixedUnknown float64 `json:"M_plus_U_over_H"`
	Eligible     float64 `json:"E_over_H"`
}

type Decision struct {
	Counts                     Counts   `json:"counts"`
	Ratios                     *Ratios  `json:"ratios"`
	Route                      string   `json:"route"`
	Reasons                    []string `json:"reasons"`
	Lessons                    int      `json:"eligible_lessons"`
	Projects                   int      `json:"eligible_projects"`
	WithinTasks                int      `json:"within_project_tasks"`
	CrossTasks                 int      `json:"cross_project_tasks"`
	ProductionNarrativeEnabled bool     `json:"production_narrative_enabled"`
}

// Evaluate applies A1's fixed routing rule to a supplied evidence inventory.
// final means the one permitted pre-S0b-2 recheck, not another capture extension.
func Evaluate(in Inventory, final bool) (Decision, error) {
	d := Decision{Route: "inconclusive", Reasons: []string{}}
	if in.RecheckCount < 0 || in.RecheckCount > 1 {
		return d, errors.New("recheck_count must be zero or one")
	}
	lessons := make(map[string]Lesson)
	for _, l := range in.Lessons {
		if l.ID == "" || l.Project == "" || len(l.Evidence) == 0 {
			return d, errors.New("each lesson needs an ID, project and eligibility evidence references")
		}
		if _, exists := lessons[l.ID]; exists {
			return d, fmt.Errorf("duplicate lesson ID %q", l.ID)
		}
		for _, ref := range l.Evidence {
			if ref == "" {
				return d, errors.New("empty eligibility evidence reference")
			}
		}
		lessons[l.ID] = l
	}
	targets := make(map[string]bool)
	usedLessons := make(map[string]bool)
	projects := make(map[string]bool)
	for _, t := range in.Targets {
		if t.ID == "" {
			return d, errors.New("target ID is required")
		}
		if _, exists := targets[t.ID]; exists {
			return d, fmt.Errorf("duplicate target ID %q", t.ID)
		}
		d.Counts.H++
		if t.Mixed {
			d.Counts.M++
		} else if t.Unknown {
			d.Counts.U++
		}
		eligible := false
		for _, id := range t.LessonIDs {
			l, ok := lessons[id]
			if !ok {
				return d, fmt.Errorf("target %q references missing lesson %q", t.ID, id)
			}
			if t.Mixed || t.Unknown || !l.Automatic {
				return d, errors.New("eligible lessons cannot cover mixed/unknown targets or use owner/synthetic derivation")
			}
			eligible = true
			usedLessons[id] = true
			projects[l.Project] = true
		}
		targets[t.ID] = eligible
		if eligible {
			d.Counts.E++
		}
	}
	d.Lessons, d.Projects = len(usedLessons), len(projects)
	taskIDs := make(map[string]bool)
	for _, task := range in.Tasks {
		if task.ID == "" || taskIDs[task.ID] {
			return d, errors.New("applicable task IDs must be nonempty and distinct")
		}
		taskIDs[task.ID] = true
		if len(task.TargetIDs) == 0 {
			return d, errors.New("applicable tasks must reference frozen targets")
		}
		covered := true
		for _, id := range task.TargetIDs {
			eligible, ok := targets[id]
			if !ok {
				return d, fmt.Errorf("task references missing target %q", id)
			}
			covered = covered && eligible
		}
		switch task.Stratum {
		case "within_project":
			if covered {
				d.WithinTasks++
			}
		case "cross_project":
			if covered {
				d.CrossTasks++
			}
		default:
			return d, errors.New("task stratum must be within_project or cross_project")
		}
	}
	if d.Counts.H == 0 {
		d.Reasons = append(d.Reasons, "empty inventory; origin ratios cannot be assessed")
		return d, nil
	}
	h := float64(d.Counts.H)
	d.Ratios = &Ratios{float64(d.Counts.M) / h, float64(d.Counts.U) / h, float64(d.Counts.M+d.Counts.U) / h, float64(d.Counts.E) / h}
	if 2*d.Counts.M >= d.Counts.H {
		d.Route = "a2_3_decision"
		d.Reasons = append(d.Reasons, "M/H >= 50%; separate scope, funding and budget required; S0 continues")
		return d, nil
	}
	if !final {
		d.Route = "single_recheck_pending"
		d.Reasons = append(d.Reasons, "below early mixed-origin threshold; allow the single final eligibility recheck")
		return d, nil
	}
	if in.RecheckCount != 1 {
		return d, errors.New("final evaluation requires recheck_count=1")
	}
	if d.Lessons < 6 {
		d.Reasons = append(d.Reasons, "fewer than six distinct needed eligible automatic lessons")
	}
	if d.Projects < 2 {
		d.Reasons = append(d.Reasons, "fewer than two eligible projects")
	}
	if 2*d.Counts.E < d.Counts.H {
		d.Reasons = append(d.Reasons, "E/H < 50%")
	}
	if d.WithinTasks < 3 || d.CrossTasks < 3 {
		d.Reasons = append(d.Reasons, "six covered applicable tasks required: three within-project and three cross-project")
	}
	if len(d.Reasons) != 0 {
		d.Route = "a2_3_decision"
	} else {
		d.Route = "first_party_probe"
		d.Reasons = append(d.Reasons, "eligibility inventory permits the bounded S0b-2 probe; production gates remain disabled")
	}
	return d, nil
}
