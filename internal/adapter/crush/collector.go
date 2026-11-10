package crush

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/capture"
)

var (
	ErrCollector         = errors.New("native capture failed")
	ErrCollectorBusy     = errors.New("native capture already in progress")
	ErrCollectorDeadline = errors.New("native capture requires a caller deadline")
)

// PayloadBuilder synchronously serializes a native event after admission.
// It must not detach work, retain the context or produce side effects.
type PayloadBuilder func(context.Context) (io.Reader, error)

type localAppender interface {
	Append(context.Context, capture.Metadata) (capture.Receipt, error)
}

// Collector is an experimental synchronous boundary, not a supported hook.
// It carries the caller's deadline unchanged and never starts a goroutine.
// Cancellation is cooperative: a blocked builder, reader or filesystem call
// can hold Capture past its deadline. Errors do not establish absence of a
// durable record. Do not copy a Collector after use.
type Collector struct {
	store localAppender
	busy  atomic.Bool
}

// NewCollector creates one admission boundary for a dedicated private spool.
// Separate Collector instances do not share the in-flight admission guard.
func NewCollector(spool capture.Spool) *Collector {
	return &Collector{store: spool}
}

// Capture reduces a native event and acknowledges only its valid local receipt.
// The caller must set the approved deadline before entry/serialization; this
// method neither selects a timing anchor nor resets a clock. Overlapping calls
// on this collector fail immediately without invoking their builders.
func (c *Collector) Capture(ctx context.Context, build PayloadBuilder) (receipt capture.Receipt, err error) {
	defer func() {
		if recover() != nil {
			receipt, err = capture.Receipt{}, ErrCollector
		}
	}()
	if c == nil || c.store == nil || ctx == nil || build == nil {
		return capture.Receipt{}, ErrCollector
	}
	if err := collectorContextError(ctx); err != nil {
		return capture.Receipt{}, err
	}
	if !c.busy.CompareAndSwap(false, true) {
		return capture.Receipt{}, ErrCollectorBusy
	}
	defer c.busy.Store(false)
	// Recheck cancellation after admission before invoking serialization.
	if err := collectorContextError(ctx); err != nil {
		return capture.Receipt{}, err
	}
	input, err := build(ctx)
	if ctxErr := collectorContextError(ctx); ctxErr != nil {
		return capture.Receipt{}, ctxErr
	}
	if err != nil || input == nil {
		return capture.Receipt{}, ErrCollector
	}
	m, err := ReduceStream(ctx, input)
	if ctxErr := collectorContextError(ctx); ctxErr != nil {
		return capture.Receipt{}, ctxErr
	}
	if err != nil {
		return capture.Receipt{}, ErrPayload
	}
	r, err := c.store.Append(ctx, m)
	if ctxErr := collectorContextError(ctx); ctxErr != nil {
		return capture.Receipt{}, ctxErr
	}
	if err != nil {
		// Preserve only known content-free storage errors, never wrappers.
		switch err {
		case capture.ErrStorage, capture.ErrUnsafe, capture.ErrConflict, capture.ErrCapacity, capture.ErrCorrupt:
			return capture.Receipt{}, err
		default:
			return capture.Receipt{}, ErrCollector
		}
	}
	if r.EventID != m.EventID || r.CapturedAt.IsZero() || r.State != "captured_local" || r.Origin != "unknown" || r.Coverage != "incomplete" {
		return capture.Receipt{}, ErrCollector
	}
	if err := collectorContextError(ctx); err != nil {
		return capture.Receipt{}, err
	}
	return r, nil
}

func collectorContextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		switch err {
		case context.Canceled, context.DeadlineExceeded:
			return err
		default:
			return ErrCollector
		}
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return ErrCollectorDeadline
	}
	// An elapsed deadline cannot acknowledge success even if its cancellation
	// notification has not yet been delivered by the runtime.
	if !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return nil
}
