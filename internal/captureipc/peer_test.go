package captureipc

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/capture"
)

type appendMetadataFunc func(context.Context, capture.Metadata) (capture.Receipt, error)

func (f appendMetadataFunc) Append(ctx context.Context, m capture.Metadata) (capture.Receipt, error) {
	return f(ctx, m)
}

func awaitPeer(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("peer did not finish")
		return nil
	}
}

func peerExchange(t *testing.T, p *Peer, m capture.Metadata) (capture.Receipt, error, error) {
	t.Helper()
	client, conn := unixPair(t)
	done := make(chan error, 1)
	go func() { done <- p.ServeOne(exchangeContext(t), conn) }()
	r, err := NewClient(client).Submit(exchangeContext(t), m)
	return r, err, awaitPeer(t, done)
}

func TestPeerRealSpoolReplayAndConflict(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "spool")
	p := NewPeer(capture.Spool{Directory: directory})
	m := testMetadata()
	r, clientErr, peerErr := peerExchange(t, p, m)
	if clientErr != nil || peerErr != nil || !validReceipt(m, r) || r.Duplicate {
		t.Fatal("durable round trip failed")
	}
	path := filepath.Join(directory, m.EventID+".json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var stored capture.Record
	if err := json.Unmarshal(before, &stored); err != nil || stored.Metadata != m || stored.CapturedAt != r.CapturedAt || stored.State != "captured_local" || stored.Origin != "unknown" || stored.Coverage != "incomplete" {
		t.Fatal("receipt was not bound to stored record")
	}
	for name, mode := range map[string]os.FileMode{directory: 0700, path: 0600, filepath.Join(directory, ".lock"): 0600} {
		info, err := os.Stat(name)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatal("private spool mode")
		}
	}
	replayed, clientErr, peerErr := peerExchange(t, p, m)
	want := r
	want.Duplicate = true
	if clientErr != nil || peerErr != nil || replayed != want {
		t.Fatal("exact replay changed receipt identity/time")
	}
	conflict := m
	conflict.Outcome = "failed"
	got, clientErr, peerErr := peerExchange(t, p, conflict)
	if clientErr != ErrExchange || peerErr != ErrExchange || got != (capture.Receipt{}) {
		t.Fatal("conflicting metadata received an acknowledgment")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("replay/conflict overwrote stored record")
	}
}

func TestPeerRejectsFramesBeforeAppend(t *testing.T) {
	body, _ := encodeMetadata(testMetadata())
	badBodies := [][]byte{
		bytes.Replace(body, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema_\u0076ersion":1`), 1),
		bytes.Replace(body, []byte(`"schema_version"`), []byte(`"Schema_Version"`), 1),
		bytes.Replace(body, []byte(`"schema_version":1`), []byte(`"schema_version":null`), 1),
		bytes.Replace(body, []byte(`"schema_version":1`), []byte(`"schema_version":1.0`), 1),
		bytes.Replace(body, []byte(`"event_id"`), []byte(`"tool_input":"ghp_private_fixture /synthetic/private","event_id"`), 1),
		append(bytes.Clone(body), []byte(` {}`)...),
		append([]byte(" "), body...), // Valid JSON, not the canonical wire body.
		bytes.Replace(body, []byte(`"event_id"`), []byte(`"event_\u0069d"`), 1),
		bytes.Replace(body, []byte(`"schema_version":1`), nil, 1),
		{0xff},
	}
	for n, bad := range badBodies {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			if bytes.Equal(bad, body) {
				t.Fatal("bad request mutation absent")
			}
			client, conn := unixPair(t)
			calls := 0
			p := &Peer{store: appendMetadataFunc(func(context.Context, capture.Metadata) (capture.Receipt, error) { calls++; return testReceipt(), nil })}
			if writeFrame(client, bad) != nil {
				t.Fatal("request write")
			}
			if err := p.ServeOne(exchangeContext(t), conn); err != ErrFrame || calls != 0 {
				t.Fatal("malformed/noncanonical request reached storage")
			}
			if _, err := readFrame(client); err != ErrExchange {
				t.Fatal("rejected frame left an acknowledgment")
			}
		})
	}
	for _, length := range []uint32{0, MaxFrame + 1} {
		client, conn := unixPair(t)
		var header [4]byte
		binary.BigEndian.PutUint32(header[:], length)
		if _, err := client.Write(header[:]); err != nil {
			t.Fatal(err)
		}
		p := &Peer{store: appendMetadataFunc(func(context.Context, capture.Metadata) (capture.Receipt, error) {
			t.Fatal("invalid length appended")
			return capture.Receipt{}, nil
		})}
		if p.ServeOne(exchangeContext(t), conn) != ErrFrame {
			t.Fatal("invalid length admitted")
		}
	}
	client, conn := unixPair(t)
	if _, err := client.Write([]byte{0, 0}); err != nil || client.CloseWrite() != nil {
		t.Fatal("partial request")
	}
	if NewPeer(capture.Spool{}).ServeOne(exchangeContext(t), conn) != ErrExchange {
		t.Fatal("truncated header admitted")
	}
}

