package devtools

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAuditDevinSourceLinksAndDrift(t *testing.T) {
	evidence := t.TempDir()
	if err := os.Chmod(evidence, 0o700); err != nil {
		t.Fatal(err)
	}
	transcripts := filepath.Join(t.TempDir(), "transcripts")
	if err := os.Mkdir(transcripts, 0o700); err != nil {
		t.Fatal(err)
	}
	grantMapDirectory := filepath.Join(t.TempDir(), "grants")
	if err := os.Mkdir(grantMapDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	projectRoots := map[string]string{"aurum": "/synthetic/aurum", "borneo": "/synthetic/project-a", "valo-backend": "/synthetic/valo", "kiosk": "/synthetic/project-b"}
	writeCorpusFixture(t, grantMapDirectory, "project-roots.json", projectRoots)
	content := []byte("synthetic transcript fixture")
	first := devinIndexRow{ID: "a1", WorkingDirectory: "/synthetic/project-a"}
	second := devinIndexRow{ID: "b2", WorkingDirectory: "/synthetic/project-b"}
	firstManifest := devinManifestRow{SessionID: first.ID, Project: "borneo", WorkingDirectory: first.WorkingDirectory, TranscriptPath: filepath.Join(transcripts, "a1.json"), State: "present_hashed", Bytes: int64(len(content)), SHA256: devinDigest(content), Origin: "unknown_unreconciled"}
	secondManifest := devinManifestRow{SessionID: second.ID, Project: "kiosk", WorkingDirectory: second.WorkingDirectory, TranscriptPath: filepath.Join(transcripts, "b2.json"), State: "missing", Origin: "unknown_unreconciled"}
	restore := func() {
		_ = os.Remove(filepath.Join(transcripts, "a1.json"))
		_ = os.Remove(filepath.Join(transcripts, "b2.json"))
		writeCorpusFixture(t, grantMapDirectory, "project-roots.json", projectRoots)
		if err := os.WriteFile(filepath.Join(transcripts, "a1.json"), content, 0o600); err != nil {
			t.Fatal(err)
		}
		writeCorpusFixture(t, evidence, "devin-sessions.json", []devinIndexRow{first, second})
		writeCorpusLines(t, evidence, "devin-manifest.jsonl", firstManifest, secondManifest)
	}
	restore()
	report, err := AuditDevin(evidence, transcripts, grantMapDirectory)
	if err != nil || report.IndexRows != 2 || report.ManifestRows != 2 || report.Status != "unchanged" || report.OriginalStates["borneo/present_hashed"] != 1 || report.CurrentStates["kiosk/missing"] != 1 || report.OriginalFileBytes != int64(len(content)) || report.CurrentFileBytes != int64(len(content)) || report.ModelDispatchAuthorized {
		t.Fatalf("valid fixture rejected: %+v, %v", report, err)
	}
	root, err := os.OpenRoot(transcripts)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := devinHashFile(root, "a1.json", 0); err != ErrDevinAudit {
		t.Fatal("total-byte limit not enforced")
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(transcripts, "b2.json"), []byte("new file"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err = AuditDevin(evidence, transcripts, grantMapDirectory)
	if err != nil || report.Status != "drift_detected" || report.Drift["newly_present"] != 1 {
		t.Fatal("newly present file not reported")
	}
	restore()
	if err := os.WriteFile(filepath.Join(transcripts, "a1.json"), []byte("modified transcript"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err = AuditDevin(evidence, transcripts, grantMapDirectory)
	if err != nil || report.Status != "drift_detected" || report.Drift["content_changed"] != 1 {
		t.Fatal("content drift not reported")
	}
	restore()
	if err := os.Remove(filepath.Join(transcripts, "a1.json")); err != nil {
		t.Fatal(err)
	}
	report, err = AuditDevin(evidence, transcripts, grantMapDirectory)
	if err != nil || report.Status != "drift_detected" || report.Drift["now_missing"] != 1 {
		t.Fatal("missing file drift not reported")
	}

	for _, tc := range []struct {
		name   string
		change func()
	}{
		{"duplicate index", func() { writeCorpusFixture(t, evidence, "devin-sessions.json", []devinIndexRow{first, first}) }},
		{"index row missing from manifest", func() { writeCorpusLines(t, evidence, "devin-manifest.jsonl", firstManifest) }},
		{"duplicate manifest row", func() { writeCorpusLines(t, evidence, "devin-manifest.jsonl", firstManifest, firstManifest) }},
		{"path escape", func() {
			altered := firstManifest
			altered.TranscriptPath = filepath.Join(transcripts, "..", "else.json")
			writeCorpusLines(t, evidence, "devin-manifest.jsonl", altered, secondManifest)
		}},
		{"malformed hash", func() {
			altered := firstManifest
			altered.SHA256 = "invalid"
			writeCorpusLines(t, evidence, "devin-manifest.jsonl", altered, secondManifest)
		}},
		{"ungranted project path", func() {
			altered := firstManifest
			altered.WorkingDirectory = "/synthetic/other"
			writeCorpusFixture(t, evidence, "devin-sessions.json", []devinIndexRow{{ID: first.ID, WorkingDirectory: altered.WorkingDirectory}, second})
			writeCorpusLines(t, evidence, "devin-manifest.jsonl", altered, secondManifest)
		}},
		{"unclean working directory", func() {
			altered := firstManifest
			altered.WorkingDirectory = "/synthetic/project-a/../elsewhere"
			writeCorpusFixture(t, evidence, "devin-sessions.json", []devinIndexRow{{ID: first.ID, WorkingDirectory: altered.WorkingDirectory}, second})
			writeCorpusLines(t, evidence, "devin-manifest.jsonl", altered, secondManifest)
		}},
		{"extra grant-map project", func() {
			altered := make(map[string]string, len(projectRoots)+1)
			for key, value := range projectRoots {
				altered[key] = value
			}
			altered["nexus"] = "/synthetic/nexus"
			writeCorpusFixture(t, grantMapDirectory, "project-roots.json", altered)
		}},
		{"source symlink", func() {
			if err := os.Remove(filepath.Join(transcripts, "a1.json")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(evidence, "devin-sessions.json"), filepath.Join(transcripts, "a1.json")); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore()
			tc.change()
			if got, err := AuditDevin(evidence, transcripts, grantMapDirectory); err != ErrDevinAudit || !reflect.DeepEqual(got, DevinAuditReport{}) {
				t.Fatal("invalid input accepted or partial report returned")
			}
		})
	}
	restore()
	for _, invalidRoot := range []string{"relative/transcripts", filepath.Join(transcripts, "..", "transcripts") + "/../transcripts"} {
		if got, err := AuditDevin(evidence, invalidRoot, grantMapDirectory); err != ErrDevinAudit || !reflect.DeepEqual(got, DevinAuditReport{}) {
			t.Fatal("non-absolute or unclean transcript root accepted")
		}
	}
	if err := os.Chmod(filepath.Join(evidence, "devin-sessions.json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := AuditDevin(evidence, transcripts, grantMapDirectory); err != ErrDevinAudit || !reflect.DeepEqual(got, DevinAuditReport{}) {
		t.Fatal("non-private evidence accepted")
	}
}
