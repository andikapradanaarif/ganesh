package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andikapradanaarif/ganesh/internal/s0"
)

func cliResultsFixture(t *testing.T) (s0.EvaluationSnapshot, s0.EvaluationResults, string, string) {
	t.Helper()
	s, err := s0.FreezeEvaluation(cliEvaluationDefinition())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	plan, input := filepath.Join(dir, "plan.json"), filepath.Join(dir, "results.json")
	if _, err := s0.SaveEvaluationSnapshot(plan, s); err != nil {
		t.Fatal(err)
	}
	in := s0.EvaluationResults{SchemaVersion: 1, SnapshotDigest: s.Digest, Results: []s0.EvaluationResult{}}
	for _, slot := range s.Base {
		h := sha256.Sum256([]byte("synthetic-cli-evidence/" + slot.TaskID + slot.Arm + slot.Corpus))
		e := hex.EncodeToString(h[:])
		ids := []string{}
		tokens, elapsed, cost := int64(100), int64(10), int64(1)
		in.Results = append(in.Results, s0.EvaluationResult{Answer: slot, Outcome: "fail", CriticalErrors: &ids, AnsweringTokens: &tokens, ElapsedMS: &elapsed, CostMicroUSD: &cost, EvidenceSHA256: &e})
	}
	return s, in, plan, input
}

func TestResultCLIAccountingAndPartialExit(t *testing.T) {
	s, in, plan, input := cliResultsFixture(t)
	args := []string{"s0", "report-evaluation", "--plan", plan, "--expected-digest", s.Digest, "--input", input}
	for _, partial := range []bool{false, true} {
		v := in
		if partial {
			v.Results = []s0.EvaluationResult{}
		}
		body, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if os.WriteFile(input, body, 0o600) != nil {
			t.Fatal("cannot write fixture")
		}
		var output bytes.Buffer
		err = run(context.Background(), args, &output)
		if partial {
			if err != s0.ErrIncompleteResults {
				t.Fatal("partial accounting did not exit incomplete")
			}
		} else if err != nil {
			t.Fatal(err)
		}
		var report map[string]any
		if json.Unmarshal(output.Bytes(), &report) != nil {
			t.Fatal("report absent")
		}
		state := "metadata_complete"
		if partial {
			state = "incomplete_base_metadata"
		}
		if report["state"] != state || report["snapshot_digest"] != s.Digest {
			t.Fatal("bad report identity/state")
		}
		for _, flag := range []string{"experiment_accepted", "evidence_independently_verified", "model_runs_authorized", "production_narrative_enabled"} {
			if value, exists := report[flag]; !exists || value != false {
				t.Fatal("accounting granted authority")
			}
		}
		after, _ := os.ReadFile(input)
		if !bytes.Equal(body, after) {
			t.Fatal("analyzer mutated input")
		}
	}
}

func TestResultCLIRejectsUnpinnedAndInvalidMetadata(t *testing.T) {
	s, in, plan, input := cliResultsFixture(t)
	valid, _ := json.Marshal(in)
	for _, scenario := range []string{"unknown", "duplicate_key", "case_alias", "mode", "duplicate_slot", "reused_evidence", "wrong_plan"} {
		t.Run(scenario, func(t *testing.T) {
			body := string(valid)
			switch scenario {
			case "unknown":
				body = strings.Replace(body, "{", `{"raw_answer":"ghp_SYNTHETIC_SECRET",`, 1)
			case "duplicate_key":
				body = strings.Replace(body, "{", `{"schema_version":1,`, 1)
			case "case_alias":
				body = strings.Replace(body, `"snapshot_digest"`, `"Snapshot_Digest"`, 1)
			case "duplicate_slot":
				v := in
				v.Results = append(append([]s0.EvaluationResult{}, in.Results...), in.Results[0])
				b, _ := json.Marshal(v)
				body = string(b)
			case "reused_evidence":
				v := in
				v.Results = append([]s0.EvaluationResult{}, in.Results...)
				v.Results[1].EvidenceSHA256 = v.Results[0].EvidenceSHA256
				b, _ := json.Marshal(v)
				body = string(b)
			case "wrong_plan":
				body = strings.Replace(body, s.Digest, strings.Repeat("f", 64), 1)
			}
			if os.WriteFile(input, []byte(body), 0o600) != nil {
				t.Fatal("cannot write fixture")
			}
			if scenario == "mode" {
				if os.Chmod(input, 0o644) != nil {
					t.Fatal("chmod")
				}
				defer os.Chmod(input, 0o600)
			}
			var output bytes.Buffer
			err := run(context.Background(), []string{"s0", "report-evaluation", "--plan", plan, "--expected-digest", s.Digest, "--input", input}, &output)
			if err == nil || output.Len() != 0 || strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
				t.Fatal("invalid data passed or leaked")
			}
		})
	}
	var output bytes.Buffer
	err := run(context.Background(), []string{"s0", "report-evaluation", "--plan", plan, "--expected-digest", strings.Repeat("f", 64), "--input", filepath.Join(t.TempDir(), "missing")}, &output)
	if err != s0.ErrEvaluation || output.Len() != 0 {
		t.Fatal("metadata read preceded baseline check")
	}
}
