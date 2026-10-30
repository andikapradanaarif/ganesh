package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/processclock"
)

func TestCompiledCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "ganesh")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	input := filepath.Join(t.TempDir(), "run.json")
	if output, err := exec.Command(binary, "s0", "init", "--output", input).CombinedOutput(); err != nil {
		t.Fatalf("init: %v %s", err, output)
	}
	if err := exec.Command(binary, "s0", "init", "--output", input).Run(); err == nil {
		t.Fatal("overwrote run configuration")
	}
	check := exec.Command(binary, "s0", "check", "--input", input)
	var stdout, stderr bytes.Buffer
	check.Stdout = &stdout
	check.Stderr = &stderr
	if err := check.Run(); err == nil {
		t.Fatal("unfilled preparation passed")
	}
	var result struct {
		Missing []string `json:"missing"`
		Started bool     `json:"held_out_runs_started"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || len(result.Missing) == 0 || result.Started {
		t.Fatalf("unsafe preparation: %s %v", stdout.Bytes(), err)
	}
	if _, err := processclock.Start(); err != nil {
		t.Skip("capture clock unavailable; preparation CLI checked")
	}
	spool := filepath.Join(t.TempDir(), "spool")
	metadata := `{"schema_version":1,"event_id":"00000000-0000-4000-8000-000000000001","session_id":"00000000-0000-4000-8000-000000000002","event":"tool_result","outcome":"failed"}`
	for attempt := range 2 {
		cmd := exec.Command(binary, "capture", "--spool", spool)
		cmd.Stdin = strings.NewReader(metadata)
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("capture: %v", err)
		}
		var receipt struct {
			State     string  `json:"state"`
			Origin    string  `json:"origin"`
			Coverage  string  `json:"coverage"`
			Duplicate bool    `json:"duplicate"`
			Duration  float64 `json:"process_start_to_receipt_ms"`
		}
		if err := json.Unmarshal(output, &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.State != "captured_local" || receipt.Origin != "unknown" || receipt.Coverage != "incomplete" || receipt.Duplicate != (attempt == 1) || receipt.Duration <= 0 || receipt.Duration >= 750 {
			t.Fatalf("receipt: %s", output)
		}
	}
	bad := exec.Command(binary, "capture", "--spool", spool)
	bad.Stdin = strings.NewReader(`{"raw_tool_output":"ghp_SYNTHETIC_SECRET"}`)
	output, err := bad.CombinedOutput()
	if err == nil || bytes.Contains(output, []byte("SYNTHETIC_SECRET")) {
		t.Fatalf("unsafe rejection: %s", output)
	}
	for _, entry := range mustReadDir(t, spool) {
		body, err := os.ReadFile(filepath.Join(spool, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(body, []byte("SYNTHETIC_SECRET")) {
			t.Fatal("secret fixture stored")
		}
	}
	// An input pipe that never reaches EOF cannot hold the collector forever.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	blocked := exec.Command(binary, "capture", "--spool", spool)
	blocked.Stdin = r
	start := time.Now()
	output, err = blocked.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || time.Since(start) > 2*time.Second || !bytes.Contains(output, []byte("deadline")) {
		t.Fatalf("input deadline: %s %v", output, err)
	}
}

func mustReadDir(t *testing.T, path string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}
