package requestguard

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrRunStopped = errors.New("shared experiment run stopped")
var ErrRunBusy = errors.New("shared experiment request already in flight")

// RunLimits freeze accounting inputs, not execution/disclosure authority.
// InputStop is checked after actual usage returns; it is not a tokenizer bound.
type RunLimits struct {
	ID             string            `json:"id"`
	MaxRequests    int64             `json:"max_requests"`
	MaxOutputTotal int64             `json:"max_output_total"`
	InputStop      int64             `json:"input_stop"`
	Deadline       time.Time         `json:"deadline"`
	Profiles       map[string]Config `json:"profiles"`
	Slots          map[string]string `json:"slots"`
}

type RunOptions struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Slot   string `json:"slot"`
}

type RunSummary struct {
	Requests     int64  `json:"requests"`
	KnownInput   int64  `json:"known_input_tokens"`
	KnownOutput  int64  `json:"known_output_tokens"`
	UnknownUsage int64  `json:"unknown_usage_requests"`
	SlotsUsed    int    `json:"slots_used"`
	Pending      bool   `json:"pending"`
	Stopped      string `json:"stopped,omitempty"`
}

type runRecord struct {
	Kind   string     `json:"kind"`
	Limits *RunLimits `json:"limits,omitempty"`
	Slot   string     `json:"slot,omitempty"`
	Event  *Event     `json:"event,omitempty"`
}

type runState struct {
	limits  RunLimits
	s       RunSummary
	last    map[string]int64
	output  map[string]int64
	input   map[string]int64
	pending *runRecord
}

type runFile struct {
	root *os.Root
	f    *os.File
	name string
}

func validName(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return false
		}
	}
	return true
}

type discardJournal struct{}

func (discardJournal) Append(Event) error { return nil }

func validLimits(l RunLimits) bool {
	if !validName(l.ID) || l.MaxRequests <= 0 || l.MaxRequests > 10000 || l.MaxOutputTotal <= 0 || l.MaxOutputTotal > 1e9 || l.InputStop <= 0 || l.InputStop > 1e9 || l.Deadline.IsZero() || len(l.Profiles) == 0 || len(l.Profiles) > 8 || len(l.Slots) == 0 || len(l.Slots) > 1000 {
		return false
	}
	for name, c := range l.Profiles {
		if !validName(name) {
			return false
		}
		g, err := New(c, discardJournal{})
		if err != nil {
			return false
		}
		g.Close()
	}
	for slot, profile := range l.Slots {
		if !validName(slot) {
			return false
		}
		if _, ok := l.Profiles[profile]; !ok {
			return false
		}
	}
	return true
}

func encodeRecord(r runRecord) ([]byte, error) { b, e := json.Marshal(r); return append(b, '\n'), e }
func hashLine(b []byte) string                 { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func openRunFile(path string, create bool) (*runFile, error) {
	info, err := os.Stat(filepath.Dir(path))
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, ErrStopped
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, ErrStopped
	}
	flags := os.O_RDWR | runNoFollow
	if create {
		flags |= os.O_CREATE | os.O_EXCL
	}
	f, err := root.OpenFile(filepath.Base(path), flags, 0600)
	if err != nil {
		root.Close()
		return nil, ErrStopped
	}
	r := &runFile{root: root, f: f, name: filepath.Base(path)}
	if !r.valid() {
		r.close()
		return nil, ErrStopped
	}
	return r, nil
}

func (r *runFile) valid() bool {
	f, err := r.f.Stat()
	if err != nil || !f.Mode().IsRegular() || f.Mode().Perm()&0077 != 0 || !runSingleLink(f) {
		return false
	}
	name, err := r.root.Lstat(r.name)
	return err == nil && name.Mode().IsRegular() && os.SameFile(f, name)
}
func (r *runFile) close() { _ = r.f.Close(); _ = r.root.Close() }

func (r *runFile) append(e runRecord) error {
	b, err := encodeRecord(e)
	if err != nil {
		return ErrStopped
	}
	if _, err = r.f.Seek(0, io.SeekEnd); err != nil {
		return ErrStopped
	}
	n, err := r.f.Write(b)
	if err != nil || n != len(b) || r.f.Sync() != nil {
		return ErrStopped
	}
	return nil
}

// CreateRun is exclusive. Its returned digest pins the exact limits/header.
// The caller must freeze that digest/path before any client starts.
func CreateRun(path string, l RunLimits) (string, error) {
	if !validLimits(l) {
		return "", ErrStopped
	}
	r, err := openRunFile(path, true)
	if err != nil {
		return "", err
	}
	defer r.close()
	if runLock(r.f) != nil {
		return "", ErrStopped
	}
	defer runUnlock(r.f)
	header := runRecord{Kind: "run-v1", Limits: &l}
	if r.append(header) != nil {
		return "", ErrStopped
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return "", ErrStopped
	}
	defer dir.Close()
	if dir.Sync() != nil {
		return "", ErrStopped
	}
	b, err := encodeRecord(header)
	if err != nil {
		return "", ErrStopped
	}
	return hashLine(b), nil
}

