package main

import (
	"bytes"
	"testing"
)

func TestNoExecutionOrApprovalCommand(t *testing.T) {
	for _, args := range [][]string{{}, {"run"}, {"execute"}, {"approve"}, {"inspect", "-sha256", "bad"}, {"prepare", "-out", "unused", "extra"}} {
		var out bytes.Buffer
		if run(args, &out) == nil || out.Len() != 0 {
			t.Fatal("invalid/live command admitted")
		}
	}
}
