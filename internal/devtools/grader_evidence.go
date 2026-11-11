package devtools

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

var ErrGraderEvidence = errors.New("private bounded grader evidence required")

const evidenceFileLimit = 64 << 10

// This four-criterion profile is a proposed future calibration format. It does
// not reinterpret any saved label packet or model response from an earlier run.
type evidenceRequirement struct {
	ID          string `json:"id"`
	Requirement string `json:"requirement"`
}

type evidenceFrame struct {
	Version  int                   `json:"schema_version"`
	Task     string                `json:"task_id"`
	Case     string                `json:"case_id"`
	Answer   string                `json:"answer"`
	Criteria []evidenceRequirement `json:"criteria"`
}

type evidenceSpan struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Quote string `json:"quote"`
}

type evidenceCriterion struct {
	ID          string         `json:"id"`
	Status      string         `json:"status"`
	Spans       []evidenceSpan `json:"spans"`
	Explanation string         `json:"explanation"`
}

type evidenceResponse struct {
	Version     int                 `json:"schema_version"`
	Task        string              `json:"task_id"`
	Case        string              `json:"case_id"`
	Frame       string              `json:"answer_frame_sha256"`
	Completion  *bool               `json:"completion"`
	Critical    *bool               `json:"introduced_critical_error"`
	Unsupported *bool               `json:"unsupported_claim"`
	Criteria    []evidenceCriterion `json:"criteria"`
	Notes       string              `json:"notes"`
}

type GraderEvidenceReport struct {
	Status                    string `json:"status"`
	ResponseSchemaVersion     int    `json:"response_schema_version"`
	Criteria                  int    `json:"criteria"`
	Met                       int    `json:"met"`
	Unmet                     int    `json:"unmet"`
	Unresolved                int    `json:"unresolved"`
	VerifiedSpans             int    `json:"verified_spans"`
	DerivedCompletion         *bool  `json:"derived_completion"`
	IntroducedCriticalError   *bool  `json:"introduced_critical_error"`
	UnsupportedClaim          *bool  `json:"unsupported_claim"`
	FrameSHA256               string `json:"frame_sha256"`
	ResponseSHA256            string `json:"response_sha256"`
	SemanticRelevanceVerified bool   `json:"semantic_relevance_verified"`
	CalibrationAccepted       bool   `json:"calibration_accepted"`
	ModelDispatchAuthorized   bool   `json:"model_dispatch_authorized"`
}

func evidenceDigest(s string) bool {
	return len(s) == 64 && strings.Trim(s, "0123456789abcdef") == ""
}

// decodeStudy already rejects duplicate, unknown and case-aliased keys. Require
// non-null fields here; each response version separately requires its nullable
// judgment fields to be present.
func evidenceFields(body []byte, nonnull ...string) bool {
	return corpusRequired(body, nonnull...)
}

func evidenceID(id string) bool {
	return len(id) <= 128 && calibrationTaskID.MatchString(id)
}

func evidenceCriteria(body []byte) []json.RawMessage {
	var object map[string]json.RawMessage
	var criteria []json.RawMessage
	if json.Unmarshal(body, &object) != nil || json.Unmarshal(object["criteria"], &criteria) != nil {
		return nil
	}
	return criteria
}

func evidenceCriterionID(id string) bool {
	return id == "C1" || id == "C2" || id == "C3" || id == "C4"
}

func decodeEvidenceFrame(body []byte) (evidenceFrame, error) {
	var frame evidenceFrame
	if len(body) > evidenceFileLimit || !utf8.Valid(body) || decodeStudy(body, &frame) != nil || !evidenceFields(body, "schema_version", "task_id", "case_id", "answer", "criteria") || frame.Version != 1 || !evidenceID(frame.Task) || !evidenceID(frame.Case) || strings.TrimSpace(frame.Answer) == "" || len(frame.Answer) > 32<<10 || len(frame.Criteria) != 4 {
		return evidenceFrame{}, ErrGraderEvidence
	}
	seen := make(map[string]bool)
	raw := evidenceCriteria(body)
	for i, criterion := range frame.Criteria {
		if !evidenceFields(raw[i], "id", "requirement") || !evidenceCriterionID(criterion.ID) || seen[criterion.ID] || strings.TrimSpace(criterion.Requirement) == "" || len(criterion.Requirement) > 1024 {
			return evidenceFrame{}, ErrGraderEvidence
		}
		seen[criterion.ID] = true
	}
	return frame, nil
}

func checkEvidenceSpans(answer string, spans []evidenceSpan, raw []json.RawMessage) (int, error) {
	if len(spans) > 4 || len(spans) != len(raw) {
		return 0, ErrGraderEvidence
	}
	ranges := make(map[[2]int]bool)
	for i, span := range spans {
		if !evidenceFields(raw[i], "start", "end", "quote") || span.Start < 0 || span.End <= span.Start || span.End > len(answer) || len(span.Quote) > 2048 || !utf8.ValidString(answer[span.Start:span.End]) || span.Quote != answer[span.Start:span.End] || ranges[[2]int{span.Start, span.End}] {
			return 0, ErrGraderEvidence
		}
		ranges[[2]int{span.Start, span.End}] = true
	}
	return len(spans), nil
}

func deriveEvidenceCompletion(unmet, unresolved int, critical *bool) *bool {
	if unmet > 0 || (critical != nil && *critical) {
		v := false
		return &v
	}
	if unresolved == 0 && critical != nil {
		v := true
		return &v
	}
	return nil
}

