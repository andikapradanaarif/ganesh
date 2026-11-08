package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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

func TestRepositoryRoot(t *testing.T) {
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal("repository root unavailable")
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal("repository root missing module")
	}
}
