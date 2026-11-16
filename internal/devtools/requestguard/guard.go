// Package requestguard bounds one experimental native Responses client process.
// It is developer tooling, not a production gateway or an execution grant.
package requestguard

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var ErrStopped = errors.New("experimental request guard stopped")

type Config struct {
	Endpoint             string        `json:"endpoint"`
	Model                string        `json:"model"`
	MaxRequests          int64         `json:"max_requests"`
	MaxOutputPerRequest  int64         `json:"max_output_per_request"`
	MaxOutputTotal       int64         `json:"max_output_total"`
	StopAfterInputTokens int64         `json:"stop_after_input_tokens"`
	Timeout              time.Duration `json:"timeout"`
	MaxBodyBytes         int64         `json:"max_body_bytes"`
}

type Usage struct {
	Input  int64 `json:"input_tokens"`
	Output int64 `json:"output_tokens"`
}

// A start without a finish means potentially dispatched, usage unknown.
// A failed request's nil Usage never means zero billed tokens.
type Event struct {
	Kind          string    `json:"kind"`
	Attempt       int64     `json:"attempt"`
	At            time.Time `json:"at"`
	Reason        string    `json:"reason,omitempty"`
	Status        int       `json:"status,omitempty"`
	RequestSHA256 string    `json:"request_sha256,omitempty"`
	RequestBytes  int       `json:"request_bytes,omitempty"`
	Usage         *Usage    `json:"usage"`
}

type Journal interface{ Append(Event) error }

type FileJournal struct{ f *os.File }

// OpenJournal requires a private parent and a new file. Restart/resume is not
// supported: callers must reconcile old starts, never overwrite their evidence.
func OpenJournal(path string) (*FileJournal, error) {
	parent, err := os.Stat(filepath.Dir(path))
	if err != nil || !parent.IsDir() || parent.Mode().Perm()&0077 != 0 {
		return nil, ErrStopped
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, ErrStopped
	}
	return &FileJournal{f}, nil
}

func (j *FileJournal) Append(e Event) error {
	b, err := json.Marshal(e)
	if err != nil {
		return ErrStopped
	}
	n, err := j.f.Write(append(b, '\n'))
	if err != nil || n != len(b)+1 || j.f.Sync() != nil {
		return ErrStopped
	}
	return nil
}

func (j *FileJournal) Close() error { return j.f.Close() }

type Snapshot struct {
	Attempts     int64  `json:"attempts"`
	KnownInput   int64  `json:"known_input_tokens"`
	KnownOutput  int64  `json:"known_output_tokens"`
	UnknownUsage int64  `json:"unknown_usage_requests"`
	Blocked      int64  `json:"blocked"`
	Stopped      string `json:"stopped,omitempty"`
}

// Guard serializes complete HTTP responses, including auxiliary calls, across
// every client sharing this instance. Responses are buffered before SDK delivery
// so usage is checked before a subsequent call can leave the process.
type Guard struct {
	c         Config
	journal   Journal
	ctx       context.Context
	cancel    context.CancelFunc
	gate      chan struct{}
	mu        sync.Mutex
	journalMu sync.Mutex
	s         Snapshot
	seen      map[[32]byte]bool
}

