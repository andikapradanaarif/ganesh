package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/s0"
)

func TestCheckReportsUnknownsWithoutStartingAnything(t *testing.T) {
	r, err := s0.NewRun(time.Now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(r)
	path := filepath.Join(t.TempDir(), "run.json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run(context.Background(), []string{"s0", "check", "--input", path, "--phase", "inventory"}, &output); err == nil {
		t.Fatal("missing inputs passed")
	}
	var result s0.Readiness
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.InputsPresent || len(result.Missing) == 0 || result.ProductionNarrativeEnabled {
		t.Fatal("unsafe readiness output")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(body, after) {
		t.Fatal("check modified run record")
	}
}

func TestSubcommandsRejectIrrelevantFlagsBeforeIO(t *testing.T) {
	commands := []struct {
		name    string
		allowed map[string]bool
	}{
		{"prepare", map[string]bool{"dir": true}},
		{"check", map[string]bool{"input": true, "phase": true}},
		{"freeze-targets", map[string]bool{"input": true, "output": true}},
		{"screen", map[string]bool{"input": true, "baseline": true}},
	}
	for _, command := range commands {
		for _, flag := range []string{"dir", "input", "output", "baseline", "phase"} {
			if command.allowed[flag] {
				continue
			}
			t.Run(command.name+"/"+flag, func(t *testing.T) {
				var output bytes.Buffer
				err := run(context.Background(), []string{"s0", command.name, "--" + flag, "ghp_SYNTHETIC_SECRET"}, &output)
				if err == nil || err.Error() != "invalid S0 command flags" || output.Len() != 0 {
					t.Fatalf("irrelevant flag reached command execution: %v %s", err, output.Bytes())
				}
				if strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
					t.Fatal("flag value echoed")
				}
			})
		}
	}
	workspace := filepath.Join(t.TempDir(), "must-not-be-created")
	err := run(context.Background(), []string{"s0", "prepare", "--dir", workspace, "--phase", "heldout"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("prepare accepted phase flag")
	}
	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Fatalf("invalid command created a workspace: %v", err)
	}
}

func TestInvalidRecordsDoNotEchoTheirContents(t *testing.T) {
	for _, body := range []string{`{"raw":"ghp_SYNTHETIC_SECRET"}`, `{} {"raw":"ghp_SYNTHETIC_SECRET"}`, `{"schema_version":`, string(bytes.Repeat([]byte("x"), 1024*1024+1))} {
		path := filepath.Join(t.TempDir(), "bad.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		err := run(context.Background(), []string{"s0", "check", "--input", path}, &output)
		if err == nil || bytes.Contains(output.Bytes(), []byte("SYNTHETIC_SECRET")) || bytes.Contains([]byte(err.Error()), []byte("SYNTHETIC_SECRET")) {
			t.Fatal("unsafe parser output")
		}
	}
}

func TestRunRecordReadabilityWarningIsAdvisoryAndReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file-mode expectations do not attest Windows ACLs")
	}
	r, err := s0.NewRun(time.Now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"fixture-one", "fixture-two", "fixture-three"} {
		r.Projects = append(r.Projects, s0.Project{Path: filepath.Join(t.TempDir(), name), LocalReadPermission: "allowed"})
	}
	value := "fixture-selection-and-permission-reference"
	r.Capture = s0.Capture{Client: &value, Build: &value, Installation: &value, TranscriptAccess: &value}
	body, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []os.FileMode{0600, 0400, 0640, 0604, 0644} {
		t.Run(mode.String(), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ghp_SYNTHETIC_SECRET.json")
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, mode); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := run(context.Background(), []string{"s0", "check", "--input", path}, &output); err != nil {
				t.Fatal(err)
			}
			var result s0.Readiness
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			wantWarnings := []string{}
			if mode&0044 != 0 {
				wantWarnings = append(wantWarnings, "run_record_group_or_other_readable")
			}
			if !reflect.DeepEqual(result.Warnings, wantWarnings) || !result.InputsPresent || result.PermissionsIndependentlyVerified || result.FrozenEvidenceIndependentlyVerified || result.ProductionNarrativeEnabled {
				t.Fatalf("warning changed trust or readiness: %+v", result)
			}
			if strings.Contains(output.String(), "SYNTHETIC_SECRET") || strings.Contains(output.String(), value) {
				t.Fatal("warning leaked a path or record content")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(body, after) {
				t.Fatal("check modified the run record")
			}
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != mode {
				t.Fatal("check changed file permissions")
			}
		})
	}
}
