package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/andikapradanaarif/ganesh/internal/devtools"
)

func TestGraderContractCommand(t *testing.T) {
	for _, args := range [][]string{{"grader-contract-prepare"}, {"grader-contract-prepare", "--dir", "x"}, {"grader-contract-check"}, {"grader-contract-check", "--dir", "x", "--case", "case-01", "--manifest-sha256", strings.Repeat("a", 64)}, {"grader-contract-prepare", "--dir", "x", "--output-dir", "y", "--run"}} {
		var out, errout bytes.Buffer
		if run(args, &out, &errout) != devtools.ErrCommand || out.Len() != 0 || errout.Len() != 0 {
			t.Fatal("incomplete/live command accepted or echoed")
		}
	}
	var out, errout bytes.Buffer
	args := []string{"grader-contract-check", "--dir", "private-canary-does-not-exist", "--case", "case-01", "--manifest-sha256", strings.Repeat("a", 64), "--response-sha256", strings.Repeat("b", 64)}
	if run(args, &out, &errout) != devtools.ErrGraderContract || out.Len() != 0 || errout.Len() != 0 {
		t.Fatal("private path leaked or missing input accepted")
	}
	if _, err := parse([]string{"grader-contract-prepare", "--dir", "x", "--output-dir", "y"}); err != nil {
		t.Fatal("valid prepare arguments")
	}
}
