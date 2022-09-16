package sbox

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"math/big"
	"sync"

	"golang.org/x/crypto/nacl/secretbox"
)

const (
	versionLen = 4  // size of a uint32
	nonceLen   = 24 // nonce is 24 bytes
)

var (
	magic    = []byte{'s', 'b', 'o', 'x'} // magic prefix for packed blobs
	magicLen = len(magic)                 // length of the magix prefix

	// ErrInvalidHeader is returned when the header is too short.
	ErrInvalidHeader = errors.New("invalid sbox header")

	// ErrInvalidMagic is returned is the header does not start with magic
	// prefix.
	ErrInvalidMagic = errors.New("invalid magic")

	// ErrCouldNotDecrypt is returned when the secret box decryption fails.
	ErrCouldNotDecrypt = errors.New("could not decrypt")

	// ErrInvalidNonce is returned when a user provided nonce is of an
	// invalid size. A user provided nonce must be 0 < N <= 24.
	ErrInvalidNonce = errors.New("invalid nonce size")
)

// NewKey generates a new secret key for a NACL secret box. This key must not
// be disclosed.
func NewKey() (*[32]byte, error) {
	var k [32]byte

	_, err := io.ReadFull(rand.Reader, k[:])
	if err != nil {
		return nil, err
	}

	return &k, nil
}

// Decrypt decrypts the packed blob using provided key. It unpacks the sbox
// header and returns the version and unencrypted data if successful.
func Decrypt(key *[32]byte, packed []byte) ([]byte, uint32, error) {
	if len(packed) < magicLen+versionLen+nonceLen {
		return nil, 0, ErrInvalidHeader
	}

	// verify magic
	if !bytes.Equal(packed[0:magicLen], magic) {
		return nil, 0, ErrInvalidMagic
	}

	// unpack version
	version := binary.BigEndian.Uint32(packed[magicLen : magicLen+versionLen])

	var nonce [24]byte
	offset := magicLen + versionLen
	copy(nonce[:], packed[offset:offset+nonceLen])

	decrypted, ok := secretbox.Open(nil, packed[offset+nonceLen:],
		&nonce, key)
	if !ok {
		return nil, 0, ErrCouldNotDecrypt
	}

	return decrypted, version, nil
}

// encrypt returns an encrypted blob that is prefixed with the version and
// nonce.
func encrypt(version uint32, key *[32]byte, nonce [24]byte, data []byte) ([]byte, error) {
	// version
	v := make([]byte, 4)
	binary.BigEndian.PutUint32(v, version)

	// encrypt data
	blob := secretbox.Seal(nil, data, &nonce, key)

	// pack all the things
	packed := make([]byte, len(magic)+len(v)+len(nonce)+len(blob))
	copy(packed[0:], magic)
	copy(packed[len(magic):], v)
	copy(packed[len(magic)+len(v):], nonce[:])
	copy(packed[len(magic)+len(v)+len(nonce):], blob)

	return packed, nil
}

// Encrypt encrypts data with the provided key and generates a random nonce.
// Note that it is the callers responsibility to ensure that a nonce is NEVER
// reused with the same key.  It prefixes the encrypted blob with an sbox
// header which encodes the provided version. The user provided version can be
// used as a hint to identify or version the packed blob. Version is not
// inspected or used by Encrypt and Decrypt.
func Encrypt(version uint32, key *[32]byte, data []byte) ([]byte, error) {
	// random nonce
	var nonce [24]byte
	_, err := io.ReadFull(rand.Reader, nonce[:])
	if err != nil {
		return nil, err
	}
	return encrypt(version, key, nonce, data)
}

// EncryptN encrypts data with the provided key and nonce. Note that it is the
// callers responsibility to ensure that a nonce is NEVER reused with the same
// key. It prefixes the encrypted blob with an sbox header which encodes the
// provided version. The user provided version can be used as a hint to
// identify or version the packed blob. Version is not inspected or used by
// Encrypt and Decrypt.
func EncryptN(version uint32, key *[32]byte, nonce [24]byte, data []byte) ([]byte, error) {
	return encrypt(version, key, nonce, data)
}

var (
	one = big.NewInt(1)
)

// Nonce represents a valid nonce and counter that can be used as an input to
// EncryptN. Note that the caller is responsible for ensuring that a nonce is
// never reused with the same key.
type Nonce struct {
	sync.Mutex
	n *big.Int
}

// current returns the current nonce value.
// This functions must be called with the mutex held.
func (n *Nonce) current() [24]byte {
	nonce := [24]byte{}
	copy(nonce[:], n.n.Bytes())
	return nonce
}

// Current returns the current nonce value.
// This functions must be called without the mutex held.
func (n *Nonce) Current() [24]byte {
	n.Lock()
	defer n.Unlock()
	return n.current()
}

// Next returns the current nonce plus one value.
// This functions must be called without the mutex held.
func (n *Nonce) Next() [24]byte {
	n.Lock()
	defer n.Unlock()
	n.n.Add(n.n, one)
	return n.current()
}

// NewNonce returns a nonce that is set to 0.
func NewNonce() *Nonce {
	return &Nonce{
		n: new(big.Int),
	}
}

// NewNonceFromBytes returns a nonce that is set to n.
func NewNonceFromBytes(n []byte) (*Nonce, error) {
	if len(n) == 0 || len(n) > 24 {
		return nil, ErrInvalidNonce
	}
	x := make([]byte, 24)
	copy(x[:], n)
	return &Nonce{
		n: new(big.Int).SetBytes(x),
	}, nil
}
