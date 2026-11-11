package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andikapradanaarif/ganesh/internal/devtools"
)

func TestGraderEvidenceCommand(t *testing.T) {
	for _, args := range [][]string{
		{"grader-evidence-check"},
		{"grader-evidence-check", "--dir", "x"},
		{"grader-evidence-check", "--dir", "x", "--frame-sha256", strings.Repeat("a", 64)},
		{"grader-evidence-check", "--dir", "x", "--frame-sha256", strings.Repeat("a", 64), "--response-sha256", strings.Repeat("G", 64)},
		{"grader-evidence-check", "--dir", "x", "--frame-sha256", strings.Repeat("a", 64), "--response-sha256", strings.Repeat("b", 64), "--run"},
		{"grader-evidence-check", "--dir", "x", "--frame-sha256", strings.Repeat("a", 64), "--response-sha256", strings.Repeat("b", 64), "extra"},
	} {
		var stdout, stderr bytes.Buffer
		if err := run(args, &stdout, &stderr); err != devtools.ErrCommand || stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatal("invalid evidence command accepted or echoed")
		}
	}
	dir := t.TempDir()
	if os.Chmod(dir, 0o700) != nil {
		t.Fatal("directory permissions")
	}
	write := func(name string, v any) string {
		b, err := json.Marshal(v)
		if err != nil || os.WriteFile(filepath.Join(dir, name), b, 0o600) != nil {
			t.Fatal("fixture write")
		}
		h := sha256.Sum256(b)
		return hex.EncodeToString(h[:])
	}
	answer := "ghp_SYNTHETIC_QUOTE_CANARY"
	var requirements, criteria []map[string]any
	for _, id := range []string{"C1", "C2", "C3", "C4"} {
		requirements = append(requirements, map[string]any{"id": id, "requirement": "Private requirement canary"})
		criteria = append(criteria, map[string]any{"id": id, "status": "met", "spans": []map[string]any{{"start": 0, "end": len(answer), "quote": answer}}, "explanation": ""})
	}
	framePin := write("answer-frame.json", map[string]any{"schema_version": 1, "task_id": "private-task-canary", "case_id": "private-case-canary", "answer": answer, "criteria": requirements})
	response := map[string]any{"schema_version": 1, "task_id": "private-task-canary", "case_id": "private-case-canary", "answer_frame_sha256": framePin, "completion": true, "introduced_critical_error": false, "unsupported_claim": nil, "criteria": criteria, "notes": "Private note canary"}
	responsePin := write("grader-evidence.json", response)
	args := []string{"grader-evidence-check", "--dir", dir, "--frame-sha256", framePin, "--response-sha256", responsePin}
	var stdout, stderr bytes.Buffer
	if err := run(args, &stdout, &stderr); err != nil || stderr.Len() != 0 {
		t.Fatal("valid command rejected")
	}
	var report devtools.GraderEvidenceReport
	if json.Unmarshal(stdout.Bytes(), &report) != nil || report.Status != "mechanically_checked" || report.ResponseSchemaVersion != 1 || report.VerifiedSpans != 4 || report.DerivedCompletion == nil || !*report.DerivedCompletion || report.UnsupportedClaim != nil || report.SemanticRelevanceVerified || report.CalibrationAccepted || report.ModelDispatchAuthorized {
		t.Fatal("incorrect evidence report")
	}
	for _, private := range []string{"ghp_", "private-task-canary", "private-case-canary", "Private requirement", "Private note", dir} {
		if strings.Contains(stdout.String(), private) {
			t.Fatal("private text leaked")
		}
	}
	if err := run(args, failingOutputWriter{}, &stderr); err != devtools.ErrGraderEvidence {
		t.Fatal("output failure not normalized")
	}
	stdout.Reset()
	args[len(args)-1] = strings.Repeat("0", 64)
	if err := run(args, &stdout, &stderr); err != devtools.ErrGraderEvidence || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("wrong response pin admitted or private output emitted")
	}
	response["completion"] = false
	args[len(args)-1] = write("grader-evidence.json", response)
	if err := run(args, &stdout, &stderr); err != devtools.ErrGraderEvidence || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("contradiction accepted or malformed input emitted")
	}
	response["schema_version"] = 2
	delete(response, "completion")
	args[len(args)-1] = write("grader-evidence.json", response)
	if err := run(args, &stdout, &stderr); err != nil || json.Unmarshal(stdout.Bytes(), &report) != nil || report.ResponseSchemaVersion != 2 || report.DerivedCompletion == nil || !*report.DerivedCompletion || report.SemanticRelevanceVerified || report.CalibrationAccepted || report.ModelDispatchAuthorized {
		t.Fatal("version 2 command failed to derive completion or promoted authority")
	}
	if strings.Contains(stdout.String(), "ghp_") || strings.Contains(stdout.String(), dir) {
		t.Fatal("version 2 private output")
	}
	stdout.Reset()
	response["introduced_critical_error"] = nil
	args[len(args)-1] = write("grader-evidence.json", response)
	if err := run(args, &stdout, &stderr); err != nil || json.Unmarshal(stdout.Bytes(), &report) != nil || report.ResponseSchemaVersion != 2 || report.DerivedCompletion != nil {
		t.Fatal("version 2 unknown critical judgment was collapsed")
	}
	if err := run(args, failingOutputWriter{}, &stderr); err != devtools.ErrGraderEvidence {
		t.Fatal("version 2 writer error not normalized")
	}
	stdout.Reset()
	response["completion"] = nil
	args[len(args)-1] = write("grader-evidence.json", response)
	if err := run(args, &stdout, &stderr); err != devtools.ErrGraderEvidence || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("version 2 supplied completion accepted")
	}
	// New synthetic observations, not a conversion path for saved outputs.
	var observations []map[string]any
	for _, id := range []string{"C1", "C2", "C3", "C4"} {
		observations = append(observations, map[string]any{
			"id": id, "coverage": "covered", "supporting_spans": []map[string]any{{"start": 0, "end": len(answer), "quote": answer}}, "coverage_explanation": "",
			"defeat": "absent", "contrary_spans": []map[string]any{}, "defeat_explanation": "Private no-defeat declaration.",
		})
	}
	response["schema_version"], response["introduced_critical_error"], response["criteria"] = 3, false, observations
	delete(response, "completion")
	args[len(args)-1] = write("grader-evidence.json", response)
	if err := run(args, &stdout, &stderr); err != nil || json.Unmarshal(stdout.Bytes(), &report) != nil || report.ResponseSchemaVersion != 3 || report.Met != 4 || report.VerifiedSpans != 4 || report.DerivedCompletion == nil || !*report.DerivedCompletion || report.UnsupportedClaim != nil || report.SemanticRelevanceVerified || report.CalibrationAccepted || report.ModelDispatchAuthorized {
		t.Fatal("version 3 command output or authority")
	}
	for _, private := range []string{"ghp_", "private-task-canary", "private-case-canary", "Private", dir} {
		if strings.Contains(stdout.String(), private) {
			t.Fatal("version 3 private output")
		}
	}
	stdout.Reset()
	observations[0]["defeat"] = "present"
	observations[0]["contrary_spans"] = observations[0]["supporting_spans"]
	args[len(args)-1] = write("grader-evidence.json", response)
	if err := run(args, &stdout, &stderr); err != nil || json.Unmarshal(stdout.Bytes(), &report) != nil || report.Unmet != 1 || report.VerifiedSpans != 5 || report.DerivedCompletion == nil || *report.DerivedCompletion {
		t.Fatal("version 3 failed to derive defeat")
	}
	stdout.Reset()
	observations[0]["defeat"], observations[0]["contrary_spans"] = "unknown", []map[string]any{}
	args[len(args)-1] = write("grader-evidence.json", response)
	if err := run(args, &stdout, &stderr); err != nil || json.Unmarshal(stdout.Bytes(), &report) != nil || report.Unresolved != 1 || report.DerivedCompletion != nil {
		t.Fatal("version 3 collapsed uncertainty")
	}
	if err := run(args, failingOutputWriter{}, &stderr); err != devtools.ErrGraderEvidence {
		t.Fatal("version 3 writer error not normalized")
	}
	stdout.Reset()
	observations[0]["status"] = nil
	args[len(args)-1] = write("grader-evidence.json", response)
	if err := run(args, &stdout, &stderr); err != devtools.ErrGraderEvidence || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("version 3 admitted model-supplied final status or emitted rejected text")
	}
}
