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

var ErrDecisionAuthorityProbe = errors.New("synthetic decision-authority probe failed")

// Frozen before implementation in issue #71. All authority is simulated.
//
//go:embed decision_authority_v1.json
var decisionFixtures []byte

const decisionFixtureDigest = "e3a9e4d6172cfd44dd1549b7c544bdb9613a0acc8813c25a3c46728263971189"

type decisionPolicy struct {
	Source     string   `json:"source"`
	Repository string   `json:"repository"`
	Registry   string   `json:"registry"`
	Key        string   `json:"key"`
	Values     []string `json:"values"`
	Designated *bool    `json:"designated"`
	MaxAge     int64    `json:"max_check_age_seconds"`
}

type decisionClaim struct {
	Subject     string `json:"subject"`
	Repository  string `json:"repository"`
	Registry    string `json:"registry"`
	Decision    string `json:"decision"`
	State       string `json:"state"`
	Value       string `json:"value"`
	Revision    string `json:"revision"`
	Environment string `json:"environment"`
}

type decisionObservation struct {
	Channel           string `json:"channel"`
	Source            string `json:"source"`
	Repository        string `json:"repository"`
	Registry          string `json:"registry"`
	Key               string `json:"key"`
	Decision          string `json:"decision"`
	State             string `json:"state"`
	Value             string `json:"value"`
	Revision          string `json:"revision"`
	Authenticated     *bool  `json:"authenticated"`
	ControlledWriters *bool  `json:"controlled_writers"`
	Adopted           int64  `json:"adopted_unix"`
	Updated           int64  `json:"updated_unix"`
	Checked           int64  `json:"checked_unix"`
	Prose             string `json:"prose"`       // Untrusted fixture text; never used for support or copied into the result.
	AuthorRole        string `json:"author_role"` // An author label cannot designate a source or grant decision authority.
}

type decisionOutcome struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	State  string `json:"state"`
	Value  string `json:"value"`
}

// The configured designation and observed authentication/writer controls are
// fake premises. No owner identity, real registry or provenance is verified.
func verifyDecisionAuthority(c decisionClaim, e *decisionObservation, p decisionPolicy, now int64) decisionOutcome {
	deny := func(reason string) decisionOutcome { return decisionOutcome{Kind: "withheld", Reason: reason} }
	if p.Source == "" || p.Repository == "" || !revisionID.MatchString(p.Registry) || !revisionID.MatchString(p.Key) || p.MaxAge <= 0 || len(p.Values) == 0 {
		return deny("invalid_policy")
	}
	allowed := make(map[string]bool)
	for _, value := range p.Values {
		if !revisionID.MatchString(value) || allowed[value] {
			return deny("invalid_policy")
		}
		allowed[value] = true
	}
	if p.Designated == nil {
		return deny("designation_unknown")
	}
	if !*p.Designated {
		return deny("source_not_designated")
	}
	if c.Subject != "decision."+p.Key {
		return deny("unsupported_subject")
	}
	if c.Environment != "" {
		return deny("unsupported_qualifier")
	}
	if c.Repository != p.Repository {
		return deny("unconfigured_repository")
	}
	if c.Registry != p.Registry {
		return deny("unconfigured_registry")
	}
	if e == nil {
		return deny("missing_evidence")
	}
	if e.Channel != "structured_registry" {
		return deny("unsupported_channel")
	}
	if e.Source != p.Source {
		return deny("unconfigured_source")
	}
	if e.Authenticated == nil {
		return deny("authentication_unknown")
	}
	if !*e.Authenticated {
		return deny("unauthenticated_source")
	}
	if e.ControlledWriters == nil {
		return deny("writer_control_unknown")
	}
	if !*e.ControlledWriters {
		return deny("uncontrolled_writers")
	}
	if e.Repository != c.Repository || e.Registry != c.Registry || e.Key != p.Key {
		return deny("object_mismatch")
	}
	if !revisionID.MatchString(c.Decision) || !revisionID.MatchString(e.Decision) || !revisionID.MatchString(c.Revision) || !revisionID.MatchString(e.Revision) {
		return deny("invalid_identity")
	}
	if c.Decision != e.Decision {
		return deny("decision_mismatch")
	}
	if c.Revision != e.Revision {
		return deny("revision_mismatch")
	}
	if c.State != "adopted" {
		return deny("unsupported_state")
	}
	if e.State != "adopted" && e.State != "proposed" && e.State != "rejected" && e.State != "superseded" {
		return deny("invalid_state")
	}
	if e.State != "adopted" {
		return deny("not_adopted")
	}
	if !revisionID.MatchString(c.Value) || !revisionID.MatchString(e.Value) {
		return deny("invalid_value")
	}
	if !allowed[c.Value] || !allowed[e.Value] {
		return deny("unconfigured_value")
	}
	if c.Value != e.Value {
		return deny("value_mismatch")
	}
	if now <= 0 || e.Adopted <= 0 || e.Updated <= 0 || e.Checked <= 0 || e.Adopted > e.Updated || e.Updated > e.Checked || e.Checked > now {
		return deny("invalid_time")
	}
	if now-e.Checked > p.MaxAge {
		return deny("stale_check")
	}
	return decisionOutcome{Kind: "supported_adoption_record", Reason: "exact_adoption_record", State: "adopted", Value: c.Value}
}

