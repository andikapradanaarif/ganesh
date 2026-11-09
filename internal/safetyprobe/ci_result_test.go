package safetyprobe

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestFrozenCIResultCases(t *testing.T) {
	var fixtures ciFixtureSet
	if json.Unmarshal(ciFixtures, &fixtures) != nil {
		t.Fatal("invalid frozen CI fixtures")
	}
	seen := make(map[string]bool)
	for _, row := range fixtures.Cases {
		if row.Name == "" || seen[row.Name] {
			t.Fatal("duplicate or unnamed CI fixture")
		}
		seen[row.Name] = true
		t.Run(row.Name, func(t *testing.T) {
			got := verifyCIResult(row.Claim, row.Evidence, row.Head, row.Policy, row.Now)
			if got != row.Expected {
				t.Fatalf("got %+v; frozen expectation %+v", got, row.Expected)
			}
			if got.Kind != "withheld" && (got.Value != row.Claim.Conclusion || (got.Value != "success" && got.Value != "failure")) {
				t.Fatal("unsupported conclusion or prose escaped typed result")
			}
		})
	}
	report, err := RunCIResult()
	if err != nil || report.Cases != 75 || report.Matched != 75 || report.Mismatched != 0 || report.SupportedCommitResults != 8 || report.SupportedHeadResults != 4 || report.Withheld != 63 || report.StageGatePassed || report.PublicationAuthorized || !report.Synthetic || report.Status != "fixture_expectations_matched" {
		t.Fatalf("invalid CI report: %+v, %v", report, err)
	}
	body, _ := json.Marshal(report)
	for _, forbidden := range []string{"ghp_", "synthetic/project", "fixture.ci-api", "run-17", "workflow-1", "deploy", strings.Repeat("1", 40)} {
		if strings.Contains(string(body), forbidden) {
			t.Fatal("fixture content leaked into CI report")
		}
	}
}

func TestCIOracleRejectsAlwaysCurrentGreenVerifier(t *testing.T) {
	// Only three positive fixtures assert current-head success. Even a valid
	// old-commit success must not match this broader, always-green result.
	report, err := compareCIResult(ciFixtures, func(ciClaim, *ciObservation, *headObservation, ciPolicy, int64) ciOutcome {
		return ciOutcome{Kind: "supported_head_result", Reason: "exact_ci_result", Value: "success"}
	})
	if err != nil || report.Matched != 3 || report.Mismatched != 72 || report.Status != "fixture_mismatch" || report.PublicationAuthorized || report.StageGatePassed {
		t.Fatal("CI scorer accepted broader current-head claims")
	}
}

func TestCIFixtureIntegrity(t *testing.T) {
	changed := append(append([]byte(nil), ciFixtures...), '\n')
	report, err := compareCIResult(changed, verifyCIResult)
	if err != ErrCIResultProbe || !reflect.DeepEqual(report, CIResultReport{}) {
		t.Fatal("changed CI expectations accepted or partial report returned")
	}
}
