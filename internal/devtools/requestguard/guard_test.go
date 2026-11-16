package requestguard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type journal struct {
	events []Event
	failAt int
}

func (j *journal) Append(e Event) error {
	j.events = append(j.events, e)
	if len(j.events) == j.failAt {
		return errors.New("synthetic-private-journal-error")
	}
	return nil
}

const complete = `{"status":"completed","usage":{"input_tokens":11,"output_tokens":3},"output":[]}`

func setup(t *testing.T, handler http.HandlerFunc) (*Guard, *journal, *http.Client, string) {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	j := &journal{}
	g, err := New(Config{Endpoint: s.URL + "/v1/responses", Model: "fixture", MaxRequests: 3,
		MaxOutputPerRequest: 8, MaxOutputTotal: 16, StopAfterInputTokens: 100, Timeout: time.Second, MaxBodyBytes: 4096}, j)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(g.Close)
	tr, err := g.Transport(nil)
	if err != nil {
		t.Fatal(err)
	}
	return g, j, &http.Client{Transport: tr}, s.URL + "/v1/responses"
}

func call(c *http.Client, endpoint, body string) error {
	r, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return err
	}
	r.Header.Set("Authorization", "Bearer synthetic-secret-never-in-journal")
	r.Header.Set("Idempotency-Key", "must-not-escape")
	resp, err := c.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(io.Discard, resp.Body)
	return err
}

const request = `{"model":"fixture","max_output_tokens":8,"input":"synthetic-private-body"}`

func variant(n int) string {
	return strings.Replace(request, "synthetic-private-body", fmt.Sprintf("synthetic-private-body-%d", n), 1)
}

func TestFailurePoisonsEveryClientAndPreservesUnknownUsage(t *testing.T) {
	for _, status := range []int{http.StatusServiceUnavailable, http.StatusTooManyRequests, http.StatusTemporaryRedirect} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var sent atomic.Int32
			g, j, c, url := setup(t, func(w http.ResponseWriter, r *http.Request) {
				sent.Add(1)
				w.Header().Set("Location", r.URL.String())
				w.WriteHeader(status)
			})
			other, _ := g.Transport(nil)
			for _, client := range []*http.Client{c, {Transport: other}, c} {
				if call(client, url, request) == nil {
					t.Fatal("failed response accepted")
				}
			}
			state := g.Snapshot()
			if sent.Load() != 1 || state.Attempts != 1 || state.UnknownUsage != 1 || state.KnownInput != 0 || state.Blocked != 2 {
				t.Fatalf("unexpected accounting: %+v, sent=%d", state, sent.Load())
			}
			if len(j.events) != 3 || j.events[1].Usage != nil || j.events[1].Reason != "http_failed" || j.events[2].Kind != "blocked" {
				t.Fatalf("failed usage invented: %+v", j.events)
			}
			b, _ := json.Marshal(j.events)
			if strings.Contains(string(b), "synthetic-private") || strings.Contains(string(b), "synthetic-secret") {
				t.Fatal("payload/auth leaked")
			}
		})
	}
}