type decisionCase struct {
	Name     string               `json:"name"`
	Policy   decisionPolicy       `json:"policy"`
	Now      int64                `json:"now_unix"`
	Claim    decisionClaim        `json:"claim"`
	Evidence *decisionObservation `json:"evidence"`
	Expected decisionOutcome      `json:"expected"`
}

type decisionFixtureSet struct {
	Schema string         `json:"schema"`
	Cases  []decisionCase `json:"cases"`
}

type DecisionAuthorityReport struct {
	Schema                string `json:"schema"`
	FixtureSHA256         string `json:"fixture_sha256"`
	Status                string `json:"status"`
	Cases                 int    `json:"cases"`
	Matched               int    `json:"matched"`
	Mismatched            int    `json:"mismatched"`
	SupportedAdoptions    int    `json:"supported_adoptions"`
	Withheld              int    `json:"withheld"`
	Synthetic             bool   `json:"synthetic"`
	PublicationAuthorized bool   `json:"publication_authorized"`
	StageGatePassed       bool   `json:"stage_gate_passed"`
}

type decisionVerifier func(decisionClaim, *decisionObservation, decisionPolicy, int64) decisionOutcome

// Only this scorer sees expectations; adoption state is compared as well as
// kind/reason/value, so a matching value cannot silently omit its qualification.
func compareDecisionAuthority(body []byte, verify decisionVerifier) (DecisionAuthorityReport, error) {
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	if digest != decisionFixtureDigest {
		return DecisionAuthorityReport{}, ErrDecisionAuthorityProbe
	}
	var fixtures decisionFixtureSet
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(&fixtures) != nil || fixtures.Schema != "synthetic-decision-authority-v1" || len(fixtures.Cases) != 67 {
		return DecisionAuthorityReport{}, ErrDecisionAuthorityProbe
	}
	if d.Decode(new(any)) != io.EOF {
		return DecisionAuthorityReport{}, ErrDecisionAuthorityProbe
	}
	report := DecisionAuthorityReport{Schema: fixtures.Schema, FixtureSHA256: digest, Cases: len(fixtures.Cases), Synthetic: true, Status: "fixture_expectations_matched"}
	for _, row := range fixtures.Cases {
		got := verify(row.Claim, row.Evidence, row.Policy, row.Now)
		if got == row.Expected {
			report.Matched++
		} else {
			report.Mismatched++
		}
		switch got.Kind {
		case "supported_adoption_record":
			report.SupportedAdoptions++
		case "withheld":
			report.Withheld++
		}
	}
	if report.Mismatched > 0 {
		report.Status = "fixture_mismatch"
	}
	return report, nil
}

func RunDecisionAuthority() (DecisionAuthorityReport, error) {
	return compareDecisionAuthority(decisionFixtures, verifyDecisionAuthority)
}
