package safetyprobe

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestFrozenRepositoryHeadCases(t *testing.T) {
	var fixtures headFixtureSet
	if json.Unmarshal(headFixtures, &fixtures) != nil {
		t.Fatal("invalid frozen head fixtures")
	}
	seen := make(map[string]bool)
	for _, row := range fixtures.Cases {
		if row.Name == "" || seen[row.Name] {
			t.Fatal("duplicate or unnamed head fixture")
		}
		seen[row.Name] = true
		t.Run(row.Name, func(t *testing.T) {
			got := verifyRepositoryHead(row.Claim, row.Evidence, row.Policy, row.Now)
			if got != row.Expected {
				t.Fatalf("got %+v; frozen expectation %+v", got, row.Expected)
			}
			if got.Kind == "supported_head" && (got.Value != row.Claim.Commit || !headCommitID.MatchString(got.Value)) {
				t.Fatal("commit identity changed or prose escaped typed result")
			}
		})
	}
	report, err := RunRepositoryHead()
	if err != nil || report.Cases != 43 || report.Matched != 43 || report.Mismatched != 0 || report.SupportedHeads != 4 || report.Withheld != 39 || report.PublicationAuthorized || report.StageGatePassed || !report.Synthetic || report.Status != "fixture_expectations_matched" {
		t.Fatalf("invalid head report: %+v, %v", report, err)
	}
	body, _ := json.Marshal(report)
	for _, forbidden := range []string{"ghp_", "synthetic/project", "fixture.repository-api", "deploy", strings.Repeat("1", 40)} {
		if strings.Contains(string(body), forbidden) {
			t.Fatal("fixture content leaked into head report")
		}
	}
}

func TestHeadOracleRejectsAlwaysPositiveVerifier(t *testing.T) {
	// Only exact-head, freshness-boundary and prose-only perturbation have
	// this value. The changed-head positive and all 39 negatives must fail.
	report, err := compareRepositoryHead(headFixtures, func(headClaim, *headObservation, headPolicy, int64) headOutcome {
		return headOutcome{Kind: "supported_head", Reason: "exact_repository_head", Value: strings.Repeat("1", 40)}
	})
	if err != nil || report.Matched != 3 || report.Mismatched != 40 || report.Status != "fixture_mismatch" || report.StageGatePassed || report.PublicationAuthorized {
		t.Fatal("head scorer accepted a verifier's own success claims")
	}
}

func TestHeadFixtureIntegrity(t *testing.T) {
	changed := append(append([]byte(nil), headFixtures...), '\n')
	report, err := compareRepositoryHead(changed, verifyRepositoryHead)
	if err != ErrRepositoryHeadProbe || !reflect.DeepEqual(report, RepositoryHeadReport{}) {
		t.Fatal("changed head expectations accepted or partial report returned")
	}
}
