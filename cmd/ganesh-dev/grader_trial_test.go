package main

import (
	"bytes"
	"github.com/andikapradanaarif/ganesh/internal/devtools"
	"strings"
	"testing"
)

func TestGraderTrialCommandsRequireExplicitExecution(t *testing.T) {
	for _, args := range [][]string{{"grader-trial-prepare"}, {"grader-trial-run"}, {"grader-trial-run", "--dir", "x", "--plan-sha256", strings.Repeat("a", 64)}, {"grader-trial-report", "--dir", "x", "--plan-sha256", strings.Repeat("a", 64), "--execute"}} {
		var out, errout bytes.Buffer
		if run(args, &out, &errout) != devtools.ErrCommand || out.Len() != 0 || errout.Len() != 0 {
			t.Fatal("implicit network dispatch or invalid arguments accepted")
		}
	}
}
