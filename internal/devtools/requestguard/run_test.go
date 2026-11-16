package requestguard

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runFixture(t *testing.T) (Config, RunLimits, RunOptions) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	c := Config{Endpoint: "http://127.0.0.1:1/v1/responses", Model: "fixture", MaxRequests: 6, MaxOutputPerRequest: 8, MaxOutputTotal: 48, StopAfterInputTokens: 1000, Timeout: time.Minute, MaxBodyBytes: 4096}
	l := RunLimits{ID: "synthetic-run", MaxRequests: 3, MaxOutputTotal: 24, InputStop: 1000, Deadline: time.Now().Add(time.Minute).UTC(), Profiles: map[string]Config{"answer": c}, Slots: map[string]string{"one": "answer", "two": "answer", "three": "answer", "four": "answer"}}
	return c, l, RunOptions{Path: filepath.Join(dir, "run.jsonl"), Slot: "one"}
}
func initRun(t *testing.T, l RunLimits, o RunOptions) RunOptions {
	t.Helper()
	hash, err := CreateRun(o.Path, l)
	if err != nil {
		t.Fatal(err)
	}
	o.SHA256 = hash
	return o
}
func syntheticStart() Event {
	return Event{Kind: "start", Attempt: 1, At: time.Now().UTC(), RequestSHA256: hashLine([]byte("synthetic-request")), RequestBytes: 50, RequestedOutput: 8}
}
func syntheticFinish() Event {
	return Event{Kind: "finish", Attempt: 1, At: time.Now().UTC(), Status: 200, Usage: &Usage{Input: 11, Output: 3}}
}

// Child processes use the same library as the client adapter. They never make
// network calls; their synthetic start is fsynced before signalling readiness.
func TestRunProcessHelper(t *testing.T) {
	if os.Getenv("GANESH_RUN_HELPER") == "" {
		return
	}
	var input struct {
		C    Config
		O    RunOptions
		Hold bool
	}
	if json.Unmarshal([]byte(os.Getenv("GANESH_RUN_INPUT")), &input) != nil {
		os.Exit(79)
	}
	g, err := NewShared(input.C, &journal{}, input.O)
	if err != nil {
		os.Exit(78)
	}
	if g.shared.Append(syntheticStart()) != nil {
		os.Exit(78)
	}
	if input.Hold {
		fmt.Fprintln(os.Stdout, "reserved")
		if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
			os.Exit(79)
		}
	}
	if g.shared.Append(syntheticFinish()) != nil {
		os.Exit(78)
	}
	g.Close()
	os.Exit(0)
}

func worker(t *testing.T, c Config, o RunOptions, hold bool) *exec.Cmd {
	t.Helper()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(struct {
		C    Config
		O    RunOptions
		Hold bool
	}{c, o, hold})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, bin, "-test.run=^TestRunProcessHelper$")
	cmd.Env = []string{"GANESH_RUN_HELPER=1", "GANESH_RUN_INPUT=" + string(b)}
	return cmd
}
func expectWorker(t *testing.T, cmd *exec.Cmd, code int) {
	t.Helper()
	err := cmd.Run()
	got := 0
	if err != nil {
		e, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		got = e.ExitCode()
	}
	if got != code {
		t.Fatalf("child exit %d, want %d", got, code)
	}
}

func TestRunTotalsAndCapsSurviveProcesses(t *testing.T) {
	c, l, o := runFixture(t)
	l.MaxRequests = 2
	o = initRun(t, l, o)
	expectWorker(t, worker(t, c, o, false), 0)
	o.Slot = "two"
	expectWorker(t, worker(t, c, o, false), 0)
	s, err := InspectRun(o.Path, o.SHA256)
	if err != nil || s.Requests != 2 || s.KnownInput != 22 || s.KnownOutput != 6 || s.UnknownUsage != 0 || s.SlotsUsed != 2 {
		t.Fatalf("lost cross-process usage: %+v %v", s, err)
	}
	o.Slot = "three"
	expectWorker(t, worker(t, c, o, false), 78)
	s, err = InspectRun(o.Path, o.SHA256)
	if err != nil || s.Requests != 2 || s.KnownOutput != 6 || s.UnknownUsage != 0 || s.Stopped != "run_request_or_output_limit" {
		t.Fatalf("fresh process reset cap: %+v %v", s, err)
	}
	if _, err = CreateRun(o.Path, l); err == nil {
		t.Fatal("existing run overwritten")
	}
}

