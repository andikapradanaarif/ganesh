package safetyprobe

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestFrozenEchoOriginCases(t *testing.T) {
	var fixtures echoFixtureSet
	if json.Unmarshal(echoFixtures, &fixtures) != nil {
		t.Fatal("invalid frozen echo fixtures")
	}
	seen := make(map[string]bool)
	for _, row := range fixtures.Cases {
		if row.Name == "" || seen[row.Name] {
			t.Fatal("duplicate or unnamed echo fixture")
		}
		seen[row.Name] = true
		t.Run(row.Name, func(t *testing.T) {
			got := traceEchoOrigins(row.Graph)
			if got != row.Expected {
				t.Fatalf("got %+v; frozen expectation %+v", got, row.Expected)
			}
			if got.IndependenceEstablished || got.Level == "verified" || got.UntrustedOrigins+got.UnknownOrigins+got.StructuredOrigins != got.DistinctOrigins {
				t.Fatal("invalid root accounting or authority upgrade")
			}
		})
	}
	report, err := RunEchoOrigins()
	if err != nil || report.Cases != 50 || report.Matched != 50 || report.Mismatched != 0 || report.ReportedLineages != 23 || report.IncompleteLineages != 27 || report.IndependenceEstablished || report.PublicationAuthorized || report.StageGatePassed || !report.Synthetic || report.Status != "fixture_expectations_matched" {
		t.Fatalf("invalid echo report: %+v, %v", report, err)
	}
	body, _ := json.Marshal(report)
	for _, forbidden := range []string{"ghp_", "synthetic-project", "source-1", "object-1", "deploy", `"a1"`} {
		if strings.Contains(string(body), forbidden) {
			t.Fatal("fixture content leaked into echo report")
		}
	}
}

func TestEchoOracleRejectsVoteCountingAndPromotion(t *testing.T) {
	votes := func(g echoGraph) echoOutcome {
		return echoOutcome{Kind: "reported_lineage", Reason: "declared_ancestry_traced", Level: "reported", DistinctOrigins: len(g.Targets), OriginRevisions: len(g.Targets), UntrustedOrigins: len(g.Targets)}
	}
	report, err := compareEchoOrigins(echoFixtures, votes)
	if err != nil || report.Mismatched == 0 || report.Status != "fixture_mismatch" || report.StageGatePassed || report.IndependenceEstablished {
		t.Fatal("agent votes passed as distinct source origins")
	}
	var fixtures echoFixtureSet
	if json.Unmarshal(echoFixtures, &fixtures) != nil {
		t.Fatal("invalid fixtures")
	}
	found := false
	for _, row := range fixtures.Cases {
		if row.Name == "two-agent-echo" {
			found = true
			if row.Expected.DistinctOrigins != 1 || votes(row.Graph).DistinctOrigins != 2 {
				t.Fatal("critical echo case does not isolate vote inflation")
			}
		}
	}
	if !found {
		t.Fatal("missing two-agent echo fixture")
	}
	report, err = compareEchoOrigins(echoFixtures, func(g echoGraph) echoOutcome {
		out := votes(g)
		out.Level, out.IndependenceEstablished = "verified", true
		return out
	})
	if err != nil || report.Matched != 0 || report.Mismatched != 50 || report.Status != "fixture_mismatch" || report.PublicationAuthorized || report.StageGatePassed {
		t.Fatal("scorer accepted fabricated verification or independence")
	}
}

func TestEchoFixtureIntegrity(t *testing.T) {
	changed := append(append([]byte(nil), echoFixtures...), '\n')
	report, err := compareEchoOrigins(changed, traceEchoOrigins)
	if err != ErrEchoOriginProbe || !reflect.DeepEqual(report, EchoOriginReport{}) {
		t.Fatal("changed echo expectations accepted or partial report returned")
	}
}
