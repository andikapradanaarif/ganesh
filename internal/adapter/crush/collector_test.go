package crush

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/capture"
)

type collectorAppendFunc func(context.Context, capture.Metadata) (capture.Receipt, error)

func (f collectorAppendFunc) Append(ctx context.Context, m capture.Metadata) (capture.Receipt, error) {
	return f(ctx, m)
}

func collectorReceipt(m capture.Metadata) capture.Receipt {
	return capture.Receipt{EventID: m.EventID, CapturedAt: time.Unix(1, 0).UTC(), State: "captured_local", Origin: "unknown", Coverage: "incomplete"}
}

func collectorBuilder(body string) PayloadBuilder {
	return func(context.Context) (io.Reader, error) { return strings.NewReader(body), nil }
}

func collectorContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func TestCollectorPrivateSpoolReplayConflictAndAdmission(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "spool")
	c := NewCollector(capture.Spool{Directory: directory})
	ctx := collectorContext(t)
	first, err := c.Capture(ctx, collectorBuilder(resultBody(false)))
	if err != nil || first.Duplicate || first.EventID != "34de2926-c693-8d74-b48a-54ad575f5835" {
		t.Fatalf("first capture: %+v %v", first, err)
	}
	again, err := c.Capture(ctx, collectorBuilder(resultBody(false)))
	if err != nil || !again.Duplicate || !again.CapturedAt.Equal(first.CapturedAt) {
		t.Fatal("exact replay did not recover the original receipt")
	}
	before, err := os.ReadFile(filepath.Join(directory, first.EventID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		body string
		want error
	}{
		{resultBody(true), capture.ErrConflict},
		{attempt + " trailing", ErrPayload},
		{strings.Repeat(" ", MaxNativeInput) + attempt, ErrPayload},
	} {
		r, err := c.Capture(ctx, collectorBuilder(tc.body))
		if err != tc.want || r != (capture.Receipt{}) {
			t.Fatal("unsafe admission or error receipt")
		}
	}
	after, err := os.ReadFile(filepath.Join(directory, first.EventID+".json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("rejected capture changed the prior record")
	}
	info, err := os.Stat(directory)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatal("spool not private")
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 2 {
		t.Fatal("unexpected spool files")
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatal("file not private")
		}
		body, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"ghp_", "/synthetic", "call-1", "tool_response", "tool_input"} {
			if bytes.Contains(body, []byte(forbidden)) {
				t.Fatal("native content persisted")
			}
		}
	}
	var record capture.Record
	if json.Unmarshal(after, &record) != nil || record.State != "captured_local" || record.Origin != "unknown" || record.Coverage != "incomplete" {
		t.Fatal("local labels changed")
	}
}

func TestCollectorCarriesTheOriginalDeadlineBeforeSerialization(t *testing.T) {
	ctx := collectorContext(t)
	deadline, _ := ctx.Deadline()
	c := &Collector{store: collectorAppendFunc(func(got context.Context, m capture.Metadata) (capture.Receipt, error) {
		d, ok := got.Deadline()
		if got != ctx || !ok || !d.Equal(deadline) {
			t.Fatal("append context or deadline reset")
		}
		return collectorReceipt(m), nil
	})}
	_, err := c.Capture(ctx, func(got context.Context) (io.Reader, error) {
		d, ok := got.Deadline()
		if got != ctx || !ok || !d.Equal(deadline) {
			t.Fatal("serialization context or deadline reset")
		}
		return strings.NewReader(attempt), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{"missing", context.Background(), ErrCollectorDeadline},
		{"expired", expiredCollectorContext(t), context.DeadlineExceeded},
		{"cancelled", cancelledCollectorContext(t), context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := c.Capture(tc.ctx, func(context.Context) (io.Reader, error) { t.Fatal("inadmissible call serialized"); return nil, nil })
			if err != tc.want || r != (capture.Receipt{}) {
				t.Fatal("deadline admission failed")
			}
		})
	}
}

func expiredCollectorContext(t *testing.T) context.Context {
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	t.Cleanup(cancel)
	return ctx
}

func cancelledCollectorContext(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(collectorContext(t))
	cancel()
	return ctx
}

