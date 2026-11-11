package captureipc

import (
	"context"
	"errors"
	"net"
)

var ErrAdmission = errors.New("local capture peer credentials not admitted")

// PeerIdentity is an exact expected connection-origin identity supplied by a
// trusted caller. PID must be positive and fit a kernel pid_t. UID zero means
// root, not any user. Never fill this policy from the socket being checked.
// PID reuse and descriptor inheritance/transfer require separate lifecycle
// custody; this pair identifies neither executable bytes nor the current writer.
type PeerIdentity struct {
	PID int
	UID uint32
}

// AdmitPeer takes exclusive ownership of conn during a kernel credential check.
// It returns the identical connection only on an exact PID/UID match, otherwise
// closes it and returns nil. It reads/writes no socket payload, duplicates no
// descriptor and changes no deadline. Successful admission is only a credential
// predicate, not full application authentication, worker liveness or source trust.
// The caller must supply a deadline and preserve the expected process identity
// through a separately trusted lifecycle. Do not use conn concurrently here.
func AdmitPeer(ctx context.Context, conn *net.UnixConn, expected PeerIdentity) (admitted *net.UnixConn, err error) {
	completed := false
	defer func() {
		_ = recover()
		if !completed {
			admitted, err = nil, ErrAdmission
		}
		if err != nil && conn != nil {
			_ = conn.Close()
		}
	}()
	admitted, err = admitPeer(ctx, conn, expected)
	completed = true
	return
}

func admitPeer(ctx context.Context, conn *net.UnixConn, expected PeerIdentity) (*net.UnixConn, error) {
	if ctx == nil || conn == nil || expected.PID <= 0 || int64(expected.PID) > 1<<31-1 || expected.UID == ^uint32(0) {
		return nil, ErrAdmission
	}
	if err := admissionContextError(ctx); err != nil {
		return nil, err
	}
	local, remote := conn.LocalAddr(), conn.RemoteAddr()
	if local == nil || remote == nil || local.Network() != "unix" || remote.Network() != "unix" {
		return nil, ErrAdmission
	}
	raw, err := conn.SyscallConn()
	if ctxErr := admissionContextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, ErrAdmission
	}
	var observed PeerIdentity
	var queryErr error
	// Control pins the original descriptor against close/reuse for both queries.
	// File() would duplicate it and change the ownership/deadline assumptions.
	err = raw.Control(func(fd uintptr) { observed, queryErr = socketPeerIdentity(int(fd)) })
	if ctxErr := admissionContextError(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil || queryErr != nil || observed != expected {
		return nil, ErrAdmission
	}
	return conn, nil
}

func admissionContextError(ctx context.Context) error {
	err := contextError(ctx)
	switch err {
	case nil, ErrDeadline, context.Canceled, context.DeadlineExceeded:
		return err
	default:
		return ErrAdmission
	}
}