func TestOutputReservationAndAuxiliaryAccounting(t *testing.T) {
	var sent atomic.Int32
	g, j, c, url := setup(t, func(w http.ResponseWriter, r *http.Request) {
		sent.Add(1)
		if r.Header.Get("Idempotency-Key") != "" {
			t.Error("replay hint escaped")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, complete)
	})
	for i := 0; i < 3; i++ {
		if err := call(c, url, variant(i)); err != nil {
			t.Fatal(err)
		}
	}
	if call(c, url, variant(3)) == nil {
		t.Fatal("request limit ignored")
	}
	s := g.Snapshot()
	if s.Attempts != 3 || s.KnownInput != 33 || s.KnownOutput != 9 || s.UnknownUsage != 0 || sent.Load() != 3 || len(j.events) != 7 {
		t.Fatalf("auxiliary accounting: %+v", s)
	}

	g2, _, c2, url2 := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"completed","usage":{"input_tokens":1,"output_tokens":8}}`)
	})
	if call(c2, url2, variant(0)) != nil || call(c2, url2, variant(1)) != nil {
		t.Fatal("at-cap requests rejected")
	}
	if call(c2, url2, variant(2)) == nil || g2.Snapshot().Attempts != 2 {
		t.Fatal("total output not reserved before dispatch")
	}
}

func TestRejectsInvalidRequestsBeforeDispatch(t *testing.T) {
	for _, body := range []string{
		`{"model":"other","max_output_tokens":1}`,
		`{"model":"fixture"}`, `{"model":"fixture","max_output_tokens":0}`,
		`{"model":"fixture","max_output_tokens":9}`, `{"model":"fixture","max_output_tokens":1.5}`,
		`{"model":"fixture","max_output_tokens":1,"background":true}`, `broken`, strings.Repeat("x", 4097),
	} {
		t.Run(body[:min(len(body), 45)], func(t *testing.T) {
			g, j, c, url := setup(t, func(http.ResponseWriter, *http.Request) { t.Error("invalid request dispatched") })
			if call(c, url, body) == nil || g.Snapshot().Attempts != 0 || len(j.events) != 0 {
				t.Fatal("invalid request counted/dispatched")
			}
		})
	}
	for _, path := range []string{"/chat/completions", "?secret=synthetic", "/other"} {
		g, _, c, url := setup(t, func(http.ResponseWriter, *http.Request) { t.Error("route escaped") })
		if call(c, url+path, request) == nil || g.Snapshot().Attempts != 0 {
			t.Fatal("route accepted")
		}
	}
}

func TestUsageValidationAndStops(t *testing.T) {
	for _, test := range []struct{ name, body, contentType, reason string }{
		{"missing", `{"status":"completed"}`, "application/json", "usage_unknown"},
		{"negative", `{"status":"completed","usage":{"input_tokens":-1,"output_tokens":3}}`, "application/json", "usage_unknown"},
		{"partial", `{"status":"completed","usage":{"input_tokens":1}}`, "application/json", "usage_unknown"},
		{"fraction", `{"status":"completed","usage":{"input_tokens":1.1,"output_tokens":1}}`, "application/json", "usage_unknown"},
		{"incomplete", `{"status":"incomplete","usage":{"input_tokens":1,"output_tokens":3}}`, "application/json", "usage_unknown"},
		{"overflow", `{"status":"completed","usage":{"input_tokens":1,"output_tokens":9}}`, "application/json", "provider_output_exceeded"},
		{"inputstop", `{"status":"completed","usage":{"input_tokens":100,"output_tokens":3}}`, "application/json", "input_stop"},
		{"truncated", `data: {"type":"response.created"}` + "\n\n", "text/event-stream", "usage_unknown"},
		{"toolarge", strings.Repeat("x", 4097), "application/json", "response_too_large"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var sent atomic.Int32
			g, _, c, url := setup(t, func(w http.ResponseWriter, r *http.Request) {
				sent.Add(1)
				w.Header().Set("Content-Type", test.contentType)
				fmt.Fprint(w, test.body)
			})
			if call(c, url, request) == nil || call(c, url, request) == nil || sent.Load() != 1 || g.Snapshot().Stopped != test.reason {
				t.Fatalf("usage continuation accepted: %+v", g.Snapshot())
			}
		})
	}
}

func TestSSEUsageIncludesAllReportedTokens(t *testing.T) {
	body := "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":" + complete + "}\n\ndata: [DONE]\n\n"
	g, _, c, url := setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		fmt.Fprint(w, body)
	})
	if err := call(c, url, request); err != nil {
		t.Fatal(err)
	}
	if g.Snapshot().KnownInput != 11 || g.Snapshot().KnownOutput != 3 {
		t.Fatal("SSE usage missing")
	}
	for _, bad := range []string{body + body, body + "data: {\"type\":\"response.failed\"}\n\n", "data: [DONE]\n\n", body[:40], strings.TrimSuffix(body, "\n\n"), strings.Replace(body, "event: response.completed", "event: response.failed", 1)} {
		if responseUsage([]byte(bad), "text/event-stream") != nil {
			t.Fatal("invalid stream accepted")
		}
	}
}

func TestSerializesAuxiliaryRequests(t *testing.T) {
	var active, peak atomic.Int32
	g, _, c, url := setup(t, func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		if n > peak.Load() {
			peak.Store(n)
		}
		defer active.Add(-1)
		time.Sleep(10 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, complete)
	})
	var wg sync.WaitGroup
	for i := range 3 {
		wg.Go(func() {
			if err := call(c, url, variant(i)); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if peak.Load() != 1 || g.Snapshot().Attempts != 3 {
		t.Fatal("requests overlapped or disappeared")
	}
}

func TestIdenticalSuccessfulRequestCannotBeReplayed(t *testing.T) {
	var sent atomic.Int32
	g, _, c, url := setup(t, func(w http.ResponseWriter, r *http.Request) {
		sent.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, complete)
	})
	if call(c, url, request) != nil || call(c, url, request) == nil || sent.Load() != 1 || g.Snapshot().Stopped != "request_repeat" || g.Snapshot().UnknownUsage != 0 {
		t.Fatal("successful request replay escaped")
	}
}

func TestJournalFailureStopsBeforeAnyFurtherDispatch(t *testing.T) {
	for _, stage := range []int{1, 2} {
		var sent atomic.Int32
		g, j, c, url := setup(t, func(w http.ResponseWriter, r *http.Request) {
			sent.Add(1)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, complete)
		})
		j.failAt = stage
		if call(c, url, request) == nil || call(c, url, request) == nil || sent.Load() != int32(stage-1) || g.Snapshot().Stopped != "journal_failed" {
			t.Fatal("journal failure failed open")
		}
	}
}

func TestDeadlineAndCallerCancellation(t *testing.T) {
	g, _, c, url := setup(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	})
	timer := time.AfterFunc(20*time.Millisecond, g.Close)
	defer timer.Stop()
	if call(c, url, request) == nil || g.Snapshot().UnknownUsage != 1 {
		t.Fatal("interrupted call accepted")
	}
	if call(c, url, request) == nil {
		t.Fatal("continued after interruption")
	}

	g2, _, c2, url2 := setup(t, func(http.ResponseWriter, *http.Request) { t.Error("cancelled request escaped") })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	r, _ := http.NewRequestWithContext(ctx, http.MethodPost, url2, strings.NewReader(request))
	if _, err := c2.Do(r); err == nil || g2.Snapshot().Attempts != 0 {
		t.Fatal("cancelled request dispatched")
	}
}

func TestFileJournalIsPrivateExclusiveAndDurable(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "ledger.jsonl")
	j, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = j.Append(Event{Kind: "start", Attempt: 1}); err != nil {
		t.Fatal(err)
	}
	if err = j.Close(); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	b, _ := os.ReadFile(path)
	if info.Mode().Perm() != 0600 || !strings.Contains(string(b), `"usage":null`) {
		t.Fatal("private/unknown contract changed")
	}
	if _, err = OpenJournal(path); err == nil {
		t.Fatal("old ledger overwritten")
	}
	if err = os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err = OpenJournal(filepath.Join(dir, "other")); err == nil {
		t.Fatal("public parent accepted")
	}
}