func TestRunOutputReservationAndInputStop(t *testing.T) {
	for _, kind := range []string{"output", "input"} {
		t.Run(kind, func(t *testing.T) {
			c, l, o := runFixture(t)
			if kind == "output" {
				l.MaxOutputTotal = 8
			} else {
				l.InputStop = 21
			}
			o = initRun(t, l, o)
			expectWorker(t, worker(t, c, o, false), 0)
			o.Slot = "two"
			expectWorker(t, worker(t, c, o, false), 78)
			s, err := InspectRun(o.Path, o.SHA256)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "output" && (s.Requests != 1 || s.KnownOutput != 3 || s.Stopped != "run_request_or_output_limit") {
				t.Fatalf("output reservation reset: %+v", s)
			}
			if kind == "input" && (s.Requests != 2 || s.KnownInput != 22 || s.UnknownUsage != 0 || s.Stopped != "run_input_stop") {
				t.Fatalf("input stop reset: %+v", s)
			}
			o.Slot = "three"
			expectWorker(t, worker(t, c, o, false), 78)
		})
	}
}

func TestRunInterprocessExclusionAndCrash(t *testing.T) {
	for _, crash := range []bool{false, true} {
		t.Run(fmt.Sprint(crash), func(t *testing.T) {
			c, l, o := runFixture(t)
			o = initRun(t, l, o)
			holder := worker(t, c, o, true)
			input, err := holder.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			output, err := holder.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err = holder.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = holder.Process.Kill() })
			line, err := bufio.NewReader(output).ReadString('\n')
			if err != nil || line != "reserved\n" {
				t.Fatalf("missing synced reservation: %q %v", line, err)
			}
			o.Slot = "two"
			expectWorker(t, worker(t, c, o, false), 78)
			if crash {
				_ = holder.Process.Kill()
			} else {
				_, _ = io.WriteString(input, "finish\n")
			}
			err = holder.Wait()
			_ = input.Close()
			if !crash && err != nil {
				t.Fatal(err)
			}
			s, err := InspectRun(o.Path, o.SHA256)
			if err != nil {
				t.Fatal(err)
			}
			if crash {
				if s.Requests != 1 || s.UnknownUsage != 1 || !s.Pending || s.Stopped != "unfinished_request" {
					t.Fatalf("crash reservation refunded: %+v", s)
				}
				expectWorker(t, worker(t, c, o, false), 78)
			} else {
				if s.Requests != 1 || s.KnownInput != 11 || s.Pending {
					t.Fatalf("contender corrupted holder: %+v", s)
				}
				expectWorker(t, worker(t, c, o, false), 0)
			}
		})
	}
}

