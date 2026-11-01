package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/adapter/crush"
	"github.com/andikapradanaarif/ganesh/internal/capture"
	"github.com/andikapradanaarif/ganesh/internal/processclock"
)

const nativeAttempt = `{"event":"PreToolUse","hook_event_name":"PreToolUse","session_id":"00000000-0000-4000-8000-000000000002","cwd":"/synthetic/ghp_SYNTHETIC_SECRET","tool_name":"write","tool_input":{"content":"ghp_SYNTHETIC_SECRET"},"tool_use_id":"call-1"}`

func testCompiledCrushHook(t *testing.T, binary string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "crush-spool")
	run := func(body string, exit int, args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, append([]string{"hook", "crush", "--spool", dir}, args...)...)
		cmd.Stdin = strings.NewReader(body)
		output, err := cmd.CombinedOutput()
		got := 0
		if err != nil {
			var status *exec.ExitError
			if !errors.As(err, &status) {
				t.Fatal(err)
			}
			got = status.ExitCode()
		}
		if len(output) != 0 || got != exit || ctx.Err() != nil {
			t.Fatalf("hook output/exit/deadline: %q %v %v", output, err, ctx.Err())
		}
	}
	if _, err := processclock.Start(); err != nil {
		run(nativeAttempt, 1)
		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("unsupported clock created a capture store")
		}
		return
	}
	run(nativeAttempt, 0)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var recordPath string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			if recordPath != "" {
				t.Fatal("more than one capture effect")
			}
			recordPath = filepath.Join(dir, entry.Name())
		}
	}
	before, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	run(nativeAttempt, 0)
	after, err := os.ReadFile(recordPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("native duplicate changed the durable record")
	}
	success := strings.TrimSuffix(strings.ReplaceAll(nativeAttempt, "PreToolUse", "PostToolUse"), "}") + `,"tool_response":"ghp_SYNTHETIC_SECRET"}`
	failure := strings.TrimSuffix(strings.ReplaceAll(nativeAttempt, "PreToolUse", "PostToolUseFailure"), "}") + `,"error":"ghp_SYNTHETIC_SECRET"}`
	run(success, 0)
	run(failure, 1) // Same call/result identity, contradictory outcome.
	run(strings.Replace(failure, "call-1", "call-2", 1), 0)
	large := strings.Repeat("x", 128*1024) + "ghp_SYNTHETIC_SECRET"
	largeAttempt := strings.ReplaceAll(strings.Replace(nativeAttempt, "call-1", "call-3", 1), "ghp_SYNTHETIC_SECRET", large)
	largeSuccess := strings.ReplaceAll(strings.Replace(success, "call-1", "call-3", 1), "ghp_SYNTHETIC_SECRET", large)
	largeFailure := strings.ReplaceAll(strings.Replace(failure, "call-1", "call-4", 1), "ghp_SYNTHETIC_SECRET", large)
	run(largeAttempt, 0)
	run(largeSuccess, 0)
	run(largeSuccess, 0)
	run(largeFailure, 0)
	permissionRequest := strings.ReplaceAll(nativeAttempt, "PreToolUse", "PermissionRequest")
	permissionResult := strings.ReplaceAll(nativeAttempt, "PreToolUse", "PermissionResult")
	run(permissionRequest, 0)
	run(permissionRequest, 0)
	for _, outcome := range []string{"granted", "denied", "cancelled"} {
		body := strings.TrimSuffix(permissionResult, "}") + `,"decision":"` + outcome + `"}`
		run(body, 0)
		run(body, 0)
	}
	for _, body := range []string{
		`{"raw":"ghp_SYNTHETIC_SECRET"}`,
		strings.ReplaceAll(nativeAttempt, "PreToolUse", "SessionEnd"),
		strings.Repeat(" ", crush.MaxNativeInput) + nativeAttempt,
		strings.Replace(success, `"tool_response":"ghp_SYNTHETIC_SECRET"`, `"tool_response":"`+strings.Repeat("x", crush.MaxNativeInput)+`"`, 1),
		strings.Replace(success, `"tool_response":"ghp_SYNTHETIC_SECRET"`, `"tool_response":"`+strings.Repeat("x", 128*1024)+"\xff\"", 1),
		permissionResult, // An outcome is mandatory on PermissionResult.
		strings.TrimSuffix(permissionResult, "}") + `,"decision":"allow"}`,
		strings.TrimSuffix(permissionResult, "}") + `,"decision":"ghp_SYNTHETIC_SECRET"}`,
		strings.TrimSuffix(permissionRequest, "}") + `,"decision":"granted"}`,
		strings.TrimSuffix(success, "}") + `,"decision":"granted"}`,
	} {
		run(body, 1)
	}
	run(nativeAttempt, 1, "--phase", "ghp_SYNTHETIC_SECRET")
	entries, err = os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	count, failures, requests := 0, 0, 0
	permissionOutcomes := map[string]int{}
	for _, entry := range entries {
		body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"SYNTHETIC_SECRET", "/synthetic", "call-", "tool_input", "tool_response"} {
			if bytes.Contains(body, []byte(forbidden)) {
				t.Fatal("raw native content reached the spool")
			}
		}
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		count++
		var record capture.Record
		if json.Unmarshal(body, &record) != nil || record.State != "captured_local" || record.Origin != "unknown" || record.Coverage != "incomplete" {
			t.Fatal("native capture acquired authority")
		}
		if record.Metadata.Outcome == "failed" {
			failures++
		}
		if record.Metadata.Event == "permission_request" {
			requests++
		}
		if record.Metadata.Event == "permission_result" {
			permissionOutcomes[record.Metadata.Outcome]++
		}
	}
	if count != 10 || failures != 2 || requests != 1 || len(permissionOutcomes) != 3 || permissionOutcomes["granted"] != 1 || permissionOutcomes["denied"] != 1 || permissionOutcomes["cancelled"] != 1 {
		t.Fatalf("native effects: %d records, %d failures", count, failures)
	}
	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatal(err)
	}
	run(nativeAttempt, 1)
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	// No EOF must not keep a memory-only hook alive indefinitely.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	unclosed := strings.Replace(nativeAttempt, "call-1", "call-unclosed", 1)
	if _, err := io.WriteString(w, unclosed); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "hook", "crush", "--spool", dir)
	cmd.Stdin = r
	started := time.Now()
	output, err := cmd.CombinedOutput()
	var status *exec.ExitError
	if !errors.As(err, &status) || status.ExitCode() != 1 || len(output) != 0 || ctx.Err() != nil || time.Since(started) > 2*time.Second {
		t.Fatalf("blocked hook stdin: %q %v %v", output, err, ctx.Err())
	}
	afterUnclosed, err := os.ReadDir(dir)
	if err != nil || len(afterUnclosed) != len(entries) {
		t.Fatal("complete JSON without EOF created storage")
	}
}

