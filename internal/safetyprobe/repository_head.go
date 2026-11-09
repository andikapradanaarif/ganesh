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

var ErrRepositoryHeadProbe = errors.New("synthetic repository-head probe failed")

// Frozen in issue #67 before implementation. No external inputs are admitted.
//
//go:embed repository_head_v1.json
var headFixtures []byte

const headFixtureDigest = "9e4775604cbbf263c79859cfc2ccee02501c072f3779454a5ec29ec9ddf12db4"

var headCommitID = regexp.MustCompile(`^[0-9a-f]{40}$`)

type headPolicy struct {
	Source     string `json:"source"`
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	MaxAge     int64  `json:"max_check_age_seconds"`
}

type headClaim struct {
	Subject     string `json:"subject"`
	Repository  string `json:"repository"`
	Branch      string `json:"branch"`
	Commit      string `json:"commit"`
	Revision    string `json:"revision"`
	Environment string `json:"environment"`
}

type headObservation struct {
	Channel    string `json:"channel"`
	Source     string `json:"source"`
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	Commit     string `json:"commit"`
	Revision   string `json:"revision"`
	Protected  *bool  `json:"protected"`
	Updated    int64  `json:"updated_unix"`
	Checked    int64  `json:"checked_unix"`
	Prose      string `json:"prose"` // Deliberately carried but never used for support or copied into the result.
}

type headOutcome struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Value  string `json:"value"`
}

// verifyRepositoryHead checks only declared synthetic branch-resolution data.
// It cannot read expected outcomes, authenticate an API or authorize deployment.
func verifyRepositoryHead(c headClaim, e *headObservation, p headPolicy, now int64) headOutcome {
	deny := func(reason string) headOutcome { return headOutcome{Kind: "withheld", Reason: reason} }
	if p.Source == "" || p.Repository == "" || p.Branch == "" || p.MaxAge <= 0 {
		return deny("invalid_policy")
	}
	if c.Subject != "repository.head" {
		return deny("unsupported_subject")
	}
	if c.Environment != "" {
		return deny("unsupported_qualifier")
	}
	if c.Repository != p.Repository {
		return deny("unconfigured_repository")
	}
	if c.Branch != p.Branch {
		return deny("unconfigured_branch")
	}
	if e == nil {
		return deny("missing_evidence")
	}
	if e.Channel != "protected_branch_api" {
		return deny("unsupported_channel")
	}
	if e.Source != p.Source {
		return deny("unconfigured_source")
	}
	if e.Protected == nil {
		return deny("protection_unknown")
	}
	if !*e.Protected {
		return deny("unprotected_branch")
	}
	if c.Repository != e.Repository || c.Branch != e.Branch {
		return deny("object_mismatch")
	}
	if !headCommitID.MatchString(c.Commit) || !headCommitID.MatchString(e.Commit) || !revisionID.MatchString(c.Revision) || !revisionID.MatchString(e.Revision) {
		return deny("invalid_identity")
	}
	if c.Commit != e.Commit {
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
	return headOutcome{Kind: "supported_head", Reason: "exact_repository_head", Value: c.Commit}
}

type headCase struct {
	Name     string           `json:"name"`
	Policy   headPolicy       `json:"policy"`
	Now      int64            `json:"now_unix"`
	Claim    headClaim        `json:"claim"`
	Evidence *headObservation `json:"evidence"`
	Expected headOutcome      `json:"expected"`
}

type headFixtureSet struct {
	Schema string     `json:"schema"`
	Cases  []headCase `json:"cases"`
}

type RepositoryHeadReport struct {
	Schema                string `json:"schema"`
	FixtureSHA256         string `json:"fixture_sha256"`
	Status                string `json:"status"`
	Cases                 int    `json:"cases"`
	Matched               int    `json:"matched"`
	Mismatched            int    `json:"mismatched"`
	SupportedHeads        int    `json:"supported_heads"`
	Withheld              int    `json:"withheld"`
	Synthetic             bool   `json:"synthetic"`
	PublicationAuthorized bool   `json:"publication_authorized"`
	StageGatePassed       bool   `json:"stage_gate_passed"`
}

type headVerifier func(headClaim, *headObservation, headPolicy, int64) headOutcome

// Only the scorer sees the frozen expectations. The candidate receives no
// per-case name or expected value; an always-positive mutant is tested separately.
func compareRepositoryHead(body []byte, verify headVerifier) (RepositoryHeadReport, error) {
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	if digest != headFixtureDigest {
		return RepositoryHeadReport{}, ErrRepositoryHeadProbe
	}
	var fixtures headFixtureSet
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(&fixtures) != nil || fixtures.Schema != "synthetic-repository-head-v1" || len(fixtures.Cases) != 43 {
		return RepositoryHeadReport{}, ErrRepositoryHeadProbe
	}
	if d.Decode(new(any)) != io.EOF {
		return RepositoryHeadReport{}, ErrRepositoryHeadProbe
	}
	report := RepositoryHeadReport{Schema: fixtures.Schema, FixtureSHA256: digest, Cases: len(fixtures.Cases), Status: "fixture_expectations_matched", Synthetic: true}
	for _, row := range fixtures.Cases {
		got := verify(row.Claim, row.Evidence, row.Policy, row.Now)
		if got == row.Expected {
			report.Matched++
		} else {
			report.Mismatched++
		}
		switch got.Kind {
		case "supported_head":
			report.SupportedHeads++
		case "withheld":
			report.Withheld++
		}
	}
	if report.Mismatched > 0 {
		report.Status = "fixture_mismatch"
	}
	return report, nil
}

func RunRepositoryHead() (RepositoryHeadReport, error) {
	return compareRepositoryHead(headFixtures, verifyRepositoryHead)
}
