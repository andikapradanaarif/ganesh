package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/devtools/requestguard"
)

func exerciseShared(ctx context.Context, dir, binary, mode string, c requestguard.Config, run requestguard.RunOptions, server *http.Server, hit <-chan struct{}, observe func() ([]obj, int)) (obj, error) {
	children := []obj{}
	events := []requestguard.Event{}
	var launch func(string, string, func() error, bool) error
	launch = func(name, slot string, during func() error, kill bool) error {
		root := filepath.Join(dir, name)
		if err := os.Mkdir(root, 0700); err != nil {
			return err
		}
		for _, p := range []string{"config", "data", "state", "tmp"} {
			if err := os.Mkdir(filepath.Join(root, p), 0700); err != nil {
				return err
			}
		}
		run.Slot = slot
		journal := filepath.Join(root, "requests.jsonl")
		if err := write(filepath.Join(root, "guard.json"), obj{"limits": c, "journal": journal, "run": run}); err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, "/usr/bin/sandbox-exec", "-p", policy, binary, "--cwd", filepath.Join(dir, "work"), "--data-dir", filepath.Join(root, "state"), "run", "--quiet", "Return a short synthetic fixture answer without tools.")
		cmd.Dir = filepath.Join(dir, "work")
		cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/local/bin", "TERM=dumb", "TMPDIR=" + filepath.Join(root, "tmp"), "CRUSH_GLOBAL_CONFIG=" + filepath.Join(root, "config"), "CRUSH_GLOBAL_DATA=" + filepath.Join(root, "data"), "CRUSH_DATA_DIR=" + filepath.Join(root, "state"), "CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1", "CRUSH_DISABLE_METRICS=1", "CRUSH_CLIENT_SERVER=false", "CRUSH_EVAL_MAX_STEPS=1", "CRUSH_EVAL_MAX_TOKENS=2048", "GANESH_S0_GUARD_CONFIG=" + filepath.Join(root, "guard.json")}
		stdout, stderr := &output{}, &output{}
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		started := time.Now().UTC()
		if err := cmd.Start(); err != nil {
			return err
		}
		if kill {
			select {
			case <-hit:
			case <-ctx.Done():
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				return ctx.Err()
			}
			if during != nil {
				if err := during(); err != nil {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
					return err
				}
			}
			_ = cmd.Process.Kill()
		}
		err := cmd.Wait()
		code := 0
		if err != nil {
			code = -1
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				code = exit.ExitCode()
			}
		}
		if err = writeBytes(filepath.Join(root, "stdout.private"), stdout.b.Bytes()); err != nil {
			return err
		}
		if err = writeBytes(filepath.Join(root, "stderr.private"), stderr.b.Bytes()); err != nil {
			return err
		}
		b, err := os.ReadFile(journal)
		if err != nil {
			return err
		}
		localStarts := 0
		for _, line := range bytes.Split(bytes.TrimSpace(b), []byte{'\n'}) {
			if len(line) == 0 {
				continue
			}
			var e requestguard.Event
			if err = json.Unmarshal(line, &e); err != nil {
				return err
			}
			events = append(events, e)
			if e.Kind == "start" {
				localStarts++
			}
		}
		children = append(children, obj{"name": name, "slot": slot, "exit_code": code, "killed_after_stub_observed": kill, "started_at_utc": started, "elapsed_ms": time.Since(started).Milliseconds(), "local_starts": localStarts, "journal_sha256": digest(b)})
		return nil
	}
	if mode == "shared-crash" {
		if err := launch("holder", "one", func() error { return launch("contender", "two", nil, false) }, true); err != nil {
			return nil, err
		}
		if err := launch("recovery", "three", nil, false); err != nil {
			return nil, err
		}
	} else {
		if err := launch("first", "one", nil, false); err != nil {
			return nil, err
		}
		slot := "two"
		if mode == "shared-reused-slot" {
			slot = "one"
		}
		if err := launch("second", slot, nil, false); err != nil {
			return nil, err
		}
		if mode == "shared-request-cap" {
			if err := launch("third", "three", nil, false); err != nil {
				return nil, err
			}
		}
	}
	if err := server.Close(); err != nil {
		return nil, err
	}
	observed, peak := observe()
	s, err := requestguard.InspectRun(run.Path, run.SHA256)
	if err != nil {
		return nil, err
	}
	ledger, err := os.ReadFile(run.Path)
	if err != nil {
		return nil, err
	}
	starts := map[string]int{}
	finishes, knownInput, knownOutput := int64(0), int64(0), int64(0)
	for _, e := range events {
		if e.Kind == "start" {
			starts[e.RequestSHA256]++
		}
		if e.Kind == "finish" {
			finishes++
			if e.Usage != nil {
				knownInput += e.Usage.Input
				knownOutput += e.Usage.Output
			}
		}
	}
	passed := ctx.Err() == nil && s.Requests == int64(len(observed)) && peak <= 1 && knownInput == s.KnownInput && knownOutput == s.KnownOutput
	for _, r := range observed {
		hash := r["body_sha256"].(string)
		passed = passed && starts[hash] > 0
		starts[hash]--
		passed = passed && r["dummy_auth_matches"] == true && r["method"] == http.MethodPost && r["path"] == "/v1/responses" && r["model"] == "gpt-6-luna" && r["max_output_tokens"].(int64) > 0 && r["max_output_tokens"].(int64) <= 256
	}
	for _, n := range starts {
		passed = passed && n == 0
	}
	exits := map[string]int{}
	for _, child := range children {
		exits[child["name"].(string)] = child["exit_code"].(int)
	}
	switch mode {
	case "shared-request-cap":
		passed = passed && exits["first"] == 0 && exits["second"] == 0 && exits["third"] == 78 && s.Requests == 4 && s.KnownInput == 44 && s.KnownOutput == 12 && s.UnknownUsage == 0 && s.Stopped == "run_request_or_output_limit"
	case "shared-output-cap":
		passed = passed && exits["first"] == 0 && exits["second"] == 78 && s.Requests == 3 && s.KnownInput == 33 && s.KnownOutput == 9 && s.UnknownUsage == 0 && s.Stopped == "run_request_or_output_limit"
	case "shared-input-stop":
		passed = passed && exits["first"] == 0 && exits["second"] == 78 && s.Requests == 3 && s.KnownInput == 33 && s.KnownOutput == 9 && s.UnknownUsage == 0 && s.Stopped == "run_input_stop"
	case "shared-unknown":
		passed = passed && exits["first"] == 78 && exits["second"] == 78 && s.Requests == 1 && s.UnknownUsage == 1 && s.Stopped == "http_failed"
	case "shared-deadline":
		passed = passed && exits["first"] == 78 && exits["second"] == 78 && s.Requests == 0 && s.Stopped == "deadline"
	case "shared-reused-slot":
		passed = passed && exits["first"] == 0 && exits["second"] == 78 && s.Requests == 2 && s.SlotsUsed == 1 && s.KnownInput == 22 && s.KnownOutput == 6
	case "shared-crash":
		passed = passed && exits["holder"] == -1 && exits["contender"] == 78 && exits["recovery"] == 78 && s.Requests == 1 && s.UnknownUsage == 1 && s.Pending && s.Stopped == "unfinished_request"
	}
	return obj{"mode": mode, "children": children, "requests": observed, "peak_in_flight": peak, "run_header_sha256": run.SHA256, "run_ledger_sha256": digest(ledger), "run_summary": s, "local_finishes": finishes, "passed": passed}, nil
}
