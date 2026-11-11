package captureipc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/capture"
)

func credentialHost(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("this OS has no admitted credential reader")
	}
}

func ownIdentity() PeerIdentity { return PeerIdentity{PID: os.Getpid(), UID: uint32(os.Geteuid())} }

func TestAdmissionPreservesConnectionBytesAndDeadlines(t *testing.T) {
	credentialHost(t)
	client, conn := unixPair(t)
	payload := []byte("unread synthetic metadata marker")
	if _, err := client.Write(payload); err != nil {
		t.Fatal(err)
	}
	// Credential queries must not reset descriptor deadlines to the context's
	// later deadline or consume any of the already buffered socket payload.
	if conn.SetDeadline(time.Unix(1, 0)) != nil {
		t.Fatal("deadline fixture")
	}
	got, err := AdmitPeer(exchangeContext(t), conn, ownIdentity())
	if err != nil || got != conn {
		t.Fatal("own credentials rejected or descriptor replaced")
	}
	var probe [1]byte
	if _, err := got.Read(probe[:]); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal("read deadline changed")
	}
	if _, err := got.Write(probe[:]); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal("write deadline changed")
	}
	if got.SetDeadline(time.Now().Add(5*time.Second)) != nil {
		t.Fatal("deadline restore")
	}
	read := make([]byte, len(payload))
	if _, err := io.ReadFull(got, read); err != nil || !bytes.Equal(read, payload) {
		t.Fatal("credential check consumed socket payload")
	}
	if _, err := got.Write([]byte("Y")); err != nil {
		t.Fatal("admitted connection closed")
	}
	if _, err := io.ReadFull(client, probe[:]); err != nil || probe[0] != 'Y' {
		t.Fatal("wrong admitted connection")
	}
}

func TestAdmissionRejectsPolicyMismatchAndConsumesSocket(t *testing.T) {
	credentialHost(t)
	identity := ownIdentity()
	policies := []PeerIdentity{
		{PID: 0, UID: identity.UID}, {PID: -1, UID: identity.UID},
		{PID: identity.PID + 1, UID: identity.UID},
		{PID: identity.PID, UID: identity.UID ^ 1},
		{PID: identity.PID, UID: ^uint32(0)},
	}
	if strconv.IntSize == 64 {
		tooLarge := int64(1) << 31
		policies = append(policies, PeerIdentity{PID: int(tooLarge), UID: identity.UID})
	}
	if identity.UID != 0 {
		policies = append(policies, PeerIdentity{PID: identity.PID, UID: 0})
	}
	for _, expected := range policies {
		client, conn := unixPair(t)
		got, err := AdmitPeer(exchangeContext(t), conn, expected)
		if got != nil || err != ErrAdmission {
			t.Fatal("mismatched/invalid identity admitted")
		}
		if _, err := readFrame(client); err != ErrExchange {
			t.Fatal("rejection retained socket")
		}
	}
}

func TestAdmissionContextFailuresAndPanics(t *testing.T) {
	expired, expire := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer expire()
	cancelled, cancel := context.WithCancel(exchangeContext(t))
	cancel()
	for _, tc := range []struct {
		name string
		ctx  context.Context
		err  error
	}{
		{"nil", nil, ErrAdmission}, {"missing deadline", context.Background(), ErrDeadline},
		{"expired", expired, context.DeadlineExceeded}, {"cancelled", cancelled, context.Canceled},
		{"elapsed before notification", elapsedContext{context.Background()}, context.DeadlineExceeded},
		{"exotic", exoticContext{exchangeContext(t)}, ErrAdmission},
		{"panic", panicContext{exchangeContext(t), false}, ErrAdmission},
		{"nil panic", panicContext{exchangeContext(t), true}, ErrAdmission},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, conn := unixPair(t)
			got, err := AdmitPeer(tc.ctx, conn, ownIdentity())
			if got != nil || err != tc.err {
				t.Fatal("context failure escaped or admitted connection")
			}
			if _, err := readFrame(client); err != ErrExchange {
				t.Fatal("context failure retained socket")
			}
		})
	}
}

