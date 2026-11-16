package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/andikapradanaarif/ganesh/internal/devtools/s0package"
)

func TestNoExecutionOrApprovalCommand(t *testing.T) {
	for _, args := range [][]string{{}, {"run"}, {"execute"}, {"approve"}, {"inspect", "-sha256", "bad"}, {"prepare", "-out", "unused", "extra"}} {
		var out bytes.Buffer
		if run(args, &out) == nil || out.Len() != 0 {
			t.Fatal("invalid/live command admitted")
		}
	}
}

func TestMissingOutputIsDiagnosedBeforePackageOrProjects(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{"inspect", "-package", "/nonexistent-package", "-sha256", strings.Repeat("0", 64)}, &out)
	if err == nil || !strings.Contains(s0package.UserError(err), "arguments_output") || out.Len() != 0 {
		t.Fatal("missing output was not diagnosed before access")
	}
}
