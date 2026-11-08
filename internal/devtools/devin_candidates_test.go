package devtools

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func excerptFixtureBody(t *testing.T, steps []any) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{"schema_version": "ATIF-v1.7", "session_id": "a1", "steps": steps})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestSelectDevinExcerptDoesNotFallBackOrExportOtherFields(t *testing.T) {
	steps := []any{
		map[string]any{"source": "system", "message": "system-private"},
		map[string]any{"source": "user", "message": "user-private"},
		map[string]any{"source": "agent", "message": "earlier handoff", "tool_calls": nil},
		map[string]any{"source": "agent", "message": "selected handoff", "reasoning_content": "reasoning-private", "observation": "tool-private", "tool_calls": []any{}},
		map[string]any{"source": "agent", "message": nil, "tool_calls": []any{map[string]any{"arguments": "tool-arguments-private"}}},
	}
	message, index, reason := selectDevinExcerpt(excerptFixtureBody(t, steps), "a1")
	if reason != "" || message != "selected handoff" || index != 3 {
		t.Fatal("wrong recorded message selected")
	}
	steps = append(steps, map[string]any{"source": "agent", "message": strings.Repeat("x", 16<<10+1)})
	if message, _, reason := selectDevinExcerpt(excerptFixtureBody(t, steps), "a1"); message != "" || reason != "message_over_16_kib" {
		t.Fatal("oversized last message silently fell back or truncated")
	}
	if _, _, reason := selectDevinExcerpt([]byte(`{"schema_version":"ATIF-v1.7","session_id":"a1","steps":[],"steps":[]}`), "a1"); reason != "malformed_transcript" {
		t.Fatal("duplicate source keys accepted")
	}
	if _, _, reason := selectDevinExcerpt(excerptFixtureBody(t, []any{map[string]any{"source": "agent", "Message": "aliased"}}), "a1"); reason != "malformed_step" {
		t.Fatal("aliased message field accepted")
	}
	if _, _, reason := selectDevinExcerpt(excerptFixtureBody(t, steps), "different"); reason != "session_identity_mismatch" {
		t.Fatal("foreign source identity accepted")
	}
}

func TestPrepareDevinCandidatesPrivateSourceBinding(t *testing.T) {
	privateDir := func() string {
		dir := t.TempDir()
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	evidence, grants, output, sources := privateDir(), privateDir(), privateDir(), privateDir()
	message := "Handoff: contact alice@example.com; token ghp_12345678901234567890; path /Users/synthetic/work."
	body := excerptFixtureBody(t, []any{
		map[string]any{"source": "user", "message": "user-private"},
		map[string]any{"source": "agent", "message": message, "reasoning_content": "reasoning-private", "observation": "tool-private"},
	})
	path := filepath.Join(sources, "a1.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	writeCorpusFixture(t, grants, "project-roots.json", map[string]string{"aurum": "/synthetic/aurum", "borneo": "/synthetic/borneo", "valo-backend": "/synthetic/valo", "kiosk": "/synthetic/kiosk"})
	writeCorpusFixture(t, evidence, "devin-sessions.json", []devinIndexRow{{ID: "a1", WorkingDirectory: "/synthetic/borneo"}, {ID: "b2", WorkingDirectory: "/synthetic/kiosk"}})
	writeCorpusLines(t, evidence, "devin-manifest.jsonl",
		devinManifestRow{SessionID: "a1", Project: "borneo", WorkingDirectory: "/synthetic/borneo", TranscriptPath: path, State: "present_hashed", Bytes: int64(len(body)), SHA256: devinDigest(body), Origin: "unknown_unreconciled"},
		devinManifestRow{SessionID: "b2", Project: "kiosk", WorkingDirectory: "/synthetic/kiosk", TranscriptPath: filepath.Join(sources, "b2.json"), State: "missing", Origin: "unknown_unreconciled"})
	manifest, err := os.ReadFile(filepath.Join(evidence, "devin-manifest.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	pin := devinDigest(manifest)
	report, err := PrepareDevinCandidates(evidence, sources, grants, pin, output)
	if err != nil || report.Rows != 2 || report.Candidates != 1 || report.Exclusions["source_missing"] != 1 || report.ModelDispatch {
		t.Fatalf("valid source preparation failed: %+v %v", report, err)
	}
	artifactPath := filepath.Join(output, "devin-excerpts.json")
	artifact, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(artifactPath)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("candidate artifact is not private")
	}
	if report.ArtifactSHA256 != devinDigest(artifact) {
		t.Fatal("artifact digest does not bind output")
	}
	for _, forbidden := range []string{"user-private", "reasoning-private", "tool-private", "alice@example.com", "ghp_12345678901234567890", "/Users/synthetic/work"} {
		if bytes.Contains(artifact, []byte(forbidden)) {
			t.Fatal("omitted or sensitive fixture content persisted")
		}
	}
	var batch devinExcerptBatch
	if json.Unmarshal(artifact, &batch) != nil || len(batch.Records) != 2 || batch.ManifestSHA256 != pin {
		t.Fatal("candidate source manifest is not bound")
	}
	candidate := batch.Records[0]
	if candidate.MessageSHA256 != corpusHash(message) || candidate.SourceSHA256 != devinDigest(body) || candidate.TextSHA256 != corpusHash(candidate.Text) || candidate.StepIndex == nil || *candidate.StepIndex != 1 || candidate.Origin != "unknown_unreconciled" || candidate.Closure != "unknown" || candidate.ModelDispatch {
		t.Fatal("candidate source links or boundaries invalid")
	}
	if batch.Records[1].Text != "" || batch.Records[1].Exclusion != "source_missing" {
		t.Fatal("missing source gained text")
	}
	if got, err := PrepareDevinCandidates(evidence, sources, grants, pin, output); err != ErrDevinCandidates || !reflect.DeepEqual(got, DevinCandidateReport{}) {
		t.Fatal("existing candidate file overwritten or partial report returned")
	}
	after, err := os.ReadFile(artifactPath)
	if err != nil || !bytes.Equal(artifact, after) {
		t.Fatal("existing output changed")
	}
	if got, err := PrepareDevinCandidates(evidence, sources, grants, strings.Repeat("0", 64), privateDir()); err != ErrDevinCandidates || !reflect.DeepEqual(got, DevinCandidateReport{}) {
		t.Fatal("unmatched source pin accepted")
	}
	if err := os.WriteFile(path, append(body, ' '), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := PrepareDevinCandidates(evidence, sources, grants, pin, privateDir()); err != ErrDevinCandidates || !reflect.DeepEqual(got, DevinCandidateReport{}) {
		t.Fatal("source drift accepted")
	}
	unsafe := t.TempDir()
	if err := os.Chmod(unsafe, 0o755); err != nil {
		t.Fatal(err)
	}
	if privateCandidateOutput(unsafe, []byte("{}")) != ErrDevinCandidates {
		t.Fatal("unsafe output directory accepted")
	}
}