func TestAdmissionRejectsInvalidDescriptors(t *testing.T) {
	for _, conn := range []*net.UnixConn{nil, {}} {
		if got, err := AdmitPeer(exchangeContext(t), conn, ownIdentity()); got != nil || err != ErrAdmission {
			t.Fatal("invalid descriptor admitted")
		}
	}
	client, conn := unixPair(t)
	if conn.Close() != nil {
		t.Fatal("closed fixture")
	}
	if got, err := AdmitPeer(exchangeContext(t), conn, ownIdentity()); got != nil || err != ErrAdmission {
		t.Fatal("closed connection admitted")
	}
	_ = client.Close()
	dir, err := os.MkdirTemp("/tmp", "ganesh-cred-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	gram, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Net: "unixgram", Name: filepath.Join(dir, "peer.sock")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gram.Close() })
	if got, err := AdmitPeer(exchangeContext(t), gram, ownIdentity()); got != nil || err != ErrAdmission {
		t.Fatal("datagram admitted")
	}
	if _, err := socketPeerIdentity(-1); err != ErrAdmission {
		t.Fatal("kernel query failure leaked or admitted")
	}
}

func TestAdmissionComposesWithClientPeerAndSpool(t *testing.T) {
	credentialHost(t)
	for _, reject := range []bool{false, true} {
		client, conn := unixPair(t)
		ctx := exchangeContext(t)
		p := NewPeer(capture.Spool{Directory: filepath.Join(t.TempDir(), "spool")})
		calls := 0
		if reject {
			p.store = appendMetadataFunc(func(context.Context, capture.Metadata) (capture.Receipt, error) { calls++; return testReceipt(), nil })
		}
		admitted, err := AdmitPeer(ctx, client, ownIdentity())
		if err != nil {
			t.Fatal("local expected receiver not admitted")
		}
		done := make(chan error, 1)
		go func() {
			expected := ownIdentity()
			if reject {
				expected.PID++
			}
			admitted, err := AdmitPeer(ctx, conn, expected)
			if err == nil {
				err = p.ServeOne(ctx, admitted)
			}
			done <- err
		}()
		r, err := NewClient(admitted).Submit(ctx, testMetadata())
		peerErr := awaitPeer(t, done)
		if reject {
			if err != ErrExchange || peerErr != ErrAdmission || r != (capture.Receipt{}) || calls != 0 {
				t.Fatal("rejected origin reached append or receipt")
			}
		} else if err != nil || peerErr != nil || !validReceipt(testMetadata(), r) || r.Duplicate {
			t.Fatal("admitted durable round trip failed")
		}
	}
}

// Runs only as a bounded child of TestAdmissionSeparateProcessCredentials.
// The child sends an opaque marker and waits, keeping its process identity
// live while the parent reads credentials using an independently known PID.
func TestAdmissionCredentialHelper(t *testing.T) {
	if os.Getenv("GANESH_CREDENTIAL_HELPER") != "1" {
		return
	}
	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Net: "unix", Name: os.Getenv("GANESH_CREDENTIAL_SOCKET")})
	if err != nil {
		t.Fatal("credential helper connection failed")
	}
	defer conn.Close()
	if conn.SetDeadline(time.Now().Add(5*time.Second)) != nil {
		t.Fatal("helper deadline")
	}
	if _, err := conn.Write([]byte("X")); err != nil {
		t.Fatal("helper marker")
	}
	var marker [1]byte
	_, _ = conn.Read(marker[:]) // Rejection closes the parent endpoint; either outcome ends the fixture.
}

func TestAdmissionSeparateProcessCredentials(t *testing.T) {
	credentialHost(t)
	for _, mode := range []string{"exact child", "same UID wrong process", "wrong UID"} {
		t.Run(mode, func(t *testing.T) {
			dir, err := os.MkdirTemp("/tmp", "ganesh-cred-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			address := &net.UnixAddr{Net: "unix", Name: filepath.Join(dir, "child.sock")}
			listener, err := net.ListenUnix("unix", address)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = listener.Close() })
			if os.Chmod(address.Name, 0600) != nil || listener.SetDeadline(time.Now().Add(10*time.Second)) != nil {
				t.Fatal("listener fixture")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAdmissionCredentialHelper$", "-test.timeout=8s")
			cmd.Env = []string{"GANESH_CREDENTIAL_HELPER=1", "GANESH_CREDENTIAL_SOCKET=" + address.Name, "GODEBUG=" + os.Getenv("GODEBUG")}
			cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
			if cmd.Start() != nil {
				t.Fatal("credential helper launch")
			}
			childPID := cmd.Process.Pid
			waited := false
			t.Cleanup(func() {
				if !waited {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
			})
			conn, err := listener.AcceptUnix()
			if err != nil {
				t.Fatal("child connection missing")
			}
			t.Cleanup(func() { _ = conn.Close() })
			expected := PeerIdentity{PID: childPID, UID: uint32(os.Geteuid())}
			if mode == "same UID wrong process" {
				expected.PID = os.Getpid()
			}
			if mode == "wrong UID" {
				expected.UID ^= 1
			}
			admitted, err := AdmitPeer(exchangeContext(t), conn, expected)
			if mode == "exact child" {
				if err != nil || admitted != conn {
					t.Fatal("actual child credentials mismatch")
				}
				var marker [1]byte
				if _, err := io.ReadFull(admitted, marker[:]); err != nil || marker[0] != 'X' {
					t.Fatal("admission consumed child bytes")
				}
				if _, err := admitted.Write([]byte("Y")); err != nil {
					t.Fatal("admitted child closed")
				}
				_ = admitted.Close()
			} else if admitted != nil || err != ErrAdmission {
				t.Fatal("different expected process/user admitted")
			}
			err = cmd.Wait()
			waited = true
			if err != nil {
				t.Fatal("credential helper failed")
			}
		})
	}
}

func TestUnsupportedCredentialReaderFailsClosed(t *testing.T) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		t.Skip("unsupported-platform contract")
	}
	client, conn := unixPair(t)
	if got, err := AdmitPeer(exchangeContext(t), conn, ownIdentity()); got != nil || err != ErrAdmission {
		t.Fatal("unsupported credential check admitted")
	}
	if _, err := readFrame(client); err != ErrExchange {
		t.Fatal("unsupported reader retained connection")
	}
}