type collectorCancelReader struct {
	input  io.Reader
	cancel context.CancelFunc
}

func (r collectorCancelReader) Read(p []byte) (int, error) { r.cancel(); return r.input.Read(p) }

func TestCollectorCancellationDuringBuildAndReadDoesNotAppend(t *testing.T) {
	for _, phase := range []string{"build", "read"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithCancel(collectorContext(t))
			defer cancel()
			c := &Collector{store: collectorAppendFunc(func(context.Context, capture.Metadata) (capture.Receipt, error) {
				t.Fatal("cancelled metadata appended")
				return capture.Receipt{}, nil
			})}
			r, err := c.Capture(ctx, func(context.Context) (io.Reader, error) {
				input := strings.NewReader(attempt)
				if phase == "build" {
					cancel()
					return input, nil
				}
				return collectorCancelReader{input, cancel}, nil
			})
			if err != context.Canceled || r != (capture.Receipt{}) {
				t.Fatal("cancelled call acknowledged")
			}
		})
	}
}

func TestCollectorBlockedAppendStaysSynchronousAndBusy(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	releaseStore := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseStore)
	var calls atomic.Int32
	c := &Collector{store: collectorAppendFunc(func(ctx context.Context, m capture.Metadata) (capture.Receipt, error) {
		calls.Add(1)
		close(entered)
		<-release
		return collectorReceipt(m), nil
	})}
	ctx, cancel := context.WithCancel(collectorContext(t))
	defer cancel()
	type result struct {
		receipt capture.Receipt
		err     error
	}
	done := make(chan result, 1)
	go func() { r, err := c.Capture(ctx, collectorBuilder(attempt)); done <- result{r, err} }()
	select {
	case <-entered:
	case <-done:
		t.Fatal("capture returned before entering storage")
	case <-time.After(5 * time.Second):
		t.Fatal("storage was never entered")
	}
	cancel()
	var builders atomic.Int32
	var wg sync.WaitGroup
	fresh := collectorContext(t)
	for range 16 {
		wg.Go(func() {
			r, err := c.Capture(fresh, func(context.Context) (io.Reader, error) { builders.Add(1); return strings.NewReader(attempt), nil })
			if err != ErrCollectorBusy || r != (capture.Receipt{}) {
				t.Error("overlapping call admitted")
			}
		})
	}
	wg.Wait()
	if builders.Load() != 0 || calls.Load() != 1 {
		t.Fatal("overlap started work")
	}
	select {
	case <-done:
		t.Fatal("blocked storage was detached after cancellation")
	default:
	}
	releaseStore()
	select {
	case got := <-done:
		if got.err != context.Canceled || got.receipt != (capture.Receipt{}) {
			t.Fatal("late completion acknowledged")
		}
	case <-time.After(time.Second):
		t.Fatal("released capture did not finish")
	}
	c.store = collectorAppendFunc(func(_ context.Context, m capture.Metadata) (capture.Receipt, error) { return collectorReceipt(m), nil })
	if _, err := c.Capture(collectorContext(t), collectorBuilder(attempt)); err != nil {
		t.Fatal("admission gate not released")
	}
}

type collectorPanicReader struct{}

func (collectorPanicReader) Read([]byte) (int, error) { panic("ghp_SYNTHETIC_SECRET") }

func TestCollectorPanicsAndWrappedErrorsAreContentFree(t *testing.T) {
	for _, phase := range []string{"build panic", "reader panic", "append panic", "build error", "append wrapper"} {
		t.Run(phase, func(t *testing.T) {
			c := &Collector{store: collectorAppendFunc(func(_ context.Context, m capture.Metadata) (capture.Receipt, error) {
				if phase == "append panic" {
					panic("ghp_SYNTHETIC_SECRET")
				}
				if phase == "append wrapper" {
					return collectorReceipt(m), fmt.Errorf("ghp_SYNTHETIC_SECRET: %w", capture.ErrUnsafe)
				}
				return collectorReceipt(m), nil
			})}
			build := func(context.Context) (io.Reader, error) {
				switch phase {
				case "build panic":
					panic("ghp_SYNTHETIC_SECRET")
				case "reader panic":
					return collectorPanicReader{}, nil
				case "build error":
					return nil, errors.New("ghp_SYNTHETIC_SECRET")
				}
				return strings.NewReader(attempt), nil
			}
			r, err := c.Capture(collectorContext(t), build)
			if err != ErrCollector || r != (capture.Receipt{}) || strings.Contains(err.Error(), "ghp_") {
				t.Fatal("unsafe failure result")
			}
			c.store = collectorAppendFunc(func(_ context.Context, m capture.Metadata) (capture.Receipt, error) { return collectorReceipt(m), nil })
			if _, err := c.Capture(collectorContext(t), collectorBuilder(attempt)); err != nil {
				t.Fatal("failure stranded admission gate")
			}
		})
	}
}

