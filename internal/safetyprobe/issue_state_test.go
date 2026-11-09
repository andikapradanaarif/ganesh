package safetyprobe

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestFrozenIssueStateCases(t *testing.T) {
	var fixtures issueFixtureSet
	if json.Unmarshal(issueFixtures, &fixtures) != nil {
		t.Fatal("invalid frozen fixtures")
	}
	seen := make(map[string]bool)
	for _, row := range fixtures.Cases {
		if row.Name == "" || seen[row.Name] {
			t.Fatal("duplicate or unnamed fixture")
		}
		seen[row.Name] = true
		t.Run(row.Name, func(t *testing.T) {
			got := verifyIssueState(row.Claim, row.Evidence, fixtures.Policy, fixtures.Now)
			if got != row.Expected {
				t.Fatalf("got %+v; frozen expectation %+v", got, row.Expected)
			}
			if strings.Contains(got.Value, "ghp_") || (got.Kind == "supported_enum" && got.Value != "open" && got.Value != "closed") {
				t.Fatal("prose escaped typed result")
			}
		})
	}
	report, err := RunIssueState()
	if err != nil || report.Cases != 24 || report.Matched != 24 || report.Mismatched != 0 || report.SupportedEnums != 4 || report.Withheld != 20 || report.StageGatePassed || report.PublicationAuthorized || !report.Synthetic || report.Status != "fixture_expectations_matched" {
		t.Fatalf("unexpected probe report: %+v, %v", report, err)
	}
	body, _ := json.Marshal(report)
	for _, forbidden := range []string{"ghp_", "synthetic/project", "deploy", "fixture.issue-api"} {
		if strings.Contains(string(body), forbidden) {
			t.Fatal("fixture content leaked into aggregate output")
		}
	}
}

func TestIssueStateOracleRejectsBadVerifier(t *testing.T) {
	// An always-positive candidate matches only the three closed-enum cases.
	// The open case and all twenty negative cases must still be mismatches.
	report, err := compareIssueState(issueFixtures, func(issueClaim, *issueObservation, issuePolicy, int64) issueOutcome {
		return issueOutcome{Kind: "supported_enum", Reason: "exact_issue_state", Value: "closed"}
	})
	if err != nil || report.Matched != 3 || report.Mismatched != 21 || report.Status != "fixture_mismatch" || report.StageGatePassed || report.PublicationAuthorized {
		t.Fatal("scorer accepted a verifier's own success claims")
	}
}

func TestIssueStateFixtureIntegrity(t *testing.T) {
	changed := append(append([]byte(nil), issueFixtures...), '\n')
	report, err := compareIssueState(changed, verifyIssueState)
	if err != ErrIssueStateProbe || !reflect.DeepEqual(report, IssueStateReport{}) {
		t.Fatal("changed expectations accepted or partial report returned")
	}
}
