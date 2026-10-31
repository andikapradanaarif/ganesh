package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/andikapradanaarif/ganesh/internal/s0"
)

func TestFreezeAndScreenCLI(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "targets.json")
	baseline := filepath.Join(dir, "frozen-needs.json")
	in := s0.TargetInventory{SchemaVersion: 1, State: "not_screened", Targets: []s0.ScreenTarget{
		{NeedDefinition: s0.NeedDefinition{ID: "fixture-one", Need: "synthetic first need"}},
		{NeedDefinition: s0.NeedDefinition{ID: "fixture-two", Need: "synthetic second need"}},
	}}
	writeFixture(t, input, in)
	var output bytes.Buffer
	if err := run(context.Background(), []string{"s0", "freeze-targets", "--input", input, "--output", baseline}, &output); err != nil {
		t.Fatal(err)
	}
	var freeze struct {
		State  string `json:"state"`
		Digest string `json:"need_digest"`
		Count  int    `json:"target_count"`
	}
	if err := json.Unmarshal(output.Bytes(), &freeze); err != nil || freeze.Count != 2 || len(freeze.Digest) != 64 {
		t.Fatalf("freeze: %s %v", output.Bytes(), err)
	}
	if err := run(context.Background(), []string{"s0", "freeze-targets", "--input", input, "--output", baseline}, &bytes.Buffer{}); err == nil {
		t.Fatal("CLI overwrote frozen needs")
	}
	in.State = "screened"
	in.Targets[0].Assessed = true
	in.Targets[0].Mixed = true
	in.Targets[0].Unknown = true
	in.Targets[0].ReasonRefs = []string{"fixture-input-evidence"}
	in.Targets[1].Assessed = true
	in.Targets[1].OwnerOnly = true
	in.Targets[1].ReasonRefs = []string{"fixture-owner-only-evidence"}
	writeFixture(t, input, in)
	before, _ := os.ReadFile(input)
	output.Reset()
	if err := run(context.Background(), []string{"s0", "screen", "--input", input, "--baseline", baseline}, &output); err != nil {
		t.Fatal(err)
	}
	var report s0.ScreenReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Counts != (s0.OriginCounts{H: 2, M: 1}) || report.Route != "a2_3_decision" || report.ModelRunsAuthorized || report.ProductionNarrativeEnabled {
		t.Fatalf("wrong report: %+v", report)
	}
	after, _ := os.ReadFile(input)
	if !bytes.Equal(before, after) {
		t.Fatal("screen modified source inventory")
	}
	// Unavailable transcripts can be explicitly unknown without repeating
	// assessed=true. This remains pending recheck, not inconclusive or eligible.
	in.Targets[0].Assessed = false
	in.Targets[0].Mixed = false
	in.Targets[1].Assessed = false
	in.Targets[1].OwnerOnly = false
	in.Targets[1].Unknown = true
	writeFixture(t, input, in)
	output.Reset()
	if err := run(context.Background(), []string{"s0", "screen", "--input", input, "--baseline", baseline}, &output); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Counts != (s0.OriginCounts{H: 2, U: 2}) || report.Route != "single_recheck_pending" {
		t.Fatalf("unknown-only declarations were not routed: %+v", report)
	}
	output.Reset()
	in.Targets[1].Need = "changed after freeze"
	writeFixture(t, input, in)
	if err := run(context.Background(), []string{"s0", "screen", "--input", input, "--baseline", baseline}, &output); err == nil || output.Len() != 0 {
		t.Fatal("changed need definition produced a report")
	}
}

func TestEmptyScreenCLIIsExplicitlyInconclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "targets.json")
	writeFixture(t, path, s0.NewTargetInventory())
	var output bytes.Buffer
	if err := run(context.Background(), []string{"s0", "screen", "--input", path}, &output); err == nil {
		t.Fatal("empty inventory returned success")
	}
	var report s0.ScreenReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil || report.Route != "inconclusive" || report.Ratios != nil {
		t.Fatalf("empty report: %s %v", output.Bytes(), err)
	}
}

func TestInvalidUTF8IsNotSilentlyChangedInNeedIdentity(t *testing.T) {
	body := append([]byte(`{"schema_version":1,"state":"not_screened","targets":[{"id":"fixture","need":"`), 0xff)
	body = append(body, []byte(`"}]}`)...)
	var inventory s0.TargetInventory
	if err := decode(body, &inventory); err == nil {
		t.Fatal("invalid UTF-8 was silently normalized")
	}
}

func writeFixture(t *testing.T, path string, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
}
