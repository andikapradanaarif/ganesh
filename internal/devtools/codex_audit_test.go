package devtools

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAuditCodexLinksAndDrift(t *testing.T) {
	manifestDir := t.TempDir()
	if err := os.Chmod(manifestDir, 0o700); err != nil {
		t.Fatal(err)
	}
	grantDir := t.TempDir()
	if err := os.Chmod(grantDir, 0o700); err != nil {
		t.Fatal(err)
	}
	sessionsRoot := filepath.Join(t.TempDir(), "sessions")
	rolloutDir := filepath.Join(sessionsRoot, "2026", "11", "08")
	if err := os.MkdirAll(rolloutDir, 0o700); err != nil {
		t.Fatal(err)
	}
	firstPath := filepath.Join(rolloutDir, "rollout-one.jsonl")
	secondPath := filepath.Join(rolloutDir, "rollout-two.jsonl")
	first := []byte("{\"type\":\"session_meta\",\"payload\":{\"cwd\":\"/synthetic/kiosk\"}}\n{\"type\":\"event_msg\"}\n")
	second := []byte("{\"type\":\"session_meta\",\"payload\":{\"cwd\":\"/synthetic/kiosk\"}}\n")
	projectRoots := map[string]string{"aurum": "/synthetic/aurum", "borneo": "/synthetic/borneo", "valo-backend": "/synthetic/valo", "kiosk": "/synthetic/kiosk"}
	manifest := func(pathA, pathB, hashA, hashB string) []byte {
		return []byte(fmt.Sprintf("%s  %s\n%s  %s\n", hashA, pathA, hashB, pathB))
	}
	restore := func() {
		_ = os.Remove(firstPath)
		_ = os.Remove(secondPath)
		if err := os.WriteFile(firstPath, first, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(secondPath, second, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(manifestDir, "codex-kiosk-manifest.txt"), manifest(firstPath, secondPath, devinDigest(first), devinDigest(second)), 0o600); err != nil {
			t.Fatal(err)
		}
		writeCorpusFixture(t, grantDir, "project-roots.json", projectRoots)
	}
	restore()
	report, err := AuditCodex(manifestDir, sessionsRoot, grantDir)
	if err != nil || report.Status != "unchanged" || report.ManifestRows != 2 || report.UnchangedFiles != 2 || report.CurrentFileBytes != int64(len(first)+len(second)) || report.ModelDispatchAuthorized {
		t.Fatalf("valid rollouts rejected: %+v, %v", report, err)
	}
	root, err := os.OpenRoot(sessionsRoot)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(sessionsRoot, firstPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := codexHashFile(root, rel, projectRoots["kiosk"], 0); err != ErrCodexAudit {
		t.Fatal("total-byte limit not enforced")
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(firstPath, append(append([]byte(nil), first...), []byte("change")...), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err = AuditCodex(manifestDir, sessionsRoot, grantDir)
	if err != nil || report.Status != "drift_detected" || report.ChangedFiles != 1 {
		t.Fatal("hash drift not reported")
	}
	restore()
	if err := os.Remove(secondPath); err != nil {
		t.Fatal(err)
	}
	report, err = AuditCodex(manifestDir, sessionsRoot, grantDir)
	if err != nil || report.Status != "drift_detected" || report.MissingFiles != 1 {
		t.Fatal("missing rollout not reported")
	}

	for _, tc := range []struct {
		name   string
		change func()
	}{
		{"duplicate path", func() {
			body := manifest(firstPath, firstPath, devinDigest(first), devinDigest(first))
			if err := os.WriteFile(filepath.Join(manifestDir, "codex-kiosk-manifest.txt"), body, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing manifest row", func() {
			body := []byte(fmt.Sprintf("%s  %s\n", devinDigest(first), firstPath))
			if err := os.WriteFile(filepath.Join(manifestDir, "codex-kiosk-manifest.txt"), body, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"malformed manifest hash", func() {
			body := manifest(firstPath, secondPath, "bad", devinDigest(second))
			if err := os.WriteFile(filepath.Join(manifestDir, "codex-kiosk-manifest.txt"), body, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"path escape", func() {
			body := manifest(firstPath, filepath.Join(sessionsRoot, "..", "elsewhere", "rollout-two.jsonl"), devinDigest(first), devinDigest(second))
			if err := os.WriteFile(filepath.Join(manifestDir, "codex-kiosk-manifest.txt"), body, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"wrong cwd", func() {
			altered := []byte("{\"type\":\"session_meta\",\"payload\":{\"cwd\":\"/synthetic/other\"}}\n")
			if err := os.WriteFile(firstPath, altered, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing session metadata", func() {
			if err := os.WriteFile(firstPath, []byte("{\"type\":\"event_msg\"}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"source symlink", func() {
			if err := os.Remove(firstPath); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(secondPath, firstPath); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlinked directory", func() {
			link := filepath.Join(sessionsRoot, "linked")
			_ = os.Remove(link)
			if err := os.Symlink(rolloutDir, link); err != nil {
				t.Fatal(err)
			}
			body := manifest(filepath.Join(link, "rollout-one.jsonl"), secondPath, devinDigest(first), devinDigest(second))
			if err := os.WriteFile(filepath.Join(manifestDir, "codex-kiosk-manifest.txt"), body, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"extra grant project", func() {
			altered := make(map[string]string, 5)
			for key, value := range projectRoots {
				altered[key] = value
			}
			altered["nexus"] = "/synthetic/nexus"
			writeCorpusFixture(t, grantDir, "project-roots.json", altered)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore()
			tc.change()
			if got, err := AuditCodex(manifestDir, sessionsRoot, grantDir); err != ErrCodexAudit || !reflect.DeepEqual(got, CodexAuditReport{}) {
				t.Fatal("invalid input accepted or partial report returned")
			}
		})
	}
	restore()
	for _, invalidRoot := range []string{"relative/sessions", sessionsRoot + "/../sessions"} {
		if got, err := AuditCodex(manifestDir, invalidRoot, grantDir); err != ErrCodexAudit || !reflect.DeepEqual(got, CodexAuditReport{}) {
			t.Fatal("non-absolute or unclean sessions root accepted")
		}
	}
	if err := os.Chmod(filepath.Join(manifestDir, "codex-kiosk-manifest.txt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := AuditCodex(manifestDir, sessionsRoot, grantDir); err != ErrCodexAudit || !reflect.DeepEqual(got, CodexAuditReport{}) {
		t.Fatal("non-private manifest accepted")
	}
}
