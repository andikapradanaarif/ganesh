// Package safetyprobe contains disposable synthetic experiments, not service
// authorization or production publication code.
package safetyprobe

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
)

var ErrIssueStateProbe = errors.New("synthetic issue-state probe failed")

// Frozen before candidate implementation in issue #65. No external inputs,
// connector credentials or live observations are accepted by this experiment.
//
//go:embed issue_state_v1.json
var issueFixtures []byte

const issueFixtureDigest = "dfef7a7c5c0df95f57ec09a8e14b51fe2e1342d8c0cc57ea7eb426ecb4244298"

var revisionID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,159}$`)

type issuePolicy struct {
	Source     string `json:"source"`
	Repository string `json:"repository"`
	MaxAge     int64  `json:"max_check_age_seconds"`
}

type issueClaim struct {
	Subject    string `json:"subject"`
	Repository string `json:"repository"`
	Issue      int64  `json:"issue"`
	State      string `json:"state"`
	Revision   string `json:"revision"`
}

type issueObservation struct {
	Channel    string `json:"channel"`
	Source     string `json:"source"`
	Repository string `json:"repository"`
	Issue      int64  `json:"issue"`
	State      string `json:"state"`
	Revision   string `json:"revision"`
	Updated    int64  `json:"updated_unix"`
	Checked    int64  `json:"checked_unix"`
	Prose      string `json:"prose"`
}

type issueOutcome struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Value  string `json:"value"`
}

// verifyIssueState cannot read fixture expectations. The declared fake source
// is a consistency premise, not proof that an API was actually authenticated.
func verifyIssueState(c issueClaim, e *issueObservation, p issuePolicy, now int64) issueOutcome {
	deny := func(reason string) issueOutcome { return issueOutcome{Kind: "withheld", Reason: reason} }
	if p.Source == "" || p.Repository == "" || p.MaxAge <= 0 {
		return deny("invalid_policy")
	}
	if c.Subject != "issue.state" {
		return deny("unsupported_subject")
	}
	if c.Repository != p.Repository {
		return deny("unconfigured_repository")
	}
	if e == nil {
		return deny("missing_evidence")
	}
	if e.Channel != "issue_api_enum" {
		return deny("unsupported_channel")
	}
	if e.Source != p.Source {
		return deny("unconfigured_source")
	}
	if c.Issue <= 0 || e.Issue <= 0 || !revisionID.MatchString(c.Revision) || !revisionID.MatchString(e.Revision) {
		return deny("invalid_identity")
	}
	if c.Repository != e.Repository || c.Issue != e.Issue {
		return deny("object_mismatch")
	}
	validState := func(s string) bool { return s == "open" || s == "closed" }
	if !validState(c.State) || !validState(e.State) {
		return deny("invalid_value")
	}
	if c.State != e.State {
		return deny("value_mismatch")
	}
	if c.Revision != e.Revision {
		return deny("revision_mismatch")
	}
	if now <= 0 || e.Updated <= 0 || e.Checked <= 0 || e.Updated > e.Checked || e.Checked > now {
		return deny("invalid_time")
	}
	if now-e.Checked > p.MaxAge {
		return deny("stale_check")
	}
	return issueOutcome{Kind: "supported_enum", Reason: "exact_issue_state", Value: c.State}
}

type issueCase struct {
	Name     string            `json:"name"`
	Claim    issueClaim        `json:"claim"`
	Evidence *issueObservation `json:"evidence"`
	Expected issueOutcome      `json:"expected"`
}

type issueFixtureSet struct {
	Schema string      `json:"schema"`
	Now    int64       `json:"now_unix"`
	Policy issuePolicy `json:"policy"`
	Cases  []issueCase `json:"cases"`
}

type IssueStateReport struct {
	Schema                string `json:"schema"`
	FixtureSHA256         string `json:"fixture_sha256"`
	Status                string `json:"status"`
	Cases                 int    `json:"cases"`
	Matched               int    `json:"matched"`
	Mismatched            int    `json:"mismatched"`
	SupportedEnums        int    `json:"supported_enums"`
	Withheld              int    `json:"withheld"`
	Synthetic             bool   `json:"synthetic"`
	PublicationAuthorized bool   `json:"publication_authorized"`
	StageGatePassed       bool   `json:"stage_gate_passed"`
}

type issueVerifier func(issueClaim, *issueObservation, issuePolicy, int64) issueOutcome

// compareIssueState is separate from the candidate: only this scorer sees the
// manually recorded expectations. Tests also exercise a deliberately bad verifier.
func compareIssueState(body []byte, verify issueVerifier) (IssueStateReport, error) {
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	if digest != issueFixtureDigest {
		return IssueStateReport{}, ErrIssueStateProbe
	}
	var fixtures issueFixtureSet
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(&fixtures) != nil || fixtures.Schema != "synthetic-issue-state-v1" || len(fixtures.Cases) != 24 {
		return IssueStateReport{}, ErrIssueStateProbe
	}
	if d.Decode(new(any)) != io.EOF {
		return IssueStateReport{}, ErrIssueStateProbe
	}
	report := IssueStateReport{Schema: fixtures.Schema, FixtureSHA256: digest, Cases: len(fixtures.Cases), Synthetic: true, Status: "fixture_expectations_matched"}
	for _, row := range fixtures.Cases {
		got := verify(row.Claim, row.Evidence, fixtures.Policy, fixtures.Now)
		if got == row.Expected {
			report.Matched++
		} else {
			report.Mismatched++
		}
		switch got.Kind {
		case "supported_enum":
			report.SupportedEnums++
		case "withheld":
			report.Withheld++
		}
	}
	if report.Mismatched > 0 {
		report.Status = "fixture_mismatch"
	}
	return report, nil
}

func RunIssueState() (IssueStateReport, error) {
	return compareIssueState(issueFixtures, verifyIssueState)
}
