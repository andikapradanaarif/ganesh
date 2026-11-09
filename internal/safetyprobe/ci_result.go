package safetyprobe

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
)

var ErrCIResultProbe = errors.New("synthetic CI-result probe failed")

// Frozen before candidate implementation in issue #69. No external inputs.
//
//go:embed ci_result_v1.json
var ciFixtures []byte

const ciFixtureDigest = "21aa3c78597550950562341ebfe310d38b7b381da0139912b8d476b58c1d549b"

type ciPolicy struct {
	Source     string      `json:"source"`
	Repository string      `json:"repository"`
	Workflow   string      `json:"workflow"`
	MaxAge     int64       `json:"max_check_age_seconds"`
	Head       *headPolicy `json:"head"`
}

type ciIdentity struct {
	Repository string `json:"repository"`
	Workflow   string `json:"workflow"`
	Run        string `json:"run"`
	Attempt    int64  `json:"attempt"`
	Commit     string `json:"commit"`
	Revision   string `json:"revision"`
}

type ciClaim struct {
	ciIdentity
	Subject      string `json:"subject"`
	Scope        string `json:"scope"`
	Conclusion   string `json:"conclusion"`
	Branch       string `json:"branch"`
	HeadRevision string `json:"head_revision"`
	Environment  string `json:"environment"`
}

