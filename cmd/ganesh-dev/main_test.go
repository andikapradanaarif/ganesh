package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	for _, args := range [][]string{nil, {"capture"}, {"crush-test"}, {"crush-test", "--source", "x", "extra"}, {"crush-test", "--source", "x", "--build-profile", "ghp_SYNTHETIC_SECRET"}, {"crush-test", "--source", "x", "--study", "34"}, {"crush-test", "--unknown", "ghp_SYNTHETIC_SECRET"}, {"study-summary"}, {"study-summary", "--dir", "x", "--study", "37"}, {"study-summary", "--dir", "x", "--study", "34", "--source", "ghp_SYNTHETIC_SECRET"}, {"study-summary", "run", "--dir", "x", "--study", "36"}} {
		if _, err := parse(args); err == nil || strings.Contains(err.Error(), "ghp_") {
			t.Fatal("invalid command accepted or echoed")
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
