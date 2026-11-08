package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andikapradanaarif/ganesh/internal/devtools"
)

func TestParse(t *testing.T) {
	for _, args := range [][]string{nil, {"capture"}, {"crush-test"}, {"crush-test", "--source", "x", "extra"}, {"crush-test", "--source", "x", "--build-profile", "ghp_SYNTHETIC_SECRET"}, {"crush-test", "--source", "x", "--study", "34"}, {"crush-test", "--unknown", "ghp_SYNTHETIC_SECRET"}, {"study-summary"}, {"study-summary", "--dir", "x", "--study", "37"}, {"study-summary", "--dir", "x", "--study", "34", "--source", "ghp_SYNTHETIC_SECRET"}, {"study-summary", "run", "--dir", "x", "--study", "36"}, {"corpus-audit"}, {"corpus-audit", "--dir", "x"}, {"corpus-audit", "--raw", "raw.json"}, {"corpus-audit", "--dir", "x", "--raw", "raw.json", "extra"}, {"devin-audit"}, {"devin-audit", "--dir", "x"}, {"devin-audit", "--transcript-root", "/synthetic"}, {"devin-audit", "--dir", "x", "--transcript-root", "/synthetic"}, {"codex-audit"}, {"codex-audit", "--manifest-dir", "x"}, {"codex-audit", "--sessions-root", "/synthetic"}, {"codex-audit", "--manifest-dir", "x", "--sessions-root", "/synthetic"}} {
		if _, err := parse(args); err == nil || strings.Contains(err.Error(), "ghp_") {
			t.Fatal("invalid command accepted or echoed")
		}
	}
	for _, args := range [][]string{{"devin-candidates"}, {"devin-candidates", "--dir", "x", "--transcript-root", "/synthetic", "--grant-map-dir", "x"}, {"devin-candidates", "--output-dir", "x", "--manifest-sha256", strings.Repeat("a", 64)}} {
		if _, err := parse(args); err == nil {
			t.Fatal("incomplete candidate command accepted")
		}
	}
	got, err := parse([]string{"crush-test", "--build-profile", "stripped", "--source", "/synthetic"})
	if err != nil || !reflect.DeepEqual(got, options{command: "crush-test", source: "/synthetic", profile: "stripped"}) {
		t.Fatal("valid command rejected")
	}
	got, err = parse([]string{"study-summary", "--study", "36", "--dir", "/synthetic"})
	if err != nil || got.study != "36" || got.directory != "/synthetic" {
		t.Fatal("valid summary command rejected")
	}
	got, err = parse([]string{"corpus-audit", "--dir", "/synthetic", "--raw", "raw.json"})
	if err != nil || got.directory != "/synthetic" || got.raw != "raw.json" {
		t.Fatal("valid corpus audit command rejected")
	}
	got, err = parse([]string{"devin-audit", "--dir", "/private/evidence", "--transcript-root", "/synthetic/transcripts", "--grant-map-dir", "/private/grants"})
	if err != nil || got.directory != "/private/evidence" || got.transcriptRoot != "/synthetic/transcripts" || got.grantMapDirectory != "/private/grants" {
		t.Fatal("valid Devin audit command rejected")
	}
	got, err = parse([]string{"codex-audit", "--manifest-dir", "/private/evidence", "--sessions-root", "/synthetic/sessions", "--grant-map-dir", "/private/grants"})
	if err != nil || got.directory != "/private/evidence" || got.sessionsRoot != "/synthetic/sessions" || got.grantMapDirectory != "/private/grants" {
		t.Fatal("valid Codex audit command rejected")
	}
	got, err = parse([]string{"devin-candidates", "--dir", "/private/evidence", "--transcript-root", "/synthetic/transcripts", "--grant-map-dir", "/private/grants", "--manifest-sha256", strings.Repeat("a", 64), "--output-dir", "/private/output"})
	if err != nil || got.expectedManifest != strings.Repeat("a", 64) || got.outputDirectory != "/private/output" {
		t.Fatal("valid candidate command rejected")
	}
}

func TestCalibrationCommand(t *testing.T) {
	for _, args := range [][]string{
		{"calibration-summary"},
		{"calibration-summary", "--dir", "x"},
		{"calibration-summary", "--dir", "x", "--frame-sha256", strings.Repeat("G", 64)},
		{"calibration-summary", "--dir", "x", "--frame-sha256", strings.Repeat("a", 64), "extra"},
		{"calibration-summary", "--dir", "x", "--frame-sha256", strings.Repeat("a", 64), "--run"},
	} {
		if _, err := parse(args); err == nil {
			t.Fatal("invalid calibration command accepted")
		}
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	var cases []map[string]any
	for i := range 6 {
		for j := range 3 {
			cases = append(cases, map[string]any{
				"task_id": fmt.Sprintf("synthetic-%d", i), "case_id": fmt.Sprintf("synthetic-%d-%c", i, 'a'+j),
				"completion": nil, "introduced_critical_error": nil, "unsupported_claim": nil, "notes": "",
			})
		}
	}
	body, err := json.Marshal(map[string]any{"cases": cases, "status": "unlabelled", "reviewer_identity": nil, "labelled_at_utc": nil})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "owner-labels.blank.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	// A known independent digest is supplied by the caller, not auto-discovered
	// from whatever packet happens to be on disk.
	sum := sha256.Sum256(body)
	args := []string{"calibration-summary", "--dir", dir, "--frame-sha256", hex.EncodeToString(sum[:])}
	var stdout, stderr bytes.Buffer
	if err := run(args, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	var report devtools.CalibrationReport
	if json.Unmarshal(stdout.Bytes(), &report) != nil || report.Status != "incomplete" || report.Cases != 18 || report.CalibrationAccepted || stderr.Len() != 0 {
		t.Fatal("invalid command output")
	}
	if err := run(args, failingCalibrationWriter{}, &stderr); err != devtools.ErrCalibration {
		t.Fatal("output failure not normalized")
	}
	stdout.Reset()
	args[len(args)-1] = strings.Repeat("0", 64)
	if err := run(args, &stdout, &stderr); err != devtools.ErrCalibration || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("invalid frame emitted a report or private diagnostics")
	}
}

type failingCalibrationWriter struct{}

func (failingCalibrationWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("ghp_SYNTHETIC_CANARY")
}

func TestRepositoryRoot(t *testing.T) {
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal("repository root unavailable")
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal("repository root missing module")
	}
}