func (r *runFile) replay(expected string) (runState, error) {
	state := runState{last: map[string]int64{}, output: map[string]int64{}, input: map[string]int64{}}
	if !r.valid() || len(expected) != 64 {
		return state, ErrStopped
	}
	if _, err := r.f.Seek(0, io.SeekStart); err != nil {
		return state, ErrStopped
	}
	b, err := io.ReadAll(io.LimitReader(r.f, 32<<20+1))
	if err != nil || len(b) == 0 || len(b) > 32<<20 || b[len(b)-1] != '\n' {
		return state, ErrStopped
	}
	lines := bytes.Split(b[:len(b)-1], []byte{'\n'})
	if hashLine(append(append([]byte(nil), lines[0]...), '\n')) != expected {
		return state, ErrStopped
	}
	for i, line := range lines {
		var rec runRecord
		d := json.NewDecoder(bytes.NewReader(line))
		d.DisallowUnknownFields()
		if d.Decode(&rec) != nil {
			return state, ErrStopped
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			return state, ErrStopped
		}
		if i == 0 {
			if rec.Kind != "run-v1" || rec.Limits == nil || rec.Event != nil || rec.Slot != "" || !validLimits(*rec.Limits) {
				return state, ErrStopped
			}
			state.limits = *rec.Limits
			continue
		}
		if rec.Limits != nil || state.apply(rec) != nil {
			return state, ErrStopped
		}
	}
	state.s.Pending = state.pending != nil
	state.s.SlotsUsed = len(state.last)
	if state.s.Pending && state.s.Stopped == "" {
		state.s.Stopped = "unfinished_request"
	}
	if state.s.Stopped == "" && !time.Now().Before(state.limits.Deadline) {
		state.s.Stopped = "deadline"
	}
	return state, nil
}

func (s *runState) apply(r runRecord) error {
	profile, ok := s.limits.Slots[r.Slot]
	if !ok || r.Event == nil {
		return ErrStopped
	}
	c := s.limits.Profiles[profile]
	e := r.Event
	if e.At.IsZero() || e.Attempt <= 0 {
		return ErrStopped
	}
	switch r.Kind {
	case "start":
		if s.pending != nil || s.s.Stopped != "" || e.Kind != "start" || e.Attempt != s.last[r.Slot]+1 || e.Attempt > c.MaxRequests ||
			e.At.After(s.limits.Deadline) || e.Status != 0 || e.Reason != "" || e.Usage != nil || e.RequestBytes <= 0 || int64(e.RequestBytes) > c.MaxBodyBytes ||
			e.RequestedOutput <= 0 || e.RequestedOutput > c.MaxOutputPerRequest || len(e.RequestSHA256) != 64 ||
			s.s.Requests >= s.limits.MaxRequests || e.RequestedOutput > s.limits.MaxOutputTotal-s.s.KnownOutput || e.RequestedOutput > c.MaxOutputTotal-s.output[r.Slot] {
			return ErrStopped
		}
		if _, err := hex.DecodeString(e.RequestSHA256); err != nil {
			return ErrStopped
		}
		s.s.Requests++
		s.s.UnknownUsage++
		s.last[r.Slot] = e.Attempt
		s.pending = &r
	case "finish":
		if s.pending == nil || r.Slot != s.pending.Slot || e.Kind != "finish" || e.Attempt != s.pending.Event.Attempt || e.At.Before(s.pending.Event.At) || e.RequestSHA256 != "" || e.RequestBytes != 0 || e.RequestedOutput != 0 {
			return ErrStopped
		}
		if e.Usage != nil {
			if e.Status != 200 || e.Usage.Input < 0 || e.Usage.Input > 1e9 || e.Usage.Output < 0 || e.Usage.Output > 1e9 {
				return ErrStopped
			}
			s.s.UnknownUsage--
			s.s.KnownInput += e.Usage.Input
			s.s.KnownOutput += e.Usage.Output
			s.output[r.Slot] += e.Usage.Output
			s.input[r.Slot] += e.Usage.Input
			if e.Usage.Output > s.pending.Event.RequestedOutput {
				s.s.Stopped = "provider_output_exceeded"
			}
			if s.input[r.Slot] >= c.StopAfterInputTokens {
				s.s.Stopped = "input_stop"
			}
		} else {
			s.s.Stopped = "usage_unknown"
		}
		if e.Reason != "" {
			s.s.Stopped = e.Reason
		}
		if s.s.KnownInput >= s.limits.InputStop {
			s.s.Stopped = "run_input_stop"
		}
		s.pending = nil
	case "stop":
		if s.pending != nil || e.Kind != "blocked" || e.Reason != "run_request_or_output_limit" || e.Usage != nil || e.Status != 0 || e.RequestSHA256 != "" || e.RequestBytes != 0 || e.RequestedOutput != 0 {
			return ErrStopped
		}
		s.s.Stopped = e.Reason
	default:
		return ErrStopped
	}
	return nil
}

