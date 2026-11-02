package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andikapradanaarif/ganesh/internal/s0"
)

// This CLI fixture is synthetic and independent of s0's private test helpers.
func cliEvaluationDefinition() s0.EvaluationDefinition {
	seed := uint32(0)
	hash := strings.Repeat("b", 64)
	d := s0.NewEvaluationDefinition("s0-" + strings.Repeat("1", 32))
	d.Seed = &seed
	d.Bindings = s0.EvaluationBindings{Corpus: hash, EligibleCorpus: hash, Baseline: hash, TypedCards: hash, Variants: hash, Permissions: hash,
		ModelsLimits: hash, Calibration: hash, Grading: hash, CleanSessions: hash, FullContext: hash}
	d.CalibrationIDs = []string{"synthetic-calibration"}
	for _, role := range []string{"coder", "planner", "reviewer"} {
		for n := 0; n < 10; n++ {
			within := 7
			if role == "reviewer" {
				within = 6
			}
			stratum, diagnostic := "within", n < 2
			if n >= within {
				stratum, diagnostic = "cross", n < within+2
			}
			id := fmt.Sprintf("synthetic-%s-%02d", role, n)
			prompt := sha256.Sum256([]byte(id))
			access := n < within
			d.Tasks = append(d.Tasks, s0.EvaluationTask{ID: id, Role: role, Stratum: stratum, Diagnostic: diagnostic, Tags: []string{"changed_state"},
				Prompt: hex.EncodeToString(prompt[:]), Rubric: hash, Oracle: hash, Sources: hash, BaselineCanAccess: &access})
		}
	}
	return d
}

func TestEvaluationCLIRoundTrip(t *testing.T) {
	dir := t.TempDir()
	input, snapshot := filepath.Join(dir, "evaluation.json"), filepath.Join(dir, "evaluation.frozen.json")
	body, err := json.Marshal(cliEvaluationDefinition())
	if err != nil {
		t.Fatal(err)
	}
	if os.WriteFile(input, body, 0o600) != nil {
		t.Fatal("cannot write definition")
	}
	var frozen bytes.Buffer
	if err := run(context.Background(), []string{"s0", "freeze-evaluation", "--input", input, "--output", snapshot}, &frozen); err != nil {
		t.Fatal(err)
	}
	var freezeReport s0.EvaluationReport
	if json.Unmarshal(frozen.Bytes(), &freezeReport) != nil {
		t.Fatal("cannot read freeze report")
	}
	var checked bytes.Buffer
	if err := run(context.Background(), []string{"s0", "check-evaluation", "--input", snapshot, "--expected-digest", freezeReport.Digest}, &checked); err != nil {
		t.Fatal(err)
	}
	for _, pin := range []string{"", strings.Repeat("f", 64)} {
		args := []string{"s0", "check-evaluation", "--input", snapshot}
		if pin != "" {
			args = append(args, "--expected-digest", pin)
		}
		var rejected bytes.Buffer
		if err := run(context.Background(), args, &rejected); err != s0.ErrEvaluation || rejected.Len() != 0 {
			t.Fatal("missing/wrong baseline pin accepted")
		}
	}
	for i, output := range [][]byte{frozen.Bytes(), checked.Bytes()} {
		var result map[string]any
		if json.Unmarshal(output, &result) != nil {
			t.Fatal("invalid report")
		}
		wantState := []string{"evaluation_frozen", "evaluation_snapshot_valid"}[i]
		if result["state"] != wantState || result["tasks"] != float64(30) || result["base_answers"] != float64(108) || result["conditional_answers"] != float64(24) || result["maximum_answers"] != float64(132) || result["eligible_origin_trigger"] != "not_evaluated" {
			t.Fatal("CLI reported incorrect plan state")
		}
		for _, key := range []string{"evidence_independently_verified", "model_runs_authorized", "production_narrative_enabled"} {
			if value, exists := result[key]; !exists || value != false {
				t.Fatal("explicit false assurance flags missing")
			}
		}
		if bytes.Contains(output, []byte("synthetic-")) || bytes.Contains(output, []byte(strings.Repeat("b", 64))) {
			t.Fatal("CLI echoed tasks or artifact declarations")
		}
	}
	after, _ := os.ReadFile(input)
	if !bytes.Equal(body, after) {
		t.Fatal("freeze changed definition")
	}
	prior, _ := os.ReadFile(snapshot)
	var repeated bytes.Buffer
	if err := run(context.Background(), []string{"s0", "freeze-evaluation", "--input", input, "--output", snapshot}, &repeated); err == nil || repeated.Len() != 0 {
		t.Fatal("repeated freeze overwrote or reported success")
	}
	current, _ := os.ReadFile(snapshot)
	if !bytes.Equal(prior, current) {
		t.Fatal("snapshot replaced")
	}
	info, err := os.Stat(snapshot)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("snapshot is not private")
	}
	if _, err := os.Stat(filepath.Join(dir, "run.json")); !os.IsNotExist(err) {
		t.Fatal("standalone declaration unexpectedly required or created a workspace run record")
	}
	// A changed arm must be rejected before a report is emitted.
	var altered s0.EvaluationSnapshot
	if json.Unmarshal(prior, &altered) != nil {
		t.Fatal("cannot parse snapshot")
	}
	altered.Base[0].Arm = "F"
	changed, _ := json.Marshal(altered)
	if os.WriteFile(snapshot, changed, 0o600) != nil {
		t.Fatal("cannot write altered fixture")
	}
	var invalid bytes.Buffer
	if err := run(context.Background(), []string{"s0", "check-evaluation", "--input", snapshot, "--expected-digest", freezeReport.Digest}, &invalid); err != s0.ErrEvaluation || invalid.Len() != 0 {
		t.Fatal("tampered snapshot reached report")
	}
}

func TestEvaluationCLIRejectsUnknownAndAmbiguousInput(t *testing.T) {
	valid, _ := json.Marshal(cliEvaluationDefinition())
	unknown, _ := json.Marshal(s0.NewEvaluationDefinition("s0-" + strings.Repeat("2", 32)))
	for _, body := range [][]byte{unknown, []byte(`{"raw":"ghp_SYNTHETIC_SECRET"}`), append(append([]byte{}, valid...), []byte(` {"raw":"ghp_SYNTHETIC_SECRET"}`)...), []byte(strings.Replace(string(valid), "{", `{"schema_version":1,`, 1))} {
		dir := t.TempDir()
		input, output := filepath.Join(dir, "ghp_SYNTHETIC_SECRET.json"), filepath.Join(dir, "must-not-exist")
		if os.WriteFile(input, body, 0o600) != nil {
			t.Fatal("cannot write input")
		}
		for _, command := range []string{"freeze-evaluation", "check-evaluation"} {
			args := []string{"s0", command, "--input", input}
			if command == "freeze-evaluation" {
				args = append(args, "--output", output)
			} else {
				args = append(args, "--expected-digest", strings.Repeat("a", 64))
			}
			var report bytes.Buffer
			err := run(context.Background(), args, &report)
			if err == nil || report.Len() != 0 || strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
				t.Fatal("invalid input leaked or passed")
			}
		}
		if _, err := os.Stat(output); !os.IsNotExist(err) {
			t.Fatal("invalid plan created output")
		}
	}
}
