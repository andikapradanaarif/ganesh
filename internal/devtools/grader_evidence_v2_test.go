package devtools

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestGraderEvidenceV2KeepsUnresolvedCompletionUnknown(t *testing.T) {
	frame, response := evidenceFixture(t)
	frame.Answer = strings.Replace(frame.Answer, "Capture and origin remain unknown.", "Record current state only.", 1)
	fb := evidenceJSON(t, frame)
	response.Frame = devinDigest(fb)
	response.Criteria[3].Status = "unresolved"
	response.Criteria[3].Spans = []evidenceSpan{}
	response.Criteria[3].Explanation = "The required capture/origin distinction is missing."
	response.Completion = evidenceBool(false)
	// The observed failure shape remains invalid in the original format.
	if report, err := checkGraderEvidence(fb, evidenceJSON(t, response)); err != ErrGraderEvidence || !reflect.DeepEqual(report, GraderEvidenceReport{}) {
		t.Fatal("version 1 inconsistency retroactively accepted")
	}
	// The new format does not ask the model for completion. Its unresolved
	// judgment remains unknown, including when that judgment may itself be wrong.
	report, err := checkGraderEvidence(fb, evidenceV2JSON(t, response))
	if err != nil || report.ResponseSchemaVersion != 2 || report.Met != 3 || report.Unresolved != 1 || report.VerifiedSpans != 3 || report.DerivedCompletion != nil || report.SemanticRelevanceVerified || report.CalibrationAccepted || report.ModelDispatchAuthorized {
		t.Fatal("uncertainty converted to completeness/failure or semantic assurance")
	}
	response.Criteria[3].Status = "unmet"
	report, err = checkGraderEvidence(fb, evidenceV2JSON(t, response))
	if err != nil || report.Unmet != 1 || report.DerivedCompletion == nil || *report.DerivedCompletion || report.SemanticRelevanceVerified {
		t.Fatal("known missing coverage did not derive false")
	}
}

func TestGraderEvidenceV2RejectsMixedAndMissingFields(t *testing.T) {
	for _, name := range []string{"completion true", "completion false", "completion null", "completion alias", "missing critical", "missing unsupported", "missing notes", "null notes", "unknown field", "unknown version", "float version", "null spans"} {
		t.Run(name, func(t *testing.T) {
			frame, response := evidenceFixture(t)
			var object map[string]json.RawMessage
			if json.Unmarshal(evidenceV2JSON(t, response), &object) != nil {
				t.Fatal("fixture")
			}
			switch name {
			case "completion true":
				object["completion"] = json.RawMessage(`true`)
			case "completion false":
				object["completion"] = json.RawMessage(`false`)
			case "completion null":
				object["completion"] = json.RawMessage(`null`)
			case "completion alias":
				object["Completion"] = json.RawMessage(`null`)
			case "missing critical":
				delete(object, "introduced_critical_error")
			case "missing unsupported":
				delete(object, "unsupported_claim")
			case "missing notes":
				delete(object, "notes")
			case "null notes":
				object["notes"] = json.RawMessage(`null`)
			case "unknown field":
				object["derived_completion"] = json.RawMessage(`true`)
			case "unknown version":
				object["schema_version"] = json.RawMessage(`4`)
			case "float version":
				object["schema_version"] = json.RawMessage(`2.0`)
			case "null spans":
				var criteria []map[string]json.RawMessage
				if json.Unmarshal(object["criteria"], &criteria) != nil {
					t.Fatal("criteria fixture")
				}
				criteria[0]["spans"] = json.RawMessage(`null`)
				object["criteria"] = evidenceJSON(t, criteria)
			}
			if report, err := checkGraderEvidence(evidenceJSON(t, frame), evidenceJSON(t, object)); err != ErrGraderEvidence || !reflect.DeepEqual(report, GraderEvidenceReport{}) {
				t.Fatal("mixed/invalid version 2 layout accepted or partial data exposed")
			}
		})
	}
}
