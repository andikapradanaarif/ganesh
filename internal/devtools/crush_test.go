package devtools

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func fixtureTree(t *testing.T) (string, string) {
	t.Helper()
	root, source := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "integration/crush"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"runner_test.go.fixture", "turn_ending_test.go.fixture"} {
		if err := os.WriteFile(filepath.Join(root, "integration/crush", name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root, source
}

func TestCrushPlan(t *testing.T) {
	for _, pin := range []string{BaselineRevision, EndingRevision} {
		for _, profile := range []string{"development", "stripped", "collector"} {
			t.Run(pin[:7]+"/"+profile, func(t *testing.T) {
				root, source := fixtureTree(t)
				var calls []invocation
				var temp string
				run := func(_ context.Context, call invocation) ([]byte, error) {
					calls = append(calls, call)
					if call.name == "git" && call.args[0] == "rev-parse" {
						return []byte(pin + "\n"), nil
					}
					if call.name == "go" && call.args[0] == "test" {
						var overlay struct{ Replace map[string]string }
						body, err := os.ReadFile(call.args[4])
						if err != nil || json.Unmarshal(body, &overlay) != nil || len(overlay.Replace) != 2 {
							t.Fatal("invalid overlay")
						}
						temp = filepath.Dir(call.args[4])
						if overlay.Replace[filepath.Join(source, "internal/hooks/ganesh_adapter_test.go")] != filepath.Join(root, "integration/crush/runner_test.go.fixture") {
							t.Fatal("overlay binding changed")
						}
					}
					return nil, nil
				}
				if err := runCrush(context.Background(), root, source, profile, io.Discard, []string{"PATH=/synthetic", "GOPROXY=network", "GANESH_TEST_FIXED_ENDINGS=bad"}, run); err != nil {
					t.Fatal(err)
				}
				if len(calls) != 5 {
					t.Fatal("command count changed")
				}
				for i, want := range [][]string{{"rev-parse", "HEAD"}, {"diff", "--quiet", "HEAD", "--"}, {"ls-files", "--others", "--exclude-standard", "--", "*.go"}} {
					if !reflect.DeepEqual(calls[i].args, want) || calls[i].timeout != 5*time.Second || calls[i].dir != source || calls[i].output != nil {
						t.Fatal("source gate changed")
					}
				}
				build := []string{"build"}
				if profile != "development" {
					build = append(build, "-ldflags=-s -w")
				}
				target := "./cmd/ganesh"
				if profile == "collector" {
					target = "./cmd/ganesh-capture"
				}
				build = append(build, "-o", filepath.Join(temp, "ganesh"), target)
				wantTest := []string{"test", "-p=1", "-count=1", "-overlay", filepath.Join(temp, "overlay.json"), "./internal/hooks", "./internal/agent", "-run", "^TestGanesh(ToolHookAdapter|TurnEndingCancellation)$", "-timeout=20s", "-v"}
				if !reflect.DeepEqual(calls[3].args, build) || !reflect.DeepEqual(calls[4].args, wantTest) || calls[3].dir != root || calls[4].dir != source {
					t.Fatal("fixture command changed")
				}
				for _, call := range calls[3:] {
					if call.timeout != 60*time.Second || call.name != "go" || call.output == nil {
						t.Fatal("stage boundary changed")
					}
					fixed := "GANESH_TEST_FIXED_ENDINGS=0"
					if pin == EndingRevision {
						fixed = "GANESH_TEST_FIXED_ENDINGS=1"
					}
					for _, env := range []string{"GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off", fixed} {
						if !slices.Contains(call.env, env) {
							t.Fatal("offline environment changed")
						}
					}
				}
				if !slices.Contains(calls[4].env, "GANESH_TEST_BINARY="+filepath.Join(temp, "ganesh")) {
					t.Fatal("binary binding missing")
				}
				if _, err := os.Stat(temp); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("temporary overlay retained")
				}
			})
		}
	}
}

func TestCrushFailuresStopWithoutFallback(t *testing.T) {
	for step := 0; step < 5; step++ {
		root, source := fixtureTree(t)
		calls := 0
		run := func(_ context.Context, _ invocation) ([]byte, error) {
			calls++
			if calls == step+1 {
				return nil, errors.New("ghp_SYNTHETIC_SECRET")
			}
			if calls == 1 {
				return []byte(EndingRevision), nil
			}
			return nil, nil
		}
		err := runCrush(context.Background(), root, source, "stripped", io.Discard, nil, run)
		if err == nil || strings.Contains(err.Error(), "ghp_") || calls != step+1 {
			t.Fatal("failed stage escaped or fell back")
		}
	}
	for _, scenario := range []string{"pin", "untracked", "overlay", "symlink", "flags", "profile"} {
		t.Run(scenario, func(t *testing.T) {
			root, source := fixtureTree(t)
			profile, env := "development", []string(nil)
			switch scenario {
			case "overlay", "symlink":
				target := filepath.Join(source, "internal/hooks/ganesh_adapter_test.go")
				if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
					t.Fatal(err)
				}
				var err error
				if scenario == "symlink" {
					err = os.Symlink("/missing", target)
				} else {
					err = os.WriteFile(target, nil, 0o600)
				}
				if err != nil {
					t.Fatal(err)
				}
			case "flags":
				env = []string{"GOFLAGS=ghp_SYNTHETIC_SECRET"}
			case "profile":
				profile = "ghp_SYNTHETIC_SECRET"
			}
			run := func(_ context.Context, call invocation) ([]byte, error) {
				if call.name != "git" {
					t.Fatal("unsafe source reached build")
				}
				if call.args[0] == "rev-parse" {
					if scenario == "pin" {
						return []byte("ghp_SYNTHETIC_SECRET"), nil
					}
					return []byte(EndingRevision), nil
				}
				if scenario == "untracked" && call.args[0] == "ls-files" {
					return []byte("ghp_SYNTHETIC_SECRET.go"), nil
				}
				return nil, nil
			}
			if err := runCrush(context.Background(), root, source, profile, io.Discard, env, run); err == nil || strings.Contains(err.Error(), "ghp_") {
				t.Fatal("unsafe gate accepted or echoed")
			}
		})
	}
}

func TestMetadataOutputLimitAndCancellation(t *testing.T) {
	var output limitedOutput
	if _, err := output.Write(make([]byte, 64<<10)); err != nil {
		t.Fatal(err)
	}
	if _, err := output.Write([]byte("x")); err == nil {
		t.Fatal("unbounded output")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := executeCommand(ctx, invocation{name: "ghp_SYNTHETIC_SECRET", timeout: time.Second}); err != ErrFixture {
		t.Fatal("cancellation must return static failure")
	}
}
