package captureipc

import "golang.org/x/sys/unix"

func socketPeerIdentity(fd int) (PeerIdentity, error) {
	cred, err := unix.GetsockoptUcred(fd, unix.SOL_SOCKET, unix.SO_PEERCRED)
	if err != nil || cred == nil || cred.Pid <= 0 || cred.Uid == ^uint32(0) {
		return PeerIdentity{}, ErrAdmission
	}
	return PeerIdentity{PID: int(cred.Pid), UID: cred.Uid}, nil
}
