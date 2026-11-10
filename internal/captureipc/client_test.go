package captureipc

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/capture"
)

func unixPair(t *testing.T) (*net.UnixConn, *net.UnixConn) {
	t.Helper()
	// Short private path also fits Darwin's Unix socket name bound.
	directory, err := os.MkdirTemp("/tmp", "ganesh-ipc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	address := &net.UnixAddr{Net: "unix", Name: filepath.Join(directory, "peer.sock")}
	listener, err := net.ListenUnix("unix", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	if os.Chmod(address.Name, 0o600) != nil {
		t.Fatal("socket mode")
	}
	client, err := net.DialUnix("unix", nil, address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	peer, err := listener.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = peer.Close() })
	if listener.Close() != nil {
		t.Fatal("listener close")
	}
	if peer.SetDeadline(time.Now().Add(5*time.Second)) != nil {
		t.Fatal("peer deadline")
	}
	return client, peer
}

func exchangeContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func TestClientRoundTripAndSingleUse(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		client, peer := unixPair(t)
		peerDone := make(chan error, 1)
		go func() {
			body, err := readFrame(peer)
			if err != nil {
				peerDone <- err
				return
			}
			m, err := capture.Decode(body)
			if err != nil || m != testMetadata() {
				peerDone <- ErrFrame
				return
			}
			r := testReceipt()
			r.Duplicate = duplicate
			response, err := EncodeReceipt(m, r)
			if err == nil {
				err = writeFrame(peer, response)
			}
			peerDone <- err
		}()
		c := NewClient(client)
		r, err := c.Submit(exchangeContext(t), testMetadata())
		want := testReceipt()
		want.Duplicate = duplicate
		if err != nil || r != want {
			t.Fatalf("round trip: %v", err)
		}
		if err := <-peerDone; err != nil {
			t.Fatal(err)
		}
		if err := c.Close(); err != nil {
			t.Fatal("cleanup after successful Submit was not idempotent")
		}
		if r, err := c.Submit(exchangeContext(t), testMetadata()); err != ErrUsed || r != (capture.Receipt{}) {
			t.Fatal("connection reused")
		}
	}
}

func TestClientCloseIsIdempotentBeforeSubmit(t *testing.T) {
	client, peer := unixPair(t)
	c := NewClient(client)
	if c.Close() != nil || c.Close() != nil {
		t.Fatal("repeated close failed")
	}
	if _, err := readFrame(peer); err != ErrExchange {
		t.Fatal("unused connection remained open")
	}
	if r, err := c.Submit(exchangeContext(t), testMetadata()); err != ErrExchange || r != (capture.Receipt{}) {
		t.Fatal("closed connection admitted an exchange")
	}
	if c.Close() != nil {
		t.Fatal("cleanup after failed Submit was not idempotent")
	}
	if _, err := c.Submit(exchangeContext(t), testMetadata()); err != ErrUsed {
		t.Fatal("failed exchange became reusable")
	}
	for _, invalid := range []*Client{nil, {}, NewClient(nil), NewClient(&net.UnixConn{})} {
		if invalid.Close() != ErrExchange {
			t.Fatal("unconfigured close lost its static error")
		}
	}
}

func TestClientCancellationStopsWaitWhilePeerRemainsBlocked(t *testing.T) {
	client, peer := unixPair(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	releasePeer := func() { releaseOnce.Do(func() { close(release) }) }
	defer releasePeer()
	peerDone := make(chan error, 1)
	go func() {
		body, err := readFrame(peer)
		if err != nil {
			peerDone <- err
			return
		}
		m, err := capture.Decode(body)
		if err != nil {
			peerDone <- err
			return
		}
		close(entered)
		<-release // A fake storage stage does not honor the client's context.
		response, err := EncodeReceipt(m, testReceipt())
		if err == nil {
			err = writeFrame(peer, response)
		}
		peerDone <- err
	}()
	c := NewClient(client)
	ctx, cancel := context.WithCancel(exchangeContext(t))
	defer cancel()
	type result struct {
		receipt capture.Receipt
		err     error
	}
	done := make(chan result, 1)
	go func() { r, err := c.Submit(ctx, testMetadata()); done <- result{r, err} }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("peer never entered blocked stage")
	}
	// An overlapping call cannot close or interfere with the active exchange.
	if r, err := c.Submit(exchangeContext(t), testMetadata()); err != ErrUsed || r != (capture.Receipt{}) {
		t.Fatal("overlap admitted")
	}
	cancel() // Original socket deadline is still far in the future.
	select {
	case got := <-done:
		if got.err != context.Canceled || got.receipt != (capture.Receipt{}) {
			t.Fatal("cancelled wait acknowledged")
		}
	case <-time.After(time.Second):
		t.Fatal("socket wait did not observe cancellation")
	}
	select {
	case <-peerDone:
		t.Fatal("client cancellation completed peer storage")
	default:
	}
	releasePeer()
	select {
	case <-peerDone:
		// A late peer write may succeed into kernel buffers or fail after
		// closure; neither outcome can change the client's returned result.
	case <-time.After(time.Second):
		t.Fatal("released peer did not finish")
	}
}

func TestClientMissingExpiredAndCancelledContextsConsumeSocket(t *testing.T) {
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer cancelExpired()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		ctx  context.Context
		want error
	}{{context.Background(), ErrDeadline}, {expired, context.DeadlineExceeded}, {cancelled, context.Canceled}, {nil, ErrExchange}} {
		client, peer := unixPair(t)
		c := NewClient(client)
		if r, err := c.Submit(tc.ctx, testMetadata()); err != tc.want || r != (capture.Receipt{}) {
			t.Fatal("context admission changed")
		}
		if _, err := readFrame(peer); err != ErrExchange {
			t.Fatal("inadmissible call sent a frame")
		}
		if _, err := c.Submit(exchangeContext(t), testMetadata()); err != ErrUsed {
			t.Fatal("inadmissible connection remained reusable")
		}
	}
}

