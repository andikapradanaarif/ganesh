// requestprofile builds a private, mandatory-guard Crush candidate and exercises
// synthetic loopback fixtures. It has no real provider/key/endpoint option.
package main

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/devtools/requestguard"
)

const pin = "86897c56a304bac133650a6055ab1d091e6b8f83"
const policy = `(version 1) (allow default) (deny network*) (allow network-outbound (remote ip "localhost:*"))`

type obj = map[string]any

func main() {
	source := flag.String("source", "", "local repository containing the exact pinned Crush commit")
	out := flag.String("out", "", "new private output directory (never overwritten)")
	suite := flag.String("suite", "accounting", "accounting (13 cases) or context-tools (one synthetic wire probe)")
	flag.Parse()
	if flag.NArg() != 0 || *source == "" || *out == "" || (*suite != "accounting" && *suite != "context-tools") {
		fmt.Fprintln(os.Stderr, "require -source, -out and a supported -suite")
		os.Exit(2)
	}
	if err := run(*source, *out, *suite); err != nil {
		fmt.Fprintln(os.Stderr, "Offline client fixture failed; inspect private evidence. No real inference requested.")
		os.Exit(1)
	}
}

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func write(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeBytes(path, append(b, '\n'))
}
func writeBytes(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	n, err := f.Write(b)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	return closeErr
}

// Bounded process output is private, never echoed; no account config is read.
type output struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (o *output) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	n := len(p)
	if o.b.Len() < 131072 {
		o.b.Write(p[:min(len(p), 131072-o.b.Len())])
	}
	return n, nil
}