func checkGraderEvidence(frameBody, responseBody []byte) (report GraderEvidenceReport, resultErr error) {
	defer func() {
		if resultErr != nil {
			report = GraderEvidenceReport{}
		}
	}()
	if len(frameBody) > evidenceFileLimit || len(responseBody) > evidenceFileLimit || !utf8.Valid(frameBody) || !utf8.Valid(responseBody) {
		return report, ErrGraderEvidence
	}
	frame, err := decodeEvidenceFrame(frameBody)
	if err != nil {
		return report, ErrGraderEvidence
	}
	// This peek chooses a decoder only. Its complete strict decode below must
	// still reject duplicate, aliased, unknown, missing and malformed fields.
	var header struct {
		Version int `json:"schema_version"`
	}
	if json.Unmarshal(responseBody, &header) == nil && header.Version == 3 {
		return checkGraderObservations(frameBody, frame, responseBody)
	}
	var response evidenceResponse
	if decodeStudy(responseBody, &response) != nil || !evidenceFields(responseBody, "schema_version", "task_id", "case_id", "answer_frame_sha256", "criteria", "notes") || response.Task != frame.Task || response.Case != frame.Case || response.Frame != devinDigest(frameBody) || len(response.Criteria) != 4 || len(response.Notes) > 4096 {
		return report, ErrGraderEvidence
	}
	// Version 1 requires the redundant supplied completion, including explicit
	// null. Version 2 forbids that field: only the application derives it.
	var object map[string]json.RawMessage
	if json.Unmarshal(responseBody, &object) != nil {
		return report, ErrGraderEvidence
	}
	_, completionPresent := object["completion"]
	// These counts pin the schema: decodeStudy permits only evidenceResponse's
	// nine JSON fields; version 2 omits completion. Adding a field to that struct
	// requires revisiting these version contracts and their strictness tests.
	switch response.Version {
	case 1:
		if len(object) != 9 || !completionPresent {
			return report, ErrGraderEvidence
		}
	case 2:
		if len(object) != 8 || completionPresent {
			return report, ErrGraderEvidence
		}
	default:
		return report, ErrGraderEvidence
	}
	seen := make(map[string]bool)
	rawCriteria := evidenceCriteria(responseBody)
	for i, criterion := range response.Criteria {
		if !evidenceFields(rawCriteria[i], "id", "status", "spans", "explanation") || !evidenceCriterionID(criterion.ID) || seen[criterion.ID] || len(criterion.Explanation) > 1024 || len(criterion.Spans) > 4 {
			return report, ErrGraderEvidence
		}
		seen[criterion.ID] = true
		switch criterion.Status {
		case "met":
			if len(criterion.Spans) == 0 {
				return report, ErrGraderEvidence
			}
			report.Met++
		case "unmet", "unresolved":
			if strings.TrimSpace(criterion.Explanation) == "" {
				return report, ErrGraderEvidence
			}
			if criterion.Status == "unmet" {
				report.Unmet++
			} else {
				report.Unresolved++
			}
		default:
			return report, ErrGraderEvidence
		}
		var raw struct {
			Spans []json.RawMessage `json:"spans"`
		}
		if json.Unmarshal(rawCriteria[i], &raw) != nil {
			return report, ErrGraderEvidence
		}
		count, err := checkEvidenceSpans(frame.Answer, criterion.Spans, raw.Spans)
		if err != nil {
			return report, ErrGraderEvidence
		}
		report.VerifiedSpans += count
	}
	derived := deriveEvidenceCompletion(report.Unmet, report.Unresolved, response.Critical)
	if response.Version == 1 && ((derived == nil) != (response.Completion == nil) || (derived != nil && *derived != *response.Completion)) {
		return report, ErrGraderEvidence
	}
	report.Status, report.Criteria = "mechanically_checked", 4
	report.ResponseSchemaVersion = response.Version
	report.DerivedCompletion, report.IntroducedCriticalError, report.UnsupportedClaim = derived, response.Critical, response.Unsupported
	report.FrameSHA256, report.ResponseSHA256 = devinDigest(frameBody), devinDigest(responseBody)
	// Quote membership and consistent declarations cannot establish relevance.
	// All semantic/acceptance/dispatch fields deliberately remain false.
	return report, nil
}

// CheckGraderEvidence reads two caller-pinned closed snapshots. It never opens
// sources, reference labels or credentials, runs models, or judges relevance.
func CheckGraderEvidence(directory, expectedFrame, expectedResponse string) (GraderEvidenceReport, error) {
	if !evidenceDigest(expectedFrame) || !evidenceDigest(expectedResponse) {
		return GraderEvidenceReport{}, ErrGraderEvidence
	}
	directory = filepath.Clean(directory)
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return GraderEvidenceReport{}, ErrGraderEvidence
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return GraderEvidenceReport{}, ErrGraderEvidence
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) || opened.Mode().Perm() != 0o700 {
		return GraderEvidenceReport{}, ErrGraderEvidence
	}
	frame, err := privateFile(root, "answer-frame.json", evidenceFileLimit)
	if err != nil || devinDigest(frame) != expectedFrame {
		return GraderEvidenceReport{}, ErrGraderEvidence
	}
	response, err := privateFile(root, "grader-evidence.json", evidenceFileLimit)
	if err != nil || devinDigest(response) != expectedResponse {
		return GraderEvidenceReport{}, ErrGraderEvidence
	}
	return checkGraderEvidence(frame, response)
}
