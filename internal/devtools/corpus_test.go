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

func writeCorpusFixture(t *testing.T, dir, name string, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), append(body, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeCorpusLines(t *testing.T, dir, name string, values ...any) {
	t.Helper()
	var body bytes.Buffer
	for _, value := range values {
		if err := json.NewEncoder(&body).Encode(value); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, name), body.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAuditCorpusLinksAndLocalOnlyBoundary(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	raw := corpusRawJSON{SessionID: "s1", ProjectKey: "opaque-key", Title: "A retry failed", Checkpoint: "Contact alice@example.com", Files: "", ParamVersion: "v1", EndedAt: 42}
	manifest := corpusManifest{SessionID: raw.SessionID, Project: "project-a", ProjectKey: raw.ProjectKey, EndedAt: raw.EndedAt, ParamVersion: raw.ParamVersion,
		Title: corpusField{len(raw.Title), corpusHash(raw.Title)}, Checkpoint: corpusField{len(raw.Checkpoint), corpusHash(raw.Checkpoint)}, Files: corpusField{len(raw.Files), corpusHash(raw.Files)},
		Origin: "unknown_unreconciled", ScrubStatus: "raw_unreviewed"}
	counts := make(map[string]int)
	candidate := corpusCandidate{SessionID: raw.SessionID, Project: manifest.Project, EndedAt: raw.EndedAt, ParamVersion: raw.ParamVersion,
		SourceFieldSHA256: map[string]string{"title": manifest.Title.SHA256, "checkpoint": manifest.Checkpoint.SHA256, "files": manifest.Files.SHA256},
		Title:             corpusScrub(raw.Title, counts), Checkpoint: corpusScrub(raw.Checkpoint, counts), Files: corpusScrub(raw.Files, counts),
		Redactions: counts, Origin: "unknown_unreconciled", Status: "candidate_unverified_local_only", ModelDispatch: false}
	writeCorpusFixture(t, dir, "raw.json", []corpusRawJSON{raw})
	writeCorpusFixture(t, dir, "crush-digest-manifest.jsonl", manifest)
	writeCorpusFixture(t, dir, "crush-digest-candidates.jsonl", candidate)
	for _, check := range []struct {
		name string
		dest any
	}{{"raw.json", &[]corpusRawJSON{}}, {"crush-digest-manifest.jsonl", &corpusManifest{}}, {"crush-digest-candidates.jsonl", &corpusCandidate{}}} {
		body, err := os.ReadFile(filepath.Join(dir, check.name))
		if err != nil || decodeStudy(body, check.dest) != nil {
			t.Fatalf("fixture decode failed: %s: %v", check.name, err)
		}
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := corpusRead(root, "raw.json"); err != nil {
		t.Fatalf("fixture private read: %v", err)
	}
	if report, err := AuditCorpus(dir, []string{"raw.json"}); err != nil || report.CandidateRows != 1 || report.Redactions["email"] != 1 || report.ModelDispatchAuthorized {
		t.Fatalf("valid fixture: %+v, %v", report, err)
	}

	restore := func() {
		writeCorpusFixture(t, dir, "raw.json", []corpusRawJSON{raw})
		writeCorpusFixture(t, dir, "crush-digest-manifest.jsonl", manifest)
		writeCorpusFixture(t, dir, "crush-digest-candidates.jsonl", candidate)
	}
	for _, tc := range []struct {
		name   string
		change func()
	}{
		{"source hash mismatch", func() {
			altered := manifest
			altered.Checkpoint.SHA256 = strings.Repeat("0", 64)
			writeCorpusFixture(t, dir, "crush-digest-manifest.jsonl", altered)
		}},
		{"dispatch promotion", func() {
			writeCorpusFixture(t, dir, "crush-digest-manifest.jsonl", manifest)
			altered := candidate
			altered.ModelDispatch = true
			writeCorpusFixture(t, dir, "crush-digest-candidates.jsonl", altered)
		}},
		{"candidate leakage", func() {
			altered := candidate
			altered.Checkpoint = raw.Checkpoint
			altered.ModelDispatch = false
			writeCorpusFixture(t, dir, "crush-digest-candidates.jsonl", altered)
		}},
		{"duplicate raw session", func() {
			writeCorpusFixture(t, dir, "raw.json", []corpusRawJSON{raw, raw})
		}},
		{"duplicate manifest session", func() {
			writeCorpusLines(t, dir, "crush-digest-manifest.jsonl", manifest, manifest)
		}},
		{"missing manifest row", func() {
			writeCorpusLines(t, dir, "crush-digest-manifest.jsonl")
		}},
		{"duplicate candidate session", func() {
			writeCorpusLines(t, dir, "crush-digest-candidates.jsonl", candidate, candidate)
		}},
		{"missing candidate row", func() {
			writeCorpusLines(t, dir, "crush-digest-candidates.jsonl")
		}},
		{"redaction count mismatch", func() {
			altered := candidate
			altered.Redactions = map[string]int{"email": 0}
			writeCorpusFixture(t, dir, "crush-digest-candidates.jsonl", altered)
		}},
		{"undeclared redaction key", func() {
			altered := candidate
			altered.Redactions = map[string]int{"email": 1, "unlisted": 1}
			writeCorpusFixture(t, dir, "crush-digest-candidates.jsonl", altered)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore()
			tc.change()
			if report, err := AuditCorpus(dir, []string{"raw.json"}); err != ErrCorpus || !reflect.DeepEqual(report, CorpusAudit{}) {
				t.Fatal("invalid corpus accepted or partial result returned")
			}
		})
	}
	restore()
	if err := os.Chmod(filepath.Join(dir, "raw.json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := AuditCorpus(dir, []string{"raw.json"}); err == nil {
		t.Fatal("non-private source accepted")
	}
	if _, err := AuditCorpus(dir, []string{"../raw.json"}); err == nil {
		t.Fatal("path traversal accepted")
	}
}