func run(source, out, suite string) error {
	if runtime.GOOS != "darwin" {
		return errors.New("OS sandbox unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	pre := exec.CommandContext(ctx, "/usr/bin/sandbox-exec", "-p", policy, "/usr/bin/true")
	if pre.Run() != nil {
		return errors.New("sandbox preflight failed")
	}
	abs, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	out = abs
	if err = os.Mkdir(out, 0700); err != nil {
		return err
	}
	// This directory is a new private snapshot, not a worktree or an installation.
	snapshot := filepath.Join(out, "source")
	if err = os.Mkdir(snapshot, 0700); err != nil {
		return err
	}
	if err = extract(ctx, source, snapshot); err != nil {
		return err
	}
	guard, err := os.ReadFile("internal/devtools/requestguard/guard.go")
	if err != nil {
		return err
	}
	adapter, err := os.ReadFile("integration/crush/requestprofile/adapter.go.txt")
	if err != nil {
		return err
	}
	telemetryPath := filepath.Join(snapshot, "internal/agent/telemetry.go")
	original, err := os.ReadFile(telemetryPath)
	if err != nil {
		return err
	}
	marker := []byte("instrumented.Transport = &byteCountingTransport{next: base}")
	if bytes.Count(original, marker) != 1 {
		return errors.New("source anchor changed")
	}
	patched := bytes.Replace(original, marker, []byte("instrumented.Transport = ganeshGuardTransport(base)"), 1)
	if err = os.WriteFile(telemetryPath, patched, 0600); err != nil {
		return err
	}
	if err = writeBytes(filepath.Join(snapshot, "internal/ganeshrequestguard/guard.go"), guard); err != nil {
		return err
	}
	guardSources := obj{"guard.go": digest(guard)}
	for _, name := range []string{"run.go", "runlock_unix.go", "runlock_other.go"} {
		b, err := os.ReadFile(filepath.Join("internal/devtools/requestguard", name))
		if err != nil {
			return err
		}
		if err = writeBytes(filepath.Join(snapshot, "internal/ganeshrequestguard", name), b); err != nil {
			return err
		}
		guardSources[name] = digest(b)
	}
	if err = writeBytes(filepath.Join(snapshot, "internal/agent/ganesh_requestguard.go"), adapter); err != nil {
		return err
	}
	binary := filepath.Join(out, "crush-guarded")
	modCache, err := exec.CommandContext(ctx, "go", "env", "GOMODCACHE").Output()
	if err != nil {
		return err
	}
	cache, err := exec.CommandContext(ctx, "go", "env", "GOCACHE").Output()
	if err != nil {
		return err
	}
	build := exec.CommandContext(ctx, "go", "build", "-mod=readonly", "-buildvcs=false", "-p=2", "-o", binary, ".")
	build.Dir = snapshot
	build.Env = []string{"PATH=" + os.Getenv("PATH"), "GOCACHE=" + strings.TrimSpace(string(cache)), "GOMODCACHE=" + strings.TrimSpace(string(modCache)), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off"}
	logs := &output{}
	build.Stdout = logs
	build.Stderr = logs
	fmt.Println("Building pinned client with mandatory request guard; dependencies are offline.")
	err = build.Run()
	if writeErr := writeBytes(filepath.Join(out, "build.private"), logs.b.Bytes()); writeErr != nil {
		return writeErr
	}
	if err != nil {
		return err
	}
	bin, err := os.ReadFile(binary)
	if err != nil {
		return err
	}
	results := []obj{}
	modes := []string{"503", "success", "missing-usage", "request-cap", "missing-config", "missing-run", "shared-request-cap", "shared-output-cap", "shared-input-stop", "shared-unknown", "shared-deadline", "shared-reused-slot", "shared-crash"}
	if suite == "context-tools" {
		modes = []string{"context-tools"}
	}
	for _, mode := range modes {
		fmt.Println("Running synthetic loopback client fixture:", mode)
		result, err := exercise(ctx, out, binary, mode)
		if err != nil {
			return err
		}
		results = append(results, result)
	}
	passed := true
	for _, r := range results {
		passed = passed && r["passed"] == true
	}
	info, err := buildinfo.ReadFile(binary)
	if err != nil {
		return err
	}
	report := obj{"source_revision": pin, "original_telemetry_sha256": digest(original), "patched_telemetry_sha256": digest(patched), "guard_sha256": digest(guard), "guard_sources_sha256": guardSources, "adapter_sha256": digest(adapter), "binary_sha256": digest(bin), "build_go_version": info.GoVersion, "network_policy": policy, "real_inference_requests": 0, "fixtures": results, "passed": passed, "installed_client_modified": false, "run_approved": false, "scope": "Synthetic shared accounting across native Responses client processes; no natural grading, tool isolation, tokenizer/context fit, provider billing or live run approval."}
	if suite == "context-tools" {
		report["scope"] = "Synthetic outgoing Responses context markers and exact four-tool palette only; no tool execution, filesystem permission boundary, actual owner configuration/source inventory, tokenizer/context fit, provider access, natural grading or run approval."
	}
	if err = write(filepath.Join(out, "report.json"), report); err != nil {
		return err
	}
	if !passed {
		return errors.New("fixture assertion failed")
	}
	fmt.Println("Offline client fixtures passed. Real model calls: 0. Installed client unchanged.")
	return nil
}

func extract(ctx context.Context, source, dest string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", source, "archive", "--format=tar", pin, "go.mod", "go.sum", "main.go", "internal")
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()
	r := tar.NewReader(io.LimitReader(pipe, 256<<20))
	var total int64
	for {
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(h.Name)
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") {
			return errors.New("invalid archive path")
		}
		path := filepath.Join(dest, name)
		switch h.Typeflag {
		case tar.TypeXGlobalHeader:
			// git archive includes commit metadata in a global PAX header.
			continue
		case tar.TypeDir:
			if err = os.MkdirAll(path, 0700); err != nil {
				return err
			}
		case tar.TypeReg:
			total += h.Size
			if h.Size < 0 || h.Size > 16<<20 || total > 128<<20 {
				return errors.New("archive exceeds bound")
			}
			b, err := io.ReadAll(r)
			if err != nil {
				return err
			}
			if err = writeBytes(path, b); err != nil {
				return err
			}
		default:
			return errors.New("unexpected archive object")
		}
	}
	return cmd.Wait()
}

func exercise(parent context.Context, out, binary, mode string) (obj, error) {
	dir := filepath.Join(out, mode)
	if err := os.Mkdir(dir, 0700); err != nil {
		return nil, err
	}
	for _, p := range []string{"work", "config", "data", "state", "tmp"} {
		if err := os.Mkdir(filepath.Join(dir, p), 0700); err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithTimeout(parent, 35*time.Second)
	defer cancel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	var mu sync.Mutex
	requests := []obj{}
	active, peak := 0, 0
	hit := make(chan struct{}, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil || len(b) >= 1<<20 {
			w.WriteHeader(400)
			return
		}
		var payload struct {
			Stream    bool   `json:"stream"`
			MaxOutput int64  `json:"max_output_tokens"`
			Model     string `json:"model"`
		}
		_ = json.Unmarshal(b, &payload)
		mu.Lock()
		active++
		peak = max(peak, active)
		n := len(requests) + 1
		observed := obj{"body_sha256": digest(b), "body_bytes": len(b), "path": r.URL.Path, "method": r.Method, "dummy_auth_matches": r.Header.Get("Authorization") == "Bearer fixture-only-not-a-secret", "model": payload.Model, "max_output_tokens": payload.MaxOutput, "stream": payload.Stream}
		if mode == "context-tools" {
			observed["profile"] = observeProfile(b)
		}
		requests = append(requests, observed)
		mu.Unlock()
		defer func() { mu.Lock(); active--; mu.Unlock() }()
		if mode == "shared-crash" {
			select {
			case hit <- struct{}{}:
			default:
			}
			select {
			case <-r.Context().Done():
			case <-ctx.Done():
			}
			return
		}
		if mode == "503" || mode == "shared-unknown" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(503)
			fmt.Fprint(w, `{"error":{"message":"synthetic fixture failure","type":"server_error"}}`)
			return
		}
		usage := obj{"input_tokens": 11, "output_tokens": 3, "total_tokens": 14, "input_tokens_details": obj{"cached_tokens": 0}, "output_tokens_details": obj{"reasoning_tokens": 0}}
		response := obj{"id": fmt.Sprintf("resp_fixture_%d", n), "object": "response", "created_at": 1, "model": "gpt-6-luna", "status": "completed", "output": []obj{{"id": "msg_fixture", "type": "message", "status": "completed", "role": "assistant", "content": []obj{{"type": "output_text", "text": "Synthetic fixture answer.", "annotations": []any{}}}}}, "usage": usage}
		if mode == "missing-usage" {
			delete(response, "usage")
		}
		if !payload.Stream {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		event := func(kind string, value obj) {
			value["type"] = kind
			b, _ := json.Marshal(value)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", kind, b)
		}
		event("response.created", obj{"response": obj{"id": response["id"], "status": "in_progress", "output": []any{}}})
		event("response.output_item.added", obj{"output_index": 0, "item": obj{"id": "msg_fixture", "type": "message", "role": "assistant", "content": []any{}}})
		event("response.content_part.added", obj{"output_index": 0, "content_index": 0, "item_id": "msg_fixture", "part": obj{"type": "output_text", "text": "", "annotations": []any{}}})
		event("response.output_text.delta", obj{"output_index": 0, "content_index": 0, "item_id": "msg_fixture", "delta": "Synthetic fixture answer."})
		event("response.output_text.done", obj{"output_index": 0, "content_index": 0, "item_id": "msg_fixture", "text": "Synthetic fixture answer."})
		event("response.output_item.done", obj{"output_index": 0, "item": response["output"].([]obj)[0]})
		event("response.completed", obj{"response": response})
	})}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()
	base := "http://" + listener.Addr().String() + "/v1"
	maxRequests := int64(6)
	if mode == "request-cap" {
		maxRequests = 1
	}
	limits := requestguard.Config{Endpoint: base + "/responses", Model: "gpt-6-luna", MaxRequests: maxRequests, MaxOutputPerRequest: 256, MaxOutputTotal: 1536, StopAfterInputTokens: 10000, Timeout: 25 * time.Second, MaxBodyBytes: 1 << 20}
	runLimits := requestguard.RunLimits{ID: "fixture-" + mode, MaxRequests: 12, MaxOutputTotal: 4096, InputStop: 10000, Deadline: time.Now().Add(2 * time.Minute).UTC(), Profiles: map[string]requestguard.Config{"answer": limits}, Slots: map[string]string{"one": "answer", "two": "answer", "three": "answer"}}
	switch mode {
	case "shared-request-cap":
		runLimits.MaxRequests = 4
	case "shared-output-cap":
		runLimits.MaxOutputTotal = 259
	case "shared-input-stop":
		runLimits.InputStop = 33
	case "shared-deadline":
		runLimits.Deadline = time.Now().Add(-time.Second).UTC()
	}
	runPath := filepath.Join(dir, "run.jsonl")
	runHash, err := requestguard.CreateRun(runPath, runLimits)
	if err != nil {
		return nil, err
	}
	runOpts := requestguard.RunOptions{Path: runPath, SHA256: runHash, Slot: "one"}
	journalPath := filepath.Join(dir, "requests.jsonl")
	guardConfig := obj{"limits": limits, "journal": journalPath, "run": runOpts}
	if mode == "missing-run" {
		delete(guardConfig, "run")
	}
	if err = write(filepath.Join(dir, "guard.json"), guardConfig); err != nil {
		return nil, err
	}
	model := obj{"id": "gpt-6-luna", "name": "Synthetic fixture only", "context_window": 131072, "default_max_tokens": 256}
	selected := obj{"provider": "opencode-go", "model": "gpt-6-luna", "max_tokens": 256}
	config := obj{"providers": obj{"opencode-go": obj{"id": "opencode-go", "name": "Offline fixture", "type": "openai", "base_url": base, "api_key": "fixture-only-not-a-secret", "discover_models": false, "models": []obj{model}, "extra_headers": obj{"x-opencode-session": "ganesh-request-profile-fixture"}}}, "models": obj{"large": selected, "small": selected, "summary": selected}, "options": obj{"disable_default_providers": true, "disable_provider_auto_update": true, "disable_metrics": true, "disable_auto_summarize": true, "context_paths": []string{"fixture.md"}, "global_context_paths": []string{filepath.Join(dir, "global.md")}, "disabled_tools": []string{"bash", "write", "edit", "multiedit", "agent", "fetch", "web_search", "sourcegraph", "lsp", "diagnostics", "python"}}, "mcp": obj{}}
	if mode == "context-tools" {
		options := config["options"].(obj)
		delete(options, "disabled_tools")
		options["context_paths"] = []string{"fixture.md", "backend/AGENTS.md"}
		options["notebook_enabled"] = false
		options["auto_lsp"] = false
		config["agents"] = obj{"coder": obj{"allowed_tools": probeTools}}
		if err = os.Mkdir(filepath.Join(dir, "skills"), 0700); err != nil {
			return nil, err
		}
		// A real synthetic Git root prevents ancestor project discovery.
		init := exec.CommandContext(ctx, "git", "-c", "init.defaultBranch=fixture", "init", "-q", "--template=", filepath.Join(dir, "work"))
		init.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0"}
		if err = init.Run(); err != nil {
			return nil, err
		}
	}
	if err = write(filepath.Join(dir, "work/crush.json"), config); err != nil {
		return nil, err
	}
	if err = write(filepath.Join(dir, "work/.crush.json"), config); err != nil {
		return nil, err
	}
	contextFiles := map[string]string{"work/fixture.md": "Synthetic fixture. Do not use tools.", "global.md": "Synthetic fixture. Do not use tools."}
	if mode == "context-tools" {
		contextFiles = map[string]string{"work/fixture.md": probeContexts["explicit"], "work/AGENTS.md": probeContexts["native_default"], "work/backend/AGENTS.md": probeContexts["nested_explicit"], "global.md": probeContexts["global_explicit"], "work/unlisted.md": unlistedContext}
	}
	for p, text := range contextFiles {
		if err = writeBytes(filepath.Join(dir, p), []byte(text+"\n")); err != nil {
			return nil, err
		}
	}
	if strings.HasPrefix(mode, "shared-") {
		return exerciseShared(ctx, dir, binary, mode, limits, runOpts, server, hit, func() ([]obj, int) { mu.Lock(); defer mu.Unlock(); return append([]obj(nil), requests...), peak })
	}
	cmd := exec.CommandContext(ctx, "/usr/bin/sandbox-exec", "-p", policy, binary, "--cwd", filepath.Join(dir, "work"), "--data-dir", filepath.Join(dir, "state"), "run", "--quiet", "Return a short synthetic fixture answer without tools.")
	cmd.Dir = filepath.Join(dir, "work")
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/local/bin", "TERM=dumb", "TMPDIR=" + filepath.Join(dir, "tmp"), "CRUSH_GLOBAL_CONFIG=" + filepath.Join(dir, "config"), "CRUSH_GLOBAL_DATA=" + filepath.Join(dir, "data"), "CRUSH_DATA_DIR=" + filepath.Join(dir, "state"), "CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1", "CRUSH_DISABLE_METRICS=1", "CRUSH_CLIENT_SERVER=false", "CRUSH_EVAL_MAX_STEPS=1", "CRUSH_EVAL_MAX_TOKENS=2048", "GANESH_S0_GUARD_CONFIG=" + filepath.Join(dir, "guard.json")}
	if mode == "context-tools" {
		cmd.Env = append(cmd.Env, "CRUSH_SKILLS_DIR="+filepath.Join(dir, "skills"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	}
	if mode == "missing-config" {
		cmd.Env = cmd.Env[:len(cmd.Env)-1]
	}
	stdout, stderr := &output{}, &output{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	started := time.Now().UTC()
	runErr := cmd.Run()
	elapsed := time.Since(started)
	exitCode := 0
	if runErr != nil {
		exitCode = -1
		var exit *exec.ExitError
		if errors.As(runErr, &exit) {
			exitCode = exit.ExitCode()
		}
	}
	if err = server.Close(); err != nil {
		return nil, err
	}
	if err = writeBytes(filepath.Join(dir, "stdout.private"), stdout.b.Bytes()); err != nil {
		return nil, err
	}
	if err = writeBytes(filepath.Join(dir, "stderr.private"), stderr.b.Bytes()); err != nil {
		return nil, err
	}
	ledger, err := os.ReadFile(journalPath)
	if err != nil && !(mode == "missing-config" && os.IsNotExist(err)) {
		return nil, err
	}
	var events []requestguard.Event
	for _, line := range bytes.Split(bytes.TrimSpace(ledger), []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var e requestguard.Event
		if err = json.Unmarshal(line, &e); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	mu.Lock()
	observed := append([]obj(nil), requests...)
	peakSnapshot := peak
	mu.Unlock()
	starts, finishes, known, blocked, inputTokens, outputTokens := 0, 0, 0, 0, int64(0), int64(0)
	bindingsOK := true
	for _, e := range events {
		if e.Kind == "blocked" {
			blocked++
		}
		if e.Kind == "start" {
			starts++
			if starts > len(observed) || e.Attempt != int64(starts) {
				bindingsOK = false
			} else {
				bindingsOK = bindingsOK && e.RequestSHA256 == observed[starts-1]["body_sha256"] && e.RequestBytes == observed[starts-1]["body_bytes"]
			}
		}
		if e.Kind == "finish" {
			finishes++
			expectedStatus := http.StatusOK
			if mode == "503" {
				expectedStatus = http.StatusServiceUnavailable
			}
			bindingsOK = bindingsOK && e.Attempt == int64(finishes) && e.Status == expectedStatus
			if e.Usage != nil {
				known++
				inputTokens += e.Usage.Input
				outputTokens += e.Usage.Output
			}
		}
	}
	passed := bindingsOK && peakSnapshot == 1 && starts == len(observed) && starts == finishes && starts > 0
	for _, r := range observed {
		passed = passed && r["dummy_auth_matches"] == true && r["path"] == "/v1/responses" && r["method"] == http.MethodPost && r["model"] == "gpt-6-luna" && r["max_output_tokens"].(int64) > 0 && r["max_output_tokens"].(int64) <= 256
	}
	switch mode {
	case "503":
		passed = passed && starts == 1 && known == 0 && events[1].Reason == "http_failed" && exitCode == 78 && ctx.Err() == nil
	case "missing-usage":
		passed = passed && starts == 1 && known == 0 && events[1].Reason == "usage_unknown" && exitCode == 78 && ctx.Err() == nil
	case "request-cap":
		passed = passed && starts == 1 && known == 1 && blocked > 0 && exitCode == 78 && ctx.Err() == nil
	case "missing-config":
		passed = len(observed) == 0 && len(events) == 0 && exitCode == 78 && ctx.Err() == nil && !strings.Contains(stdout.b.String(), "Synthetic fixture answer.")
	case "missing-run":
		passed = len(observed) == 0 && len(events) == 0 && exitCode == 78 && ctx.Err() == nil
	case "success":
		passed = passed && runErr == nil && starts >= 2 && known == starts && inputTokens == int64(starts)*11 && outputTokens == int64(starts)*3 && strings.Contains(stdout.b.String(), "Synthetic fixture answer.")
	case "context-tools":
		observations := []profileObservation{}
		for _, request := range observed {
			observations = append(observations, request["profile"].(profileObservation))
		}
		passed = passed && runErr == nil && known == starts && strings.Contains(stdout.b.String(), "Synthetic fixture answer.") && profilePassed(observations)
	}
	return obj{"mode": mode, "started_at_utc": started, "elapsed_ms": elapsed.Milliseconds(), "child_success": runErr == nil, "exit_code": exitCode, "child_deadline": ctx.Err() != nil, "requests": observed, "peak_in_flight": peakSnapshot, "journal_starts": starts, "journal_finishes": finishes, "blocked_events": blocked, "known_usage_requests": known, "unknown_usage_requests": starts - known, "known_input_tokens": inputTokens, "known_output_tokens": outputTokens, "journal_sha256": digest(ledger), "passed": passed}, nil
}
