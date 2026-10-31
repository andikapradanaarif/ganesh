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
		case "/fixture/claude":
			return []byte("2.1.295 (Claude Code)\n"), nil
		case "/fixture/devin":
			return []byte("devin 3000.11.234 (92c83011d113)\n"), nil
		case "/fixture/crush":
			return []byte("crush version v0.98.3-fork.0.20261027102508-630a3a9dbe00\n"), nil
		default:
			t.Fatalf("unexpected version command: %s", path)
			return nil, nil
		}
	})
	want := [][]string{{"/fixture/go", "version"}, {"/fixture/codex", "--version"}, {"/fixture/claude", "--version"}, {"/fixture/devin", "--version"}, {"/fixture/crush", "--version"}}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("unexpected command: %v", commands)
	}
	for _, result := range results {
		if result.Status != "observed" || result.Version == nil || result.CaptureProfileSupported {
			t.Fatalf("wrong discovery status: %+v", result)
		}
	}
}

func TestDevinAndCrushVersionIdentityAndRejection(t *testing.T) {
	for _, tc := range []struct {
		name, output, version string
	}{
		{"devin", "devin 3000.11.234 (92c83011d113)\n", "3000.11.234 (92c83011d113)"},
		{"devin", "devin 3000.11.234\n", "3000.11.234"},
		{"crush", "crush version v0.98.3-fork.0.20261027102508-630a3a9dbe00\n", "v0.98.3-fork.0.20261027102508-630a3a9dbe00"},
		{"crush", "crush version v0.98.3\n", "v0.98.3"},
		{"crush", "crush version v0.98.3-rc.1+fixture\n", "v0.98.3-rc.1+fixture"},
		{"devin", "devin 3000.11.234 (ghp_SYNTHETIC_SECRET)", ""},
		{"devin", "devin 3000.11.234 (92c83011d113)\nghp_SYNTHETIC_SECRET", ""},
		{"crush", "crush version v0.98.3\nghp_SYNTHETIC_SECRET", ""},
		{"crush", "crush version (devel)", ""},
	} {
		t.Run(tc.name+"/"+tc.output, func(t *testing.T) {
			results := discover(context.Background(), func(name string) (string, error) {
				if name != tc.name {
					return "", errors.New("not on fixture PATH")
				}
				return "/fixture/" + name, nil
			}, func(context.Context, string, []string) ([]byte, error) {
				return []byte(tc.output), nil
			})
			for _, result := range results {
				if result.Name != tc.name {
					continue
				}
				if tc.version == "" {
					if result.Status != "version_unrecognized" || result.Version != nil {
						t.Fatalf("unexpected content accepted: %+v", result)
					}
				} else if result.Status != "observed" || result.Version == nil || *result.Version != tc.version {
					t.Fatalf("version identity changed: %+v", result)
				}
				if result.CaptureProfileSupported {
					t.Fatal("version observation enabled capture")
				}
			}
			encoded, _ := json.Marshal(results)
			if strings.Contains(string(encoded), "SYNTHETIC_SECRET") {
				t.Fatal("unexpected probe content persisted")
			}
		})
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