func TestHookCommandAdmission(t *testing.T) {
	for _, args := range [][]string{
		nil, {"unknown"}, {"crush"}, {"crush", "--phase", "ghp_SYNTHETIC_SECRET"}, {"crush", "unexpected"},
	} {
		if err := hookCommand(context.Background(), args, strings.NewReader(nativeAttempt)); err == nil || strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
			t.Fatalf("invalid hook command: %v", err)
		}
	}
}

// The child uses the production exit-code dispatcher, with only its handler
// replaced. No panic-injection flag or environment switch enters the real CLI.
func TestHookExitProcess(t *testing.T) {
	if len(os.Args) == 5 && os.Args[2] == "--" && os.Args[3] == "--ganesh-test-hook-exit" {
		mode := os.Args[4]
		os.Exit(commandExitCode([]string{"hook", "crush"}, func() error {
			switch mode {
			case "success":
				return nil
			case "error":
				return errors.New("ghp_SYNTHETIC_SECRET")
			case "panic":
				panic("ghp_SYNTHETIC_SECRET")
			case "nil-panic":
				panic(nil)
			case "runtime-panic":
				var empty []int
				_ = empty[len(os.Args)]
			case "reader-panic":
				ready := make(chan metadataResult, 1)
				go func() { ready <- reduceCaptureInput(context.Background(), panicReader{}, crush.ReduceStream) }()
				return (<-ready).err
			default:
				return errors.New("invalid test case")
			}
			return nil
		}, os.Stderr))
	}
	for _, mode := range []string{"success", "error", "panic", "nil-panic", "runtime-panic", "reader-panic"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHookExitProcess$", "--", "--ganesh-test-hook-exit", mode)
			output, err := cmd.CombinedOutput()
			exit := 0
			if err != nil {
				var status *exec.ExitError
				if !errors.As(err, &status) {
					t.Fatal(err)
				}
				exit = status.ExitCode()
			}
			want := 1
			if mode == "success" {
				want = 0
			}
			if exit != want || len(output) != 0 || ctx.Err() != nil {
				t.Fatalf("hook exit leaked control/diagnostics: status=%d output=%q timeout=%v", exit, output, ctx.Err())
			}
		})
	}
}

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) { panic("ghp_SYNTHETIC_SECRET") }
