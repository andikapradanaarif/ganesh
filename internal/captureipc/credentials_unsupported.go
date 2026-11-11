//go:build !darwin && !linux

package captureipc

func socketPeerIdentity(int) (PeerIdentity, error) {
	return PeerIdentity{}, ErrAdmission
}
