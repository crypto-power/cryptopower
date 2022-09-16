// Package x25519 implements ECDHE over curve25519.
package x25519

import (
	"io"

	"golang.org/x/crypto/curve25519"
)

type Public [32]byte
type Scalar [32]byte

// KX is the client-generated public and secret portions of a key exchange.
type KX struct {
	Public
	Scalar // secret
}

// New begins a new key exchange by generating a public and secret value.
// Public portions must be exchanged between parties to derive a shared secret
// key.
func New(rand io.Reader) (*KX, error) {
	kx := new(KX)
	_, err := io.ReadFull(rand, kx.Scalar[:])
	if err != nil {
		return nil, err
	}

	// https://cr.yp.to/ecdh.html; Computing secret keys.
	kx.Scalar[0] &= 248
	kx.Scalar[31] &= 127
	kx.Scalar[31] |= 64

	public, err := curve25519.X25519(kx.Scalar[:], curve25519.Basepoint)
	if err != nil {
		return nil, err
	}
	copy(kx.Public[:], public)

	return kx, nil
}

// SharedKey computes a shared key with the other party from our secret value
// and their public value.  The result should be securely hashed before usage.
func (kx *KX) SharedKey(theirPublic *Public) ([]byte, error) {
	return curve25519.X25519(kx.Scalar[:], theirPublic[:])
}

// ValidScalar returns whether a secret X25519 scalar was properly constructed.
func ValidScalar(s *Scalar) bool {
	return s[0]&248 == s[0] && s[31]&127 == s[31] && s[31]|64 == s[31]
}