// InspectRun never repairs records, refunds reservations or guesses usage.
// An unfinished start is visible as unknown and blocks future dispatch.
func InspectRun(path, expected string) (RunSummary, error) {
	r, err := openRunFile(path, false)
	if err != nil {
		return RunSummary{}, err
	}
	defer r.close()
	if runLock(r.f) != nil {
		return RunSummary{}, ErrStopped
	}
	defer runUnlock(r.f)
	s, err := r.replay(expected)
	return s.s, err
}

// RunJournal holds a nonblocking OS file lock from synced start through finish.
// A crash releases the lock but leaves the start pending. Only a later request
// from the same living journal can continue a slot; a new process cannot reuse it.
type RunJournal struct {
	mu      sync.Mutex
	r       *runFile
	local   Journal
	opts    RunOptions
	attempt int64
	locked  bool
	closed  bool
}

func NewShared(c Config, local Journal, o RunOptions) (*Guard, error) {
	if local == nil || !validName(o.Slot) {
		return nil, ErrStopped
	}
	r, err := openRunFile(o.Path, false)
	if err != nil {
		return nil, err
	}
	if err := runLock(r.f); err != nil {
		r.close()
		return nil, err
	}
	s, err := r.replay(o.SHA256)
	profile, ok := s.limits.Slots[o.Slot]
	want, _ := json.Marshal(s.limits.Profiles[profile])
	got, _ := json.Marshal(c)
	if err != nil || !ok || !bytes.Equal(want, got) || s.s.Stopped != "" || s.last[o.Slot] != 0 || !time.Now().Before(s.limits.Deadline) {
		runUnlock(r.f)
		r.close()
		return nil, ErrStopped
	}
	runUnlock(r.f)
	j := &RunJournal{r: r, local: local, opts: o}
	g, err := New(c, j)
	if err != nil {
		r.close()
		return nil, err
	}
	parentCancel := g.cancel
	ctx, cancel := context.WithDeadline(g.ctx, s.limits.Deadline)
	g.ctx = ctx
	g.cancel = func() { cancel(); parentCancel() }
	g.shared = j
	return g, nil
}

func (j *RunJournal) Append(e Event) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return ErrStopped
	}
	if e.Kind == "blocked" {
		return j.local.Append(e)
	}
	if e.Kind == "start" {
		if j.locked {
			return ErrStopped
		}
		if err := runLock(j.r.f); err != nil {
			return err
		}
		j.locked = true
		s, err := j.r.replay(j.opts.SHA256)
		if err != nil || s.s.Stopped != "" || s.last[j.opts.Slot] != j.attempt || e.Attempt != j.attempt+1 || !time.Now().Before(s.limits.Deadline) {
			return j.fail()
		}
		rec := runRecord{Kind: "start", Slot: j.opts.Slot, Event: &e}
		if s.s.Requests >= s.limits.MaxRequests || e.RequestedOutput > s.limits.MaxOutputTotal-s.s.KnownOutput {
			blocked := Event{Kind: "blocked", Attempt: e.Attempt, At: e.At, Reason: "run_request_or_output_limit"}
			if j.r.append(runRecord{Kind: "stop", Slot: j.opts.Slot, Event: &blocked}) != nil || j.local.Append(blocked) != nil {
				return j.fail()
			}
			_ = j.fail()
			return ErrRunStopped
		}
		if s.apply(rec) != nil || j.r.append(rec) != nil || j.local.Append(e) != nil {
			return j.fail()
		}
		j.attempt = e.Attempt
		return nil
	}
	if e.Kind != "finish" || !j.locked {
		return ErrStopped
	}
	s, err := j.r.replay(j.opts.SHA256)
	if err != nil {
		return j.fail()
	}
	// replay marks a trailing start unfinished; the lock holder may finish it.
	if s.s.Stopped == "unfinished_request" {
		s.s.Stopped = ""
	}
	rec := runRecord{Kind: "finish", Slot: j.opts.Slot, Event: &e}
	if s.apply(rec) != nil || j.local.Append(e) != nil || j.r.append(rec) != nil {
		return j.fail()
	}
	runUnlock(j.r.f)
	j.locked = false
	if s.s.Stopped != "" {
		return ErrRunStopped
	}
	return nil
}

func (j *RunJournal) fail() error {
	if j.locked {
		runUnlock(j.r.f)
		j.locked = false
	}
	// No continuation through this instance after an accounting failure.
	j.closed = true
	j.r.close()
	return ErrStopped
}
func (j *RunJournal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.closed {
		if j.locked {
			runUnlock(j.r.f)
		}
		j.r.close()
		j.closed = true
	}
	return nil
}