type ciObservation struct {
	ciIdentity
	Channel    string `json:"channel"`
	Source     string `json:"source"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Completed  int64  `json:"completed_unix"`
	Updated    int64  `json:"updated_unix"`
	Checked    int64  `json:"checked_unix"`
	Prose      string `json:"prose"` // Carried as untrusted fixture data; never used for support or copied into a result.
}

type ciOutcome struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Value  string `json:"value"`
}

// verifyCIResult receives no case names or expected outcomes. All source and
// protection declarations are synthetic premises, not authenticated API evidence.
func verifyCIResult(c ciClaim, e *ciObservation, head *headObservation, p ciPolicy, now int64) ciOutcome {
	deny := func(reason string) ciOutcome { return ciOutcome{Kind: "withheld", Reason: reason} }
	if p.Source == "" || p.Repository == "" || p.Workflow == "" || p.MaxAge <= 0 {
		return deny("invalid_policy")
	}
	if c.Subject != "ci.result" {
		return deny("unsupported_subject")
	}
	if c.Scope != "commit" && c.Scope != "current_head" {
		return deny("unsupported_scope")
	}
	if c.Environment != "" || (c.Scope == "commit" && (c.Branch != "" || c.HeadRevision != "")) {
		return deny("unsupported_qualifier")
	}
	if c.Repository != p.Repository {
		return deny("unconfigured_repository")
	}
	if c.Workflow != p.Workflow {
		return deny("unconfigured_workflow")
	}
	if e == nil {
		return deny("missing_evidence")
	}
	if e.Channel != "ci_run_api" {
		return deny("unsupported_channel")
	}
	if e.Source != p.Source {
		return deny("unconfigured_source")
	}
	if !revisionID.MatchString(c.Run) || !revisionID.MatchString(e.Run) || c.Attempt <= 0 || e.Attempt <= 0 || !headCommitID.MatchString(c.Commit) || !headCommitID.MatchString(e.Commit) || !revisionID.MatchString(c.Revision) || !revisionID.MatchString(e.Revision) {
		return deny("invalid_identity")
	}
	if c.Repository != e.Repository || c.Workflow != e.Workflow {
		return deny("object_mismatch")
	}
	if c.Run != e.Run {
		return deny("run_mismatch")
	}
	if c.Attempt != e.Attempt {
		return deny("attempt_mismatch")
	}
	if c.Commit != e.Commit {
		return deny("commit_mismatch")
	}
	if c.Revision != e.Revision {
		return deny("revision_mismatch")
	}
	if e.Status != "completed" {
		return deny("run_not_completed")
	}
	validConclusion := func(value string) bool { return value == "success" || value == "failure" }
	if !validConclusion(c.Conclusion) || !validConclusion(e.Conclusion) {
		return deny("invalid_conclusion")
	}
	if c.Conclusion != e.Conclusion {
		return deny("conclusion_mismatch")
	}
	if now <= 0 || e.Completed <= 0 || e.Updated <= 0 || e.Checked <= 0 || e.Completed > e.Updated || e.Updated > e.Checked || e.Checked > now {
		return deny("invalid_time")
	}
	if now-e.Checked > p.MaxAge {
		return deny("stale_check")
	}
	if c.Scope == "current_head" {
		if p.Head == nil {
			return deny("missing_head_policy")
		}
		if p.Head.Repository != p.Repository {
			return deny("invalid_head_policy")
		}
		claim := headClaim{Subject: "repository.head", Repository: c.Repository, Branch: c.Branch, Commit: c.Commit, Revision: c.HeadRevision}
		checked := verifyRepositoryHead(claim, head, *p.Head, now)
		if checked.Kind != "supported_head" {
			return deny("head_" + checked.Reason)
		}
		return ciOutcome{Kind: "supported_head_result", Reason: "exact_ci_result", Value: c.Conclusion}
	}
	return ciOutcome{Kind: "supported_commit_result", Reason: "exact_ci_result", Value: c.Conclusion}
}

type ciCase struct {
	Name     string           `json:"name"`
	Policy   ciPolicy         `json:"policy"`
	Now      int64            `json:"now_unix"`
	Claim    ciClaim          `json:"claim"`
	Evidence *ciObservation   `json:"evidence"`
	Head     *headObservation `json:"head"`
	Expected ciOutcome        `json:"expected"`
}

type ciFixtureSet struct {
	Schema string   `json:"schema"`
	Cases  []ciCase `json:"cases"`
}

type CIResultReport struct {
	Schema                 string `json:"schema"`
	FixtureSHA256          string `json:"fixture_sha256"`
	Status                 string `json:"status"`
	Cases                  int    `json:"cases"`
	Matched                int    `json:"matched"`
	Mismatched             int    `json:"mismatched"`
	SupportedCommitResults int    `json:"supported_commit_results"`
	SupportedHeadResults   int    `json:"supported_head_results"`
	Withheld               int    `json:"withheld"`
	Synthetic              bool   `json:"synthetic"`
	PublicationAuthorized  bool   `json:"publication_authorized"`
	StageGatePassed        bool   `json:"stage_gate_passed"`
}

type ciVerifier func(ciClaim, *ciObservation, *headObservation, ciPolicy, int64) ciOutcome

// Only this scorer sees the frozen expectations. Candidate results are compared
// as complete outcomes, including the requested commit/head qualification.
func compareCIResult(body []byte, verify ciVerifier) (CIResultReport, error) {
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	if digest != ciFixtureDigest {
		return CIResultReport{}, ErrCIResultProbe
	}
	var fixtures ciFixtureSet
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(&fixtures) != nil || fixtures.Schema != "synthetic-ci-result-v1" || len(fixtures.Cases) != 75 {
		return CIResultReport{}, ErrCIResultProbe
	}
	if d.Decode(new(any)) != io.EOF {
		return CIResultReport{}, ErrCIResultProbe
	}
	report := CIResultReport{Schema: fixtures.Schema, FixtureSHA256: digest, Cases: len(fixtures.Cases), Synthetic: true, Status: "fixture_expectations_matched"}
	for _, row := range fixtures.Cases {
		got := verify(row.Claim, row.Evidence, row.Head, row.Policy, row.Now)
		if got == row.Expected {
			report.Matched++
		} else {
			report.Mismatched++
		}
		switch got.Kind {
		case "supported_commit_result":
			report.SupportedCommitResults++
		case "supported_head_result":
			report.SupportedHeadResults++
		case "withheld":
			report.Withheld++
		}
	}
	if report.Mismatched > 0 {
		report.Status = "fixture_mismatch"
	}
	return report, nil
}

func RunCIResult() (CIResultReport, error) {
	return compareCIResult(ciFixtures, verifyCIResult)
}
