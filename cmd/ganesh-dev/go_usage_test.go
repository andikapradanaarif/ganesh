package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andikapradanaarif/ganesh/internal/devtools"
)

func TestGoUsageCommand(t *testing.T) {
	for _, args := range [][]string{
		{"go-usage-check"}, {"go-usage-check", "--dir", "x"},
		{"go-usage-check", "--dir", "x", "--usage-sha256", strings.Repeat("G", 64)},
		{"go-usage-check", "--dir", "x", "--usage-sha256", strings.Repeat("a", 64), "extra"},
		{"go-usage-check", "--dir", "x", "--usage-sha256", strings.Repeat("a", 64), "--run"},
	} {
		var stdout, stderr bytes.Buffer
		if err := run(args, &stdout, &stderr); err != devtools.ErrCommand || stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatal("invalid usage command admitted or echoed")
		}
	}
	dir := t.TempDir()
	if os.Chmod(dir, 0o700) != nil {
		t.Fatal("directory permissions")
	}
	usage := map[string]any{"prompt_tokens": 10, "completion_tokens": 2, "total_tokens": 12, "cost": 0, "vendor_note": "ghp_SYNTHETIC_SECRET", "opencode": map[string]any{"billing_source": "go", "allowance": 0.0001, "tokens": map[string]int{"input": 7, "output": 2, "reasoning": 0, "cache_read": 3}}}
	body, err := json.Marshal(usage)
	if err != nil || os.WriteFile(filepath.Join(dir, "go-usage.json"), body, 0o600) != nil {
		t.Fatal("private fixture")
	}
	h := sha256.Sum256(body)
	args := []string{"go-usage-check", "--dir", dir, "--usage-sha256", hex.EncodeToString(h[:])}
	var stdout, stderr bytes.Buffer
	if err := run(args, &stdout, &stderr); err != nil || stderr.Len() != 0 {
		t.Fatal("valid usage command")
	}
	var report devtools.GoUsageReport
	if json.Unmarshal(stdout.Bytes(), &report) != nil || report.PromptTokens != 10 || report.UncachedInputTokens != 7 || report.CacheReadTokens == nil || *report.CacheReadTokens != 3 || report.StandardReasoningTokens != nil || report.ModelIdentityVerified || report.GradingValidated || report.CalibrationAccepted || report.ModelDispatchAuthorized {
		t.Fatal("invalid usage output or promoted authority")
	}
	if strings.Contains(stdout.String(), "ghp_") || strings.Contains(stdout.String(), dir) {
		t.Fatal("private metadata leaked")
	}
	if err := run(args, failingOutputWriter{}, &stderr); err != devtools.ErrGoUsage {
		t.Fatal("writer error not normalized")
	}
	stdout.Reset()
	args[len(args)-1] = strings.Repeat("0", 64)
	if err := run(args, &stdout, &stderr); err != devtools.ErrGoUsage || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("invalid pin output")
	}
	usage["cost"] = json.Number("1e-9999")
	body, err = json.Marshal(usage)
	if err != nil || os.WriteFile(filepath.Join(dir, "go-usage.json"), body, 0o600) != nil {
		t.Fatal("invalid usage fixture")
	}
	h = sha256.Sum256(body)
	args[len(args)-1] = hex.EncodeToString(h[:])
	if err := run(args, &stdout, &stderr); err != devtools.ErrGoUsage || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("tiny nonzero charge or invalid output admitted")
	}
}
