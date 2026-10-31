package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
