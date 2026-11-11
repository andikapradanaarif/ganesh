package devtools

import (
	"encoding/json"
	"strings"
)

// Schema 3 separates declared positive content from declared whole-answer
// defeat. Neither declaration is mechanically established as semantically true.
type evidenceObservation struct {
	ID                  string         `json:"id"`
	Coverage            string         `json:"coverage"`
	SupportingSpans     []evidenceSpan `json:"supporting_spans"`
	CoverageExplanation string         `json:"coverage_explanation"`
	Defeat              string         `json:"defeat"`
	ContrarySpans       []evidenceSpan `json:"contrary_spans"`
	DefeatExplanation   string         `json:"defeat_explanation"`
}

type evidenceObservationResponse struct {
	Version     int                   `json:"schema_version"`
	Task        string                `json:"task_id"`
	Case        string                `json:"case_id"`
	Frame       string                `json:"answer_frame_sha256"`
	Critical    *bool                 `json:"introduced_critical_error"`
	Unsupported *bool                 `json:"unsupported_claim"`
	Criteria    []evidenceObservation `json:"criteria"`
	Notes       string                `json:"notes"`
}

// Only combine caller-supplied enums. This function cannot see the answer,
// quoted text, notes, expected outcomes or critical/unsupported judgments.
func deriveObservationStatus(coverage, defeat string) (string, error) {
	if coverage != "covered" && coverage != "missing" && coverage != "unknown" {
		return "", ErrGraderEvidence
	}
	if defeat != "present" && defeat != "absent" && defeat != "unknown" {
		return "", ErrGraderEvidence
	}
	if coverage == "missing" || defeat == "present" {
		return "unmet", nil
	}
	if coverage == "unknown" || defeat == "unknown" {
		return "unresolved", nil
	}
	return "met", nil
}

func checkGraderObservations(frameBody []byte, frame evidenceFrame, responseBody []byte) (GraderEvidenceReport, error) {
	var response evidenceObservationResponse
	if decodeStudy(responseBody, &response) != nil || !evidenceFields(responseBody, "schema_version", "task_id", "case_id", "answer_frame_sha256", "criteria", "notes") || response.Version != 3 || response.Task != frame.Task || response.Case != frame.Case || response.Frame != devinDigest(frameBody) || len(response.Criteria) != 4 || len(response.Notes) > 4096 {
		return GraderEvidenceReport{}, ErrGraderEvidence
	}
	var object map[string]json.RawMessage
	// Eight schema-3 fields are permitted by the strict struct decode, so this
	// also requires both nullable judgments. Revisit if that key universe changes.
	if json.Unmarshal(responseBody, &object) != nil || len(object) != 8 {
		return GraderEvidenceReport{}, ErrGraderEvidence
	}
	var report GraderEvidenceReport
	seen := make(map[string]bool)
	rawCriteria := evidenceCriteria(responseBody)
	for i, criterion := range response.Criteria {
		if !evidenceFields(rawCriteria[i], "id", "coverage", "supporting_spans", "coverage_explanation", "defeat", "contrary_spans", "defeat_explanation") || !evidenceCriterionID(criterion.ID) || seen[criterion.ID] || len(criterion.CoverageExplanation) > 1024 || len(criterion.DefeatExplanation) > 1024 || strings.TrimSpace(criterion.DefeatExplanation) == "" {
			return GraderEvidenceReport{}, ErrGraderEvidence
		}
		seen[criterion.ID] = true
		status, err := deriveObservationStatus(criterion.Coverage, criterion.Defeat)
		if err != nil || (criterion.Coverage == "covered" && len(criterion.SupportingSpans) == 0) || (criterion.Coverage != "covered" && strings.TrimSpace(criterion.CoverageExplanation) == "") || (criterion.Defeat == "present" && len(criterion.ContrarySpans) == 0) || (criterion.Defeat == "absent" && len(criterion.ContrarySpans) != 0) {
			return GraderEvidenceReport{}, ErrGraderEvidence
		}
		var raw struct {
			Supporting []json.RawMessage `json:"supporting_spans"`
			Contrary   []json.RawMessage `json:"contrary_spans"`
		}
		if json.Unmarshal(rawCriteria[i], &raw) != nil {
			return GraderEvidenceReport{}, ErrGraderEvidence
		}
		for _, lane := range []struct {
			spans []evidenceSpan
			raw   []json.RawMessage
		}{{criterion.SupportingSpans, raw.Supporting}, {criterion.ContrarySpans, raw.Contrary}} {
			count, err := checkEvidenceSpans(frame.Answer, lane.spans, lane.raw)
			if err != nil {
				return GraderEvidenceReport{}, ErrGraderEvidence
			}
			report.VerifiedSpans += count
		}
		switch status {
		case "met":
			report.Met++
		case "unmet":
			report.Unmet++
		case "unresolved":
			report.Unresolved++
		}
	}
	report.Status, report.ResponseSchemaVersion, report.Criteria = "mechanically_checked", 3, 4
	report.DerivedCompletion = deriveEvidenceCompletion(report.Unmet, report.Unresolved, response.Critical)
	report.IntroducedCriticalError, report.UnsupportedClaim = response.Critical, response.Unsupported
	report.FrameSHA256, report.ResponseSHA256 = devinDigest(frameBody), devinDigest(responseBody)
	// All relevance/acceptance/dispatch flags remain false, even when both lanes
	// contain exact quotations. No missing semantic observation is inferred.
	return report, nil
}
