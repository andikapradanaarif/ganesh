package safetyprobe

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestFrozenDecisionAuthorityCases(t *testing.T) {
	var fixtures decisionFixtureSet
	if json.Unmarshal(decisionFixtures, &fixtures) != nil {
		t.Fatal("invalid frozen decision fixtures")
	}
	seen := make(map[string]bool)
	for _, row := range fixtures.Cases {
		if row.Name == "" || seen[row.Name] {
			t.Fatal("duplicate or unnamed decision fixture")
		}
		seen[row.Name] = true
		t.Run(row.Name, func(t *testing.T) {
			got := verifyDecisionAuthority(row.Claim, row.Evidence, row.Policy, row.Now)
			if got != row.Expected {
				t.Fatalf("got %+v; frozen expectation %+v", got, row.Expected)
			}
			if got.Kind == "supported_adoption_record" && (got.State != "adopted" || got.Value != row.Claim.Value || !revisionID.MatchString(got.Value)) {
				t.Fatal("adoption qualification lost or prose escaped typed result")
			}
		})
	}
	report, err := RunDecisionAuthority()
	if err != nil || report.Cases != 67 || report.Matched != 67 || report.Mismatched != 0 || report.SupportedAdoptions != 5 || report.Withheld != 62 || report.StageGatePassed || report.PublicationAuthorized || !report.Synthetic || report.Status != "fixture_expectations_matched" {
		t.Fatalf("invalid decision report: %+v, %v", report, err)
	}
	body, _ := json.Marshal(report)
	for _, forbidden := range []string{"ghp_", "synthetic/project", "fixture.decision-registry", "decision-17", "option_a", "deploy"} {
		if strings.Contains(string(body), forbidden) {
			t.Fatal("fixture content leaked into decision report")
		}
	}
}

func TestDecisionOracleRejectsTrivialVerifiers(t *testing.T) {
	for _, test := range []struct {
		name    string
		outcome decisionOutcome
		matched int
	}{
		{"always-adopt", decisionOutcome{Kind: "supported_adoption_record", Reason: "exact_adoption_record", State: "adopted", Value: "option_a"}, 4},
		{"always-deny", decisionOutcome{Kind: "withheld", Reason: "unsupported_channel"}, 4},
		{"omit-adoption-state", decisionOutcome{Kind: "supported_adoption_record", Reason: "exact_adoption_record", Value: "option_a"}, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			report, err := compareDecisionAuthority(decisionFixtures, func(decisionClaim, *decisionObservation, decisionPolicy, int64) decisionOutcome {
				return test.outcome
			})
			if err != nil || report.Matched != test.matched || report.Mismatched != 67-test.matched || report.Status != "fixture_mismatch" || report.StageGatePassed || report.PublicationAuthorized {
				t.Fatal("decision scorer accepted a trivial or unqualified answer")
			}
		})
	}
}

func TestDecisionFixtureIntegrity(t *testing.T) {
	changed := append(append([]byte(nil), decisionFixtures...), '\n')
	report, err := compareDecisionAuthority(changed, verifyDecisionAuthority)
	if err != ErrDecisionAuthorityProbe || !reflect.DeepEqual(report, DecisionAuthorityReport{}) {
		t.Fatal("changed decision expectations accepted or partial report returned")
	}
}
