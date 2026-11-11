package captureipc

import "golang.org/x/sys/unix"

func socketPeerIdentity(fd int) (PeerIdentity, error) {
	cred, err := unix.GetsockoptXucred(fd, unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
	// Apple defines XUCRED_VERSION as zero. Unknown layouts are not admitted.
	if err != nil || cred == nil || cred.Version != 0 || cred.Uid == ^uint32(0) {
		return PeerIdentity{}, ErrAdmission
	}
	pid, err := unix.GetsockoptInt(fd, unix.SOL_LOCAL, unix.LOCAL_PEERPID)
	if err != nil || pid <= 0 {
		return PeerIdentity{}, ErrAdmission
	}
	return PeerIdentity{PID: pid, UID: cred.Uid}, nil
}