func TestCollectorRejectsUnboundOrPromotedReceipts(t *testing.T) {
	for _, mutate := range []func(*capture.Receipt){
		func(r *capture.Receipt) { r.EventID = "00000000-0000-4000-8000-000000000099" },
		func(r *capture.Receipt) { r.CapturedAt = time.Time{} },
		func(r *capture.Receipt) { r.State = "accepted_by_service" },
		func(r *capture.Receipt) { r.Origin = "verified" },
		func(r *capture.Receipt) { r.Coverage = "complete" },
	} {
		c := &Collector{store: collectorAppendFunc(func(_ context.Context, m capture.Metadata) (capture.Receipt, error) {
			r := collectorReceipt(m)
			mutate(&r)
			return r, nil
		})}
		if r, err := c.Capture(collectorContext(t), collectorBuilder(attempt)); err != ErrCollector || r != (capture.Receipt{}) {
			t.Fatal("invalid receipt acknowledged")
		}
	}
}

func TestCollectorSuppressesReceiptWhenAppendOutlastsDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(collectorContext(t), 100*time.Millisecond)
	defer cancel()
	appended := false
	c := &Collector{store: collectorAppendFunc(func(got context.Context, m capture.Metadata) (capture.Receipt, error) {
		appended = true
		<-got.Done()
		// A non-cooperative store can report success after the caller's limit.
		return collectorReceipt(m), nil
	})}
	r, err := c.Capture(ctx, collectorBuilder(attempt))
	if !appended || err != context.DeadlineExceeded || r != (capture.Receipt{}) {
		t.Fatal("late successful append was acknowledged")
	}
}

func TestCollectorPreservesPrivateStoreAndCapacityErrors(t *testing.T) {
	for _, kind := range []string{"unsafe", "capacity"} {
		t.Run(kind, func(t *testing.T) {
			spool := capture.Spool{Directory: filepath.Join(t.TempDir(), "spool")}
			want := capture.ErrCapacity
			if kind == "unsafe" {
				if os.Mkdir(spool.Directory, 0o700) != nil || os.Chmod(spool.Directory, 0o755) != nil {
					t.Fatal("unsafe fixture setup failed")
				}
				want = capture.ErrUnsafe
			} else {
				spool.MaxBytes = 1
			}
			r, err := NewCollector(spool).Capture(collectorContext(t), collectorBuilder(attempt))
			if err != want || r != (capture.Receipt{}) {
				t.Fatal("store rejection changed or acknowledged")
			}
		})
	}
}

func TestCollectorRejectsUnconfiguredCallsAndNilReaders(t *testing.T) {
	valid := &Collector{store: collectorAppendFunc(func(_ context.Context, m capture.Metadata) (capture.Receipt, error) {
		return collectorReceipt(m), nil
	})}
	for _, c := range []*Collector{nil, {}} {
		if r, err := c.Capture(collectorContext(t), collectorBuilder(attempt)); err != ErrCollector || r != (capture.Receipt{}) {
			t.Fatal("unconfigured collector admitted work")
		}
	}
	for _, build := range []PayloadBuilder{nil, func(context.Context) (io.Reader, error) { return nil, nil }} {
		if r, err := valid.Capture(collectorContext(t), build); err != ErrCollector || r != (capture.Receipt{}) {
			t.Fatal("missing builder/reader admitted work")
		}
	}
	if r, err := valid.Capture(nil, collectorBuilder(attempt)); err != ErrCollector || r != (capture.Receipt{}) {
		t.Fatal("missing context admitted work")
	}
}
