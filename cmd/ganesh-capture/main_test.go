package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRejectsNonCaptureCommandsWithoutReadingInput(t *testing.T) {
	for _, args := range [][]string{
		nil, {"doctor"}, {"s0", "prepare"}, {"ghp_SYNTHETIC_SECRET"},
		{"capture", "--phase", "ghp_SYNTHETIC_SECRET"},
		{"hook", "crush", "--phase", "ghp_SYNTHETIC_SECRET"}, {"hook", "unknown"},
	} {
		var output bytes.Buffer
		err := run(context.Background(), args, forbiddenReader{}, &output)
		if err == nil || strings.Contains(err.Error(), "SYNTHETIC_SECRET") || output.Len() != 0 {
			t.Fatal("unsupported command read input or exposed it")
		}
	}
}

type forbiddenReader struct{}

func (forbiddenReader) Read([]byte) (int, error) { panic("must not read raw input") }
