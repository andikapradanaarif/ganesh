package s0

// RunInputs contains preparation evidence only. The CLI has no operation for
// launching answers, so filling these fields does not run or authorize a model.
type RunInputs struct {
	Projects             []string  `json:"permitted_personal_project_paths"`
	Client               *string   `json:"client_build"`
	Installation         *string   `json:"installation"`
	ConfigRevision       *string   `json:"capture_config_revision"`
	TranscriptAccess     *string   `json:"transcript_access_evidence"`
	AnsweringModel       *string   `json:"answering_model_id"`
	GradingModel         *string   `json:"grading_model_id_and_family"`
	SpendUSD             *float64  `json:"total_api_spend_cap_usd"`
	AnswerTimeoutSeconds *int      `json:"per_answer_timeout_seconds"`
	OutputReasoningCaps  *string   `json:"output_and_reasoning_caps"`
	CalibrationRetries   *string   `json:"calibration_and_retry_allowance"`
	FrozenEvidence       *string   `json:"frozen_corpus_tasks_rubrics_permissions_arms_order_seeds_and_digest"`
	CaptureMeasurements  *string   `json:"capture_cold_start_latency_failures_and_session_end_measurements"`
	A23                  A23Budget `json:"optional_a2_3"`
}

type A23Budget struct {
	Days             *float64 `json:"focused_day_cap"`
	SpendUSD         *float64 `json:"spend_cap_usd"`
	DecisionDeadline *string  `json:"decision_deadline"`
	Decision         string   `json:"decision"`
}

func NewRunInputs() RunInputs {
	return RunInputs{Projects: []string{}, A23: A23Budget{Decision: "pending"}}
}

// Missing reports unknown run inputs. A2.3 never blocks S0 preparation.
// Presence checks do not independently verify permissions or frozen evidence.
func (r RunInputs) Missing() []string {
	missing := []string{}
	if len(r.Projects) < 3 || len(r.Projects) > 5 {
		missing = append(missing, "3–5 permitted personal project paths")
	}
	seen := map[string]bool{}
	for _, p := range r.Projects {
		if p == "" || seen[p] {
			missing = append(missing, "nonempty distinct project paths")
			break
		}
		seen[p] = true
	}
	for _, item := range []struct {
		name  string
		value *string
	}{
		{"client/build", r.Client}, {"installation", r.Installation}, {"capture config revision", r.ConfigRevision},
		{"transcript access evidence", r.TranscriptAccess}, {"answering model ID", r.AnsweringModel},
		{"grading model ID/family", r.GradingModel}, {"output/reasoning caps", r.OutputReasoningCaps},
		{"calibration/retry allowance", r.CalibrationRetries}, {"frozen evidence", r.FrozenEvidence},
		{"capture measurements", r.CaptureMeasurements},
	} {
		if item.value == nil || *item.value == "" {
			missing = append(missing, item.name)
		}
	}
	if r.SpendUSD == nil || *r.SpendUSD < 0 {
		missing = append(missing, "nonnegative API spend cap")
	}
	if r.AnswerTimeoutSeconds == nil || *r.AnswerTimeoutSeconds <= 0 {
		missing = append(missing, "positive per-answer timeout")
	}
	return missing
}