func TestPeerOriginalContextAndAdmission(t *testing.T) {
	ctx := exchangeContext(t)
	client, conn := unixPair(t)
	p := &Peer{store: appendMetadataFunc(func(got context.Context, m capture.Metadata) (capture.Receipt, error) {
		if got != ctx || m != testMetadata() {
			t.Error("caller context/deadline or metadata changed")
		}
		return testReceipt(), nil
	})}
	done := make(chan error, 1)
	go func() { done <- p.ServeOne(ctx, conn) }()
	if _, err := NewClient(client).Submit(exchangeContext(t), testMetadata()); err != nil || awaitPeer(t, done) != nil {
		t.Fatal("context round trip")
	}
	expired, expire := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer expire()
	cancelled, cancel := context.WithCancel(exchangeContext(t))
	cancel()
	for _, tc := range []struct {
		ctx context.Context
		err error
	}{{nil, ErrExchange}, {context.Background(), ErrDeadline}, {expired, context.DeadlineExceeded}, {cancelled, context.Canceled}} {
		client, conn := unixPair(t)
		p := &Peer{store: appendMetadataFunc(func(context.Context, capture.Metadata) (capture.Receipt, error) {
			t.Fatal("inadmissible context appended")
			return capture.Receipt{}, nil
		})}
		if p.ServeOne(tc.ctx, conn) != tc.err {
			t.Fatal("context admission error")
		}
		if _, err := readFrame(client); err != ErrExchange {
			t.Fatal("admission failure did not close connection")
		}
	}
	for _, p := range []*Peer{nil, {}} {
		client, conn := unixPair(t)
		if p.ServeOne(exchangeContext(t), conn) != ErrExchange {
			t.Fatal("unconfigured peer admitted")
		}
		if _, err := readFrame(client); err != ErrExchange {
			t.Fatal("unconfigured peer retained connection")
		}
	}
	if p.ServeOne(ctx, nil) != ErrExchange || p.ServeOne(ctx, &net.UnixConn{}) != ErrExchange {
		t.Fatal("invalid connection admitted")
	}
}

func TestPeerCancellationUnblocksReadWithoutAppend(t *testing.T) {
	client, conn := unixPair(t)
	ctx, cancel := context.WithCancel(exchangeContext(t))
	defer cancel()
	done := make(chan error, 1)
	p := &Peer{store: appendMetadataFunc(func(context.Context, capture.Metadata) (capture.Receipt, error) {
		t.Error("empty cancelled request appended")
		return capture.Receipt{}, nil
	})}
	go func() { done <- p.ServeOne(ctx, conn) }()
	cancel()
	if awaitPeer(t, done) != context.Canceled {
		t.Fatal("cancelled read not interrupted")
	}
	if _, err := readFrame(client); err != ErrExchange {
		t.Fatal("cancelled read retained connection")
	}
}

func TestPeerBlockedAppendKeepsAdmissionWhileClientReturns(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	releaseStore := func() { once.Do(func() { close(release) }) }
	defer releaseStore()
	var calls atomic.Int32
	p := &Peer{store: appendMetadataFunc(func(context.Context, capture.Metadata) (capture.Receipt, error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release // Deliberately non-cooperative storage, not a timing sample.
		}
		return testReceipt(), nil
	})}
	client, conn := unixPair(t)
	peerCtx, cancelPeer := context.WithCancel(exchangeContext(t))
	defer cancelPeer()
	clientCtx, cancelClient := context.WithCancel(exchangeContext(t))
	defer cancelClient()
	peerDone := make(chan error, 1)
	go func() { peerDone <- p.ServeOne(peerCtx, conn) }()
	type result struct {
		receipt capture.Receipt
		err     error
	}
	clientDone := make(chan result, 1)
	go func() { r, err := NewClient(client).Submit(clientCtx, testMetadata()); clientDone <- result{r, err} }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("append not entered")
	}
	cancelPeer()
	cancelClient()
	select {
	case got := <-clientDone:
		if got.err != context.Canceled || got.receipt != (capture.Receipt{}) {
			t.Fatal("blocked storage acknowledged")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("client wait retained blocked append")
	}
	select {
	case <-peerDone:
		t.Fatal("cancellation released blocked append/admission")
	default:
	}
	body, _ := encodeMetadata(testMetadata())
	for range 16 {
		otherClient, otherConn := unixPair(t)
		if writeFrame(otherClient, body) != nil {
			t.Fatal("overlap fixture")
		}
		if p.ServeOne(exchangeContext(t), otherConn) != ErrBusy {
			t.Fatal("overlap admitted while cancelled append blocked")
		}
		if _, err := readFrame(otherClient); err != ErrExchange {
			t.Fatal("rejected overlap connection remained open")
		}
	}
	if calls.Load() != 1 {
		t.Fatal("overlap reached storage")
	}
	releaseStore()
	if awaitPeer(t, peerDone) != context.Canceled {
		t.Fatal("late append acknowledged after peer cancellation")
	}
	if _, clientErr, peerErr := peerExchange(t, p, testMetadata()); clientErr != nil || peerErr != nil || calls.Load() != 2 {
		t.Fatal("peer failed to release admission after append returned")
	}
}