func TestRunBindingsAndCorruptionFailClosed(t *testing.T) {
	for _, kind := range []string{"profile", "slot", "digest", "truncated", "malformed", "unknown-field", "symlink", "hardlink", "permissions", "reused-slot"} {
		t.Run(kind, func(t *testing.T) {
			c, l, o := runFixture(t)
			o = initRun(t, l, o)
			switch kind {
			case "profile":
				c.MaxRequests++
			case "slot":
				o.Slot = "unfrozen"
			case "digest":
				o.SHA256 = strings.Repeat("0", 64)
			case "truncated", "malformed", "unknown-field":
				f, err := os.OpenFile(o.Path, os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				bad := map[string]string{"truncated": "{", "malformed": "{}\n", "unknown-field": "{\"kind\":\"start\",\"unexpected\":true}\n"}[kind]
				_, err = io.WriteString(f, bad)
				_ = f.Close()
				if err != nil {
					t.Fatal(err)
				}
			case "symlink", "hardlink":
				alias := o.Path + ".alias"
				var err error
				if kind == "symlink" {
					err = os.Symlink(o.Path, alias)
				} else {
					err = os.Link(o.Path, alias)
				}
				if err != nil {
					t.Fatal(err)
				}
				o.Path = alias
			case "permissions":
				if err := os.Chmod(o.Path, 0644); err != nil {
					t.Fatal(err)
				}
			case "reused-slot":
				expectWorker(t, worker(t, c, o, false), 0)
			}
			if g, err := NewShared(c, &journal{}, o); err == nil {
				g.Close()
				t.Fatal("unsafe run/profile accepted")
			}
		})
	}
}

func TestRunUnknownAndJournalFailurePersist(t *testing.T) {
	for _, kind := range []string{"unknown", "local-start", "local-finish", "global-finish"} {
		t.Run(kind, func(t *testing.T) {
			c, l, o := runFixture(t)
			o = initRun(t, l, o)
			local := &journal{}
			if kind == "local-start" {
				local.failAt = 1
			}
			if kind == "local-finish" {
				local.failAt = 2
			}
			g, err := NewShared(c, local, o)
			if err != nil {
				t.Fatal(err)
			}
			defer g.Close()
			err = g.shared.Append(syntheticStart())
			if kind == "local-start" {
				if err == nil {
					t.Fatal("start failure ignored")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				finish := syntheticFinish()
				if kind == "unknown" {
					finish.Usage = nil
					finish.Reason = "http_failed"
					finish.Status = 503
				}
				if kind == "global-finish" {
					_ = g.shared.r.f.Close()
				}
				if g.shared.Append(finish) == nil {
					t.Fatal("failed accounting accepted")
				}
			}
			s, err := InspectRun(o.Path, o.SHA256)
			if err != nil {
				t.Fatal(err)
			}
			if s.Requests != 1 || s.UnknownUsage != 1 || s.KnownInput != 0 || s.Stopped == "" {
				t.Fatalf("unknown usage lost: %+v", s)
			}
			o.Slot = "two"
			expectWorker(t, worker(t, c, o, false), 78)
		})
	}
}

func TestSharedDeadlineCannotReset(t *testing.T) {
	c, l, o := runFixture(t)
	l.Deadline = time.Now().Add(-time.Second).UTC()
	o = initRun(t, l, o)
	expectWorker(t, worker(t, c, o, false), 78)
	s, err := InspectRun(o.Path, o.SHA256)
	if err != nil || s.Requests != 0 || s.Stopped != "deadline" {
		t.Fatalf("deadline reset: %+v %v", s, err)
	}
}

func TestSharedGuardAccountsWireRequests(t *testing.T) {
	_, _, client, url := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, complete)
	})
	_ = client
	c, l, o := runFixture(t)
	c.Endpoint = url
	l.Profiles["answer"] = c
	o = initRun(t, l, o)
	for _, slot := range []string{"one", "two"} {
		o.Slot = slot
		g, err := NewShared(c, &journal{}, o)
		if err != nil {
			t.Fatal(err)
		}
		tr, err := g.Transport(nil)
		if err != nil {
			t.Fatal(err)
		}
		if call(&http.Client{Transport: tr}, url, request) != nil {
			t.Fatal("guarded request failed")
		}
		g.Close()
	}
	s, err := InspectRun(o.Path, o.SHA256)
	if err != nil || s.Requests != 2 || s.KnownInput != 22 || s.KnownOutput != 6 {
		t.Fatalf("guard not bound to run: %+v %v", s, err)
	}
}