func New(c Config, j Journal) (*Guard, error) {
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		(u.Scheme != "https" && !(u.Scheme == "http" && strings.HasPrefix(u.Host, "127.0.0.1:"))) ||
		!strings.HasSuffix(u.Path, "/responses") || c.Model == "" || j == nil ||
		c.MaxRequests <= 0 || c.MaxRequests > 10000 || c.MaxOutputPerRequest <= 0 ||
		c.MaxOutputTotal < c.MaxOutputPerRequest || c.MaxOutputTotal > 1e9 ||
		c.StopAfterInputTokens <= 0 || c.StopAfterInputTokens > 1e9 ||
		c.Timeout <= 0 || c.Timeout > time.Hour || c.MaxBodyBytes <= 0 || c.MaxBodyBytes > 16<<20 {
		return nil, ErrStopped
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	g := &Guard{c: c, journal: j, ctx: ctx, cancel: cancel, gate: make(chan struct{}, 1), seen: make(map[[32]byte]bool)}
	g.gate <- struct{}{}
	return g, nil
}

func (g *Guard) Close()             { g.cancel() }
func (g *Guard) Snapshot() Snapshot { g.mu.Lock(); defer g.mu.Unlock(); return g.s }

// Transport accepts only a plain transport. A custom OAuth/debug transport could
// bypass this accounting or log payloads and must be qualified separately.
func (g *Guard) Transport(base http.RoundTripper) (http.RoundTripper, error) {
	if base == nil {
		base = http.DefaultTransport
	}
	t, ok := base.(*http.Transport)
	if !ok {
		return nil, ErrStopped
	}
	t = t.Clone()
	t.DisableKeepAlives = true // no Go transport replay on a reused connection
	t.ForceAttemptHTTP2 = false
	t.Protocols = new(http.Protocols)
	t.Protocols.SetHTTP1(true)
	t.Proxy = nil // no environment-selected destination or credentials
	return &transport{g: g, next: t}, nil
}

type transport struct {
	g    *Guard
	next http.RoundTripper
}

func (g *Guard) stop(reason string) error {
	g.mu.Lock()
	if g.s.Stopped == "" {
		g.s.Stopped = reason
	}
	g.mu.Unlock()
	g.cancel()
	return ErrStopped
}

func (g *Guard) event(e Event) error {
	e.At = time.Now().UTC()
	g.journalMu.Lock()
	err := g.journal.Append(e)
	g.journalMu.Unlock()
	if err != nil {
		return g.stop("journal_failed")
	}
	return nil
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	g := t.g
	if req.Body != nil {
		defer req.Body.Close()
	}
	select {
	case <-g.ctx.Done():
		return nil, g.block()
	case <-req.Context().Done():
		return nil, g.stop("cancelled")
	case <-g.gate:
	}
	defer func() { g.gate <- struct{}{} }()
	if g.ctx.Err() != nil {
		return nil, g.block()
	}
	if req.Context().Err() != nil {
		return nil, g.stop("cancelled")
	}
	if req.Method != http.MethodPost || req.URL.String() != g.c.Endpoint || req.Body == nil ||
		req.Host != "" && req.Host != req.URL.Host {
		return nil, g.stop("request_route")
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, g.c.MaxBodyBytes+1))
	if err != nil || int64(len(body)) > g.c.MaxBodyBytes {
		return nil, g.stop("request_body")
	}
	var input struct {
		Model      string `json:"model"`
		MaxOutput  *int64 `json:"max_output_tokens"`
		Background bool   `json:"background"`
	}
	if json.Unmarshal(body, &input) != nil || input.Model != g.c.Model || input.Background ||
		input.MaxOutput == nil || *input.MaxOutput <= 0 || *input.MaxOutput > g.c.MaxOutputPerRequest {
		return nil, g.stop("request_profile")
	}
	h := sha256.Sum256(body)
	g.mu.Lock()
	if g.seen[h] {
		g.mu.Unlock()
		if g.event(Event{Kind: "blocked", Reason: "request_repeat"}) != nil {
			return nil, ErrStopped
		}
		return nil, g.stop("request_repeat")
	}
	if g.s.Attempts >= g.c.MaxRequests || *input.MaxOutput > g.c.MaxOutputTotal-g.s.KnownOutput {
		g.mu.Unlock()
		if g.event(Event{Kind: "blocked", Reason: "request_or_output_limit"}) != nil {
			return nil, ErrStopped
		}
		return nil, g.stop("request_or_output_limit")
	}
	// Charge before dispatch. A failed journal/transport remains conservatively unknown.
	g.s.Attempts++
	g.s.UnknownUsage++
	g.seen[h] = true
	attempt := g.s.Attempts
	g.mu.Unlock()
	if g.event(Event{Kind: "start", Attempt: attempt, RequestSHA256: hex.EncodeToString(h[:]), RequestBytes: len(body)}) != nil {
		return nil, ErrStopped
	}
	ctx, cancel := context.WithCancel(req.Context())
	stop := context.AfterFunc(g.ctx, cancel)
	defer func() { stop(); cancel() }()
	forward := req.Clone(ctx)
	forward.Body = io.NopCloser(bytes.NewReader(body))
	forward.GetBody = nil
	forward.ContentLength = int64(len(body))
	// Remove replay hints even though a fresh HTTP/1 connection is required.
	forward.Header = req.Header.Clone()
	forward.Header.Del("Idempotency-Key")
	forward.Header.Del("X-Idempotency-Key")
	resp, err := t.next.RoundTrip(forward)
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	finish := Event{Kind: "finish", Attempt: attempt, Status: status}
	var data []byte
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		finish.Reason = "transport_failed"
	} else if resp == nil || resp.Body == nil {
		finish.Reason = "response_missing"
	} else {
		data, err = io.ReadAll(io.LimitReader(resp.Body, g.c.MaxBodyBytes+1))
		closeErr := resp.Body.Close()
		switch {
		case err != nil || closeErr != nil:
			finish.Reason = "response_interrupted"
		case int64(len(data)) > g.c.MaxBodyBytes:
			finish.Reason = "response_too_large"
		case status != http.StatusOK:
			finish.Reason = "http_failed"
		default:
			finish.Usage = responseUsage(data, resp.Header.Get("Content-Type"))
			if finish.Usage == nil {
				finish.Reason = "usage_unknown"
			}
		}
	}
	if finish.Usage != nil {
		g.mu.Lock()
		g.s.UnknownUsage--
		g.s.KnownInput += finish.Usage.Input
		g.s.KnownOutput += finish.Usage.Output
		if finish.Usage.Output > *input.MaxOutput {
			finish.Reason = "provider_output_exceeded"
		}
		// Input is a post-response stop, not a tokenizer/context-fit guarantee.
		if g.s.KnownInput >= g.c.StopAfterInputTokens {
			finish.Reason = "input_stop"
		}
		g.mu.Unlock()
	}
	if g.event(finish) != nil {
		return nil, ErrStopped
	}
	if finish.Reason != "" {
		return nil, g.stop(finish.Reason)
	}
	resp.Body = io.NopCloser(bytes.NewReader(data))
	return resp, nil
}