func TestPeerLostAckReplaysRealDurableRecord(t *testing.T) {
	spool := capture.Spool{Directory: filepath.Join(t.TempDir(), "spool")}
	published := make(chan capture.Receipt, 1)
	release := make(chan struct{})
	var once sync.Once
	releaseAck := func() { once.Do(func() { close(release) }) }
	defer releaseAck()
	var calls atomic.Int32
	p := &Peer{store: appendMetadataFunc(func(ctx context.Context, m capture.Metadata) (capture.Receipt, error) {
		r, err := spool.Append(ctx, m)
		if calls.Add(1) == 1 {
			published <- r
			<-release
		}
		return r, err
	})}
	client, conn := unixPair(t)
	ctx, cancel := context.WithCancel(exchangeContext(t))
	defer cancel()
	peerDone := make(chan error, 1)
	go func() { peerDone <- p.ServeOne(exchangeContext(t), conn) }()
	clientDone := make(chan error, 1)
	go func() {
		r, err := NewClient(client).Submit(ctx, testMetadata())
		if r != (capture.Receipt{}) {
			clientDone <- errors.New("unexpected receipt")
			return
		}
		clientDone <- err
	}()
	var original capture.Receipt
	select {
	case original = <-published:
	case <-time.After(5 * time.Second):
		t.Fatal("record not published")
	}
	if !validReceipt(testMetadata(), original) || original.Duplicate {
		t.Fatal("publication failed")
	}
	cancel()
	if awaitPeer(t, clientDone) != context.Canceled {
		t.Fatal("lost acknowledgment not preserved")
	}
	releaseAck()
	if err := awaitPeer(t, peerDone); err != nil && err != ErrExchange {
		t.Fatal("late peer failure leaked diagnostics")
	}
	replayed, clientErr, peerErr := peerExchange(t, p, testMetadata())
	want := original
	want.Duplicate = true
	if clientErr != nil || peerErr != nil || replayed != want {
		t.Fatal("published but unacknowledged record not replayed exactly")
	}
}

func TestPeerAppendFailuresPanicsAndReceipts(t *testing.T) {
	for _, mode := range []string{"error", "wrapped storage", "panic", "nil panic", "wrong identity", "zero time", "promoted state", "promoted origin", "promoted coverage"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			p := &Peer{store: appendMetadataFunc(func(context.Context, capture.Metadata) (capture.Receipt, error) {
				calls++
				if calls > 1 {
					return testReceipt(), nil
				}
				r := testReceipt()
				switch mode {
				case "error":
					return r, errors.New("ghp_peer_private /synthetic/private")
				case "wrapped storage":
					return r, fmt.Errorf("ghp_peer_private: %w", capture.ErrCapacity)
				case "panic":
					panic("ghp_peer_private /synthetic/private")
				case "nil panic":
					panic(nil)
				case "wrong identity":
					r.EventID = r.EventID[:len(r.EventID)-1] + "f"
				case "zero time":
					r.CapturedAt = time.Time{}
				case "promoted state":
					r.State = "accepted"
				case "promoted origin":
					r.Origin = "first_party"
				case "promoted coverage":
					r.Coverage = "complete"
				}
				return r, nil
			})}
			r, clientErr, peerErr := peerExchange(t, p, testMetadata())
			if r != (capture.Receipt{}) || clientErr != ErrExchange || peerErr != ErrExchange || strings.Contains(peerErr.Error(), "ghp_") {
				t.Fatal("unsafe receipt, panic or private diagnostic escaped")
			}
			if _, clientErr, peerErr := peerExchange(t, p, testMetadata()); clientErr != nil || peerErr != nil {
				t.Fatal("failure retained admission")
			}
		})
	}
}
