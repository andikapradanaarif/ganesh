package captureipc

import (
	"bytes"
	"context"
	"errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/capture"
)

var ErrBusy = errors.New("local capture peer already in progress")

type metadataAppender interface {
	Append(context.Context, capture.Metadata) (capture.Receipt, error)
}

// Peer serves admitted connections synchronously against one private spool.
// It has no listener, authentication, process lifecycle or background queue.
// Reuse one Peer to admit at most one exchange at a time; separate instances
// do not share admission. Do not copy a Peer after use.
type Peer struct {
	store metadataAppender
	busy  atomic.Bool
}

func NewPeer(spool capture.Spool) *Peer { return &Peer{store: spool} }

// ServeOne takes exclusive ownership of an already admitted Unix stream
// connection, including closing it on every return, even admission failure.
// Never pass the same connection concurrently or use it again afterwards.
// The caller supplies the deadline and authenticates the client separately.
// One canonical request is appended before a bound receipt is written; no
// error receipt, retry or additional request is sent on this connection.
// A blocked Append keeps this Peer busy until it actually returns. Neither
// cancellation nor an error establishes that no durable publication occurred.
func (p *Peer) ServeOne(ctx context.Context, conn *net.UnixConn) (err error) {
	completed := false
	defer func() {
		_ = recover()
		if !completed {
			err = ErrExchange
		}
	}()
	err = p.serveOne(ctx, conn)
	completed = true
	return
}

func (p *Peer) serveOne(ctx context.Context, conn *net.UnixConn) error {
	if conn == nil {
		return ErrExchange
	}
	defer conn.Close()
	if p == nil || p.store == nil || ctx == nil {
		return ErrExchange
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	if !p.busy.CompareAndSwap(false, true) {
		return ErrBusy
	}
	defer p.busy.Store(false)
	local, remote := conn.LocalAddr(), conn.RemoteAddr()
	if local == nil || remote == nil || local.Network() != "unix" || remote.Network() != "unix" {
		return ErrExchange
	}
	deadline, _ := ctx.Deadline()
	if conn.SetDeadline(deadline) != nil {
		return ErrExchange
	}
	// Interrupt descriptor I/O on cancellation. This cannot interrupt arbitrary
	// storage syscalls and must not release admission while Append is blocked.
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Now()) })
	defer stop()
	if err := contextError(ctx); err != nil {
		return err
	}
	body, err := readFrame(conn)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return ctxErr
	}
	if err != nil {
		return err
	}
	m, err := capture.Decode(body)
	if err != nil {
		return ErrFrame
	}
	canonical, err := encodeMetadata(m)
	if err != nil || !bytes.Equal(body, canonical) {
		return ErrFrame
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	r, err := p.store.Append(ctx, m)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return ctxErr
	}
	if err != nil {
		// Only the local static error escapes; no error frame or wrapped
		// storage diagnostic is disclosed to the connecting client.
		return ErrExchange
	}
	response, err := EncodeReceipt(m, r)
	if err != nil {
		return ErrExchange
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	err = writeFrame(conn, response)
	if ctxErr := contextError(ctx); ctxErr != nil {
		return ctxErr
	}
	return err
}