func TestClientInvalidMetadataAndUnconfiguredConnections(t *testing.T) {
	for _, c := range []*Client{nil, {}, NewClient(nil), NewClient(&net.UnixConn{})} {
		if r, err := c.Submit(exchangeContext(t), testMetadata()); err != ErrExchange || r != (capture.Receipt{}) {
			t.Fatal("unconfigured socket admitted")
		}
	}
	client, peer := unixPair(t)
	c := NewClient(client)
	m := testMetadata()
	m.SessionID = "ghp_SYNTHETIC_SECRET"
	if r, err := c.Submit(exchangeContext(t), m); err != ErrFrame || r != (capture.Receipt{}) {
		t.Fatal("raw metadata admitted")
	}
	if _, err := readFrame(peer); err != ErrExchange {
		t.Fatal("invalid metadata dispatched")
	}
}

func TestClientRejectsInvalidAndPartialResponses(t *testing.T) {
	for _, kind := range []string{"disconnect", "partial header", "partial body", "too large", "wrong request", "promoted", "invalid utf8"} {
		t.Run(kind, func(t *testing.T) {
			client, peer := unixPair(t)
			peerDone := make(chan error, 1)
			go func() {
				defer peer.Close()
				if _, err := readFrame(peer); err != nil {
					peerDone <- err
					return
				}
				var err error
				switch kind {
				case "disconnect":
				case "partial header":
					_, err = peer.Write([]byte{0, 0})
				case "partial body":
					_, err = peer.Write([]byte{0, 0, 0, 20, '{'})
				case "too large":
					var header [4]byte
					binary.BigEndian.PutUint32(header[:], MaxFrame+1)
					_, err = peer.Write(header[:])
				case "wrong request":
					m := testMetadata()
					m.Outcome = "failed"
					var b []byte
					b, err = EncodeReceipt(m, testReceipt())
					if err == nil {
						err = writeFrame(peer, b)
					}
				case "promoted":
					b, _ := EncodeReceipt(testMetadata(), testReceipt())
					err = writeFrame(peer, []byte(strings.Replace(string(b), "unknown", "verified", 1)))
				case "invalid utf8":
					err = writeFrame(peer, []byte{'{', '"', 0xff, '"', ':', '1', '}'})
				}
				peerDone <- err
			}()
			want := ErrFrame
			if kind == "disconnect" || kind == "partial header" || kind == "partial body" {
				want = ErrExchange
			}
			r, err := NewClient(client).Submit(exchangeContext(t), testMetadata())
			if err != want || r != (capture.Receipt{}) || strings.Contains(err.Error(), "ghp_") || strings.Contains(err.Error(), "ganesh-ipc-") {
				t.Fatal("unsafe peer response")
			}
			if err := <-peerDone; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestClientDeadlineOnPartialResponse(t *testing.T) {
	client, peer := unixPair(t)
	peerDone := make(chan error, 1)
	go func() {
		if _, err := readFrame(peer); err != nil {
			peerDone <- err
			return
		}
		_, err := peer.Write([]byte{0, 0, 0, 20, '{'})
		peerDone <- err
	}()
	ctx, cancel := context.WithTimeout(exchangeContext(t), 100*time.Millisecond)
	defer cancel()
	r, err := NewClient(client).Submit(ctx, testMetadata())
	if err != context.DeadlineExceeded || r != (capture.Receipt{}) {
		t.Fatal("partial response deadline acknowledged")
	}
	if err := <-peerDone; err != nil {
		t.Fatal(err)
	}
}

type panicContext struct {
	context.Context
	nilValue bool
}

func (p panicContext) Err() error {
	if p.nilValue {
		panic(nil)
	}
	panic("ghp_SYNTHETIC_SECRET")
}

func TestClientContextPanicsFailClosed(t *testing.T) {
	for _, nilValue := range []bool{false, true} {
		client, peer := unixPair(t)
		r, err := NewClient(client).Submit(panicContext{exchangeContext(t), nilValue}, testMetadata())
		if err != ErrExchange || r != (capture.Receipt{}) {
			t.Fatal("panic acknowledged or leaked")
		}
		if _, err := readFrame(peer); err != ErrExchange {
			t.Fatal("panic retained socket")
		}
	}
}

type exoticContext struct{ context.Context }

func (exoticContext) Err() error { return errors.New("ghp_SYNTHETIC_SECRET") }

func TestClientMasksExoticContextErrors(t *testing.T) {
	client, _ := unixPair(t)
	if r, err := NewClient(client).Submit(exoticContext{exchangeContext(t)}, testMetadata()); err != ErrExchange || r != (capture.Receipt{}) {
		t.Fatal("exotic error escaped")
	}
}

type elapsedContext struct{ context.Context }

func (elapsedContext) Deadline() (time.Time, bool) { return time.Unix(1, 0), true }

func TestClientRejectsElapsedDeadlineBeforeCancellationNotification(t *testing.T) {
	client, peer := unixPair(t)
	// Simulates the interval before an elapsed deadline's Err is delivered.
	ctx := elapsedContext{context.Background()}
	if r, err := NewClient(client).Submit(ctx, testMetadata()); err != context.DeadlineExceeded || r != (capture.Receipt{}) {
		t.Fatal("elapsed deadline admitted")
	}
	if _, err := readFrame(peer); err != ErrExchange {
		t.Fatal("elapsed deadline sent metadata")
	}
}
