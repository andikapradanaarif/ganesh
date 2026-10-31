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

func TestCompiledCaptureCommand(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "ganesh")
	if output, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	dir := filepath.Join(t.TempDir(), "spool")
	input := `{"schema_version":1,"event_id":"00000000-0000-4000-8000-000000000001","session_id":"00000000-0000-4000-8000-000000000002","event":"tool_result","outcome":"failed"}`
	if _, err := processclock.Start(); err != nil {
		cmd := exec.Command(binary, "capture", "--spool", dir)
		cmd.Stdin = strings.NewReader(input)
		if output, err := cmd.CombinedOutput(); err == nil || !bytes.Contains(output, []byte("unsupported")) {
			t.Fatalf("unsupported clock claimed capture: %s %v", output, err)
		}
		return
	}
	for attempt := range 2 {
		cmd := exec.Command(binary, "capture", "--spool", dir)
		cmd.Stdin = strings.NewReader(input)
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
	// Rejection must not echo unknown fields or leave secret bodies anywhere.
	cmd := exec.Command(binary, "capture", "--spool", dir)
	cmd.Stdin = strings.NewReader(`{"raw":"ghp_SYNTHETIC_SECRET"}`)
	if output, err := cmd.CombinedOutput(); err == nil || bytes.Contains(output, []byte("SYNTHETIC_SECRET")) {
		t.Fatalf("unsafe rejection: %s %v", output, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(body, []byte("SYNTHETIC_SECRET")) {
			t.Fatal("raw fixture persisted")
		}
	}
	// No EOF from stdin must still let the process exit within the budget.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	cmd = exec.Command(binary, "capture", "--spool", dir)
	cmd.Stdin = r
	started := time.Now()
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || time.Since(started) > 2*time.Second || !bytes.Contains(output, []byte("deadline")) {
		t.Fatalf("blocked stdin: %s %v", output, err)
	}
	cmd = exec.Command(binary, "capture", "--spool", dir, "--phase", "ghp_SYNTHETIC_SECRET")
	if output, err := cmd.CombinedOutput(); err == nil || string(output) != "invalid capture command flags\n" {
		t.Fatalf("irrelevant flag: %s %v", output, err)
	}
}