func (g *Guard) block() error {
	g.mu.Lock()
	g.s.Blocked++
	first := g.s.Blocked == 1
	reason := g.s.Stopped
	g.mu.Unlock()
	if first {
		if g.event(Event{Kind: "blocked", Reason: reason}) != nil {
			return ErrStopped
		}
	}
	return g.stop("deadline_or_stopped")
}

// Native Responses JSON or a completed SSE event. Missing/truncated/error
// streams, duplicate terminal events and non-completed status stay unknown.
func responseUsage(data []byte, contentType string) *Usage {
	if strings.HasPrefix(contentType, "text/event-stream") {
		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Buffer(make([]byte, 4096), len(data)+1)
		var terminal []byte
		var payload, eventName string
		done := false
		dispatch := func() bool {
			if payload == "" {
				eventName = ""
				return true
			}
			if payload == "[DONE]" {
				if terminal == nil || done {
					return false
				}
				done = true
				payload, eventName = "", ""
				return true
			}
			if terminal != nil || done {
				return false
			}
			var event struct {
				Type     string          `json:"type"`
				Response json.RawMessage `json:"response"`
			}
			if json.Unmarshal([]byte(payload), &event) != nil || event.Type == "" ||
				(eventName != "" && eventName != event.Type) || event.Type == "error" ||
				event.Type == "response.failed" || event.Type == "response.incomplete" {
				return false
			}
			if event.Type == "response.completed" {
				terminal = event.Response
			}
			payload, eventName = "", ""
			return true
		}
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				if !dispatch() {
					return nil
				}
				continue
			}
			if strings.HasPrefix(line, "event:") {
				eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				continue
			}
			if strings.HasPrefix(line, "data:") {
				if payload != "" {
					payload += "\n"
				}
				payload += strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
			}
		}
		if scanner.Err() != nil || terminal == nil || payload != "" {
			return nil
		}
		data = terminal
	} else if !strings.HasPrefix(contentType, "application/json") {
		return nil
	}
	var result struct {
		Status string `json:"status"`
		Usage  *struct {
			Input  *int64 `json:"input_tokens"`
			Output *int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(data, &result) != nil || result.Status != "completed" || result.Usage == nil ||
		result.Usage.Input == nil || result.Usage.Output == nil || *result.Usage.Input < 0 ||
		*result.Usage.Output < 0 || *result.Usage.Input > 1e9 || *result.Usage.Output > 1e9 {
		return nil
	}
	return &Usage{Input: *result.Usage.Input, Output: *result.Usage.Output}
}
