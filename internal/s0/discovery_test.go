package s0

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestDiscoveryUsesOnlyFixedVersionCommands(t *testing.T) {
	var commands [][]string
	results := discover(context.Background(), func(name string) (string, error) { return "/fixture/" + name, nil }, func(_ context.Context, path string, args []string) ([]byte, error) {
		commands = append(commands, append([]string{path}, args...))
		switch path {
		case "/fixture/go":
			return []byte("go version go1.27.3 darwin/amd64\n"), nil
		case "/fixture/codex":
			return []byte("codex-cli 0.166.0\n"), nil
		default:
			return []byte("2.1.295 (Claude Code)\n"), nil
		}
	})
	want := [][]string{{"/fixture/go", "version"}, {"/fixture/codex", "--version"}, {"/fixture/claude", "--version"}}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("unexpected command: %v", commands)
	}
	for _, result := range results {
		if result.Status != "observed" || result.Version == nil || result.CaptureProfileSupported {
			t.Fatalf("wrong discovery status: %+v", result)
		}
	}
}

func TestProbeFailureAndUnexpectedContentDoNotLeak(t *testing.T) {
	results := discover(context.Background(), func(name string) (string, error) {
		if name == "go" {
			return "", errors.New("not present")
		}
		return "/fixture/" + name, nil
	}, func(_ context.Context, path string, _ []string) ([]byte, error) {
		if path == "/fixture/codex" {
			return []byte("codex-cli 0.166.0\nghp_SYNTHETIC_SECRET"), nil
		}
		return nil, errors.New("sk-SYNTHETIC_SECRET")
	})
	if results[0].Status != "not_on_path" || results[1].Status != "version_unrecognized" || results[2].Status != "probe_failed" {
		t.Fatalf("wrong outcomes: %+v", results)
	}
	encoded, _ := json.Marshal(results)
	if strings.Contains(string(encoded), "SYNTHETIC_SECRET") {
		t.Fatal("probe content persisted")
	}
}

func TestProbeContextAndOutputAreBounded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results := discover(ctx, func(name string) (string, error) { return "/fixture/" + name, nil }, func(ctx context.Context, _ string, _ []string) ([]byte, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("probe has no deadline")
		}
		return nil, ctx.Err()
	})
	for _, result := range results {
		if result.Status != "probe_cancelled" {
			t.Fatal(result.Status)
		}
	}
	writer := boundedOutput{limit: 8}
	if _, err := writer.Write([]byte("123456789")); err == nil || len(writer.body) != 0 {
		t.Fatal("unbounded version output")
	}
}
