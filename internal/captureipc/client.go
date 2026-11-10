package captureipc

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/capture"
)

var (
	ErrDeadline = errors.New("local capture exchange requires a caller deadline")
	ErrUsed     = errors.New("local capture connection already consumed")
)

// Client owns one caller-supplied connected Unix stream socket. The caller
// must admit/authenticate its peer separately; the request digest is no proof
// of identity or durability. There is no dial, listener or worker lifecycle.
// Do not copy a Client after use.
type Client struct {
	conn *net.UnixConn
	used atomic.Bool
}

// NewClient takes exclusive ownership of conn, including its deadlines.
func NewClient(conn *net.UnixConn) *Client { return &Client{conn: conn} }

// Close releases an unused or active connection. An already closed connection
// succeeds, so deferred cleanup after Submit is safe and quiet.
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return ErrExchange
	}
	if err := c.conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return ErrExchange
	}
	return nil
}

// Submit sends only normalized metadata, once, under the original caller
// deadline. The first call consumes/closes the connection, even on admission
// failure. Subsequent calls fail without disturbing an active exchange.
// A timeout after sending is ambiguous about peer storage effects.
func (c *Client) Submit(ctx context.Context, m capture.Metadata) (receipt capture.Receipt, err error) {
	completed := false
	defer func() {
		_ = recover()
		if !completed {
			receipt, err = capture.Receipt{}, ErrExchange
		}
	}()
	receipt, err = c.submit(ctx, m)
	completed = true
	return
}

func (c *Client) submit(ctx context.Context, m capture.Metadata) (capture.Receipt, error) {
	if c == nil || c.conn == nil {
		return capture.Receipt{}, ErrExchange
	}
	if !c.used.CompareAndSwap(false, true) {
		return capture.Receipt{}, ErrUsed
	}
	defer c.Close()
	if ctx == nil {
		return capture.Receipt{}, ErrExchange
	}
	if err := contextError(ctx); err != nil {
		return capture.Receipt{}, err
	}
	local, remote := c.conn.LocalAddr(), c.conn.RemoteAddr()
	if local == nil || remote == nil || local.Network() != "unix" || remote.Network() != "unix" {
		return capture.Receipt{}, ErrExchange
	}
	body, err := encodeMetadata(m)
	if err != nil {
		return capture.Receipt{}, err
	}
	deadline, _ := ctx.Deadline()
	if c.conn.SetDeadline(deadline) != nil {
		return capture.Receipt{}, ErrExchange
	}
	// Cancellation may precede a far-future socket deadline. This callback
	// interrupts socket I/O only; it performs no capture or storage work.
	stop := context.AfterFunc(ctx, func() { _ = c.conn.SetDeadline(time.Now()) })
	defer stop()
	if err := contextError(ctx); err != nil {
		return capture.Receipt{}, err
	}
	err = writeFrame(c.conn, body)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return capture.Receipt{}, ctxErr
	}
	if err != nil {
		return capture.Receipt{}, err
	}
	response, err := readFrame(c.conn)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return capture.Receipt{}, ctxErr
	}
	if err != nil {
		return capture.Receipt{}, err
	}
	r, err := decodeReceipt(response, m, requestDigest(body))
	if ctxErr := contextError(ctx); ctxErr != nil {
		return capture.Receipt{}, ctxErr
	}
	if err != nil {
		return capture.Receipt{}, err
	}
	return r, nil
}

func contextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		switch err {
		case context.Canceled, context.DeadlineExceeded:
			return err
		default:
			return ErrExchange
		}
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return ErrDeadline
	}
	if !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return nil
}
