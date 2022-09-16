package dcnet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"strings"

	"decred.org/cspp/v2/chacha20prng"
	"decred.org/cspp/v2/x25519"
	"github.com/companyzero/sntrup4591761"
	"github.com/decred/dcrd/crypto/blake256"
)

// SRMixPads creates a vector of exponential DC-net pads from a vector of
// shared secrets with each participating peer in the DC-net.
func SRMixPads(kp [][]byte, my int) []*big.Int {
	h := blake256.New()
	scratch := make([]byte, 8)

	pads := make([]*big.Int, len(kp))
	partialPad := new(big.Int)
	for j := 0; j < len(kp); j++ {
		pads[j] = new(big.Int)
		for i := 0; i < len(kp); i++ {
			if my == i {
				continue
			}
			binary.LittleEndian.PutUint64(scratch, uint64(j)+1)
			h.Reset()
			h.Write(kp[i])
			h.Write(scratch)
			digest := h.Sum(nil)
			partialPad.SetBytes(digest)
			if my > i {
				pads[j].Add(pads[j], partialPad)
			} else {
				pads[j].Sub(pads[j], partialPad)
			}
		}
		pads[j].Mod(pads[j], F)
	}
	return pads
}

// SRMix creates the padded {m**1, m**2, ..., m**n} message exponentials
// vector.  Message must be bounded by the field prime and must be unique to
// every exponential SR run in a mix session to ensure anonymity.
func SRMix(m *big.Int, pads []*big.Int) []*big.Int {
	mix := make([]*big.Int, len(pads))
	exp := new(big.Int)
	for i := int64(0); i < int64(len(mix)); i++ {
		mexp := new(big.Int).Exp(m, exp.SetInt64(i+1), nil)
		mix[i] = mexp.Add(mexp, pads[i])
		mix[i].Mod(mix[i], F)
	}
	return mix
}

// AddVectors sums each vector element over F, returning a new vector.  When
// peers are honest (DC-mix pads sum to zero) this creates the unpadded vector
// of message power sums.
func AddVectors(vs ...[]*big.Int) []*big.Int {
	sums := make([]*big.Int, len(vs))
	for i := range sums {
		sums[i] = new(big.Int)
		for j := range vs {
			sums[i].Add(sums[i], vs[j][i])
		}
		sums[i].Mod(sums[i], F)
	}
	return sums
}

// Coefficients calculates a{0}..a{n} for the polynomial:
//   g(x) = a{0} + a{1}x + a{2}x**2 + ... + a{n-1}x**(n-1) + a{n}x**n  (mod F)
// where
//   a{n}   = -1
//   a{n-1} = -(1/1) *    a{n}*S{0}
//   a{n-2} = -(1/2) * (a{n-1}*S{0} +   a{n}*S{1})
//   a{n-3} = -(1/3) * (a{n-2}*S{0} + a{n-1}*S{1} + a{n}*S{2})
//   ...
//
// The roots of this polynomial are the set of recovered messages.
//
// Note that the returned slice of coefficients is one element larger than the
// slice of partial sums.
func Coefficients(S []*big.Int) []*big.Int {
	n := len(S) + 1
	a := make([]*big.Int, n)
	a[len(a)-1] = big.NewInt(-1)
	a[len(a)-1].Add(a[len(a)-1], F) // a{n} = -1 (mod F) = F - 1
	scratch := new(big.Int)
	for i := 0; i < len(a)-1; i++ {
		a[n-2-i] = new(big.Int)
		for j := 0; j <= i; j++ {
			a[n-2-i].Add(a[n-2-i], scratch.Mul(a[n-1-i+j], S[j]))
		}
		xinv := scratch.ModInverse(scratch.SetInt64(int64(i)+1), F)
		xinv.Neg(xinv)
		a[n-2-i].Mul(a[n-2-i], xinv)
		a[n-2-i].Mod(a[n-2-i], F)
	}
	return a
}

// IsRoot checks that the message m is a root of the polynomial with
// coefficients a (mod F) without solving for every root.
func IsRoot(m *big.Int, a []*big.Int) bool {
	sum := new(big.Int)
	scratch := new(big.Int)
	for i := range a {
		scratch.Exp(m, scratch.SetInt64(int64(i)), F)
		scratch.Mul(scratch, a[i])
		sum.Add(sum, scratch)
	}
	sum.Mod(sum, F)
	return sum.Sign() == 0
}

// Vec is a N-element vector of Msize []byte messages.
type Vec struct {
	N     int
	Msize int
	Data  []byte
}

// NewVec returns a zero vector for holding n messages of msize length.
func NewVec(n, msize int) *Vec {
	return &Vec{
		N:     n,
		Msize: msize,
		Data:  make([]byte, n*msize),
	}
}

// IsDim returns whether the Vec has dimensions n-by-msize.
func (v *Vec) IsDim(n, msize int) bool {
	return v.N == n && v.Msize == msize && len(v.Data) == n*msize
}

// Equals returns whether the two vectors have equal dimensions and data.
func (v *Vec) Equals(other *Vec) bool {
	return other.IsDim(v.N, v.Msize) && bytes.Equal(other.Data, v.Data)
}

// M returns the i'th message of the vector.
func (v *Vec) M(i int) []byte {
	return v.Data[i*v.Msize : i*v.Msize+v.Msize]
}

func (v *Vec) String() string {
	b := new(strings.Builder)
	b.Grow(2 + v.N*(2*v.Msize+1))
	b.WriteString("[")
	for i := 0; i < v.N; i++ {
		if i != 0 {
			b.WriteString(" ")
		}
		fmt.Fprintf(b, "%x", v.M(i))
	}
	b.WriteString("]")
	return b.String()
}

// Aliases for sntrup4591761 types
type (
	PQSecretKey  = [sntrup4591761.PrivateKeySize]byte
	PQPublicKey  = [sntrup4591761.PublicKeySize]byte
	PQCiphertext = [sntrup4591761.CiphertextSize]byte
)

// KX contains the client public and secret keys to perform shared key exchange
// with other peers.
type KX struct {
	X25519       *x25519.KX
	PQPublic     *[sntrup4591761.PublicKeySize]byte
	PQSecret     *[sntrup4591761.PrivateKeySize]byte
	PQCleartexts []*[sntrup4591761.SharedKeySize]byte
}

// NewKX generates X25519 and Sntrup4591761 public and secret keys from a PRNG.
func NewKX(prng io.Reader) (*KX, error) {
	kx := new(KX)
	var err error
	kx.X25519, err = x25519.New(prng)
	if err != nil {
		return nil, err
	}
	pk, sk, err := sntrup4591761.GenerateKey(prng)
	if err != nil {
		return nil, err
	}
	kx.PQPublic = pk
	kx.PQSecret = sk
	return kx, nil
}

// Encapsulate performs encapsulation for sntrup4591761 key exchanges with each
// other peer in the DC-net.  It populates the PQCleartexts field of kx and
// return encrypted cyphertexts of these shared keys.
//
// Encapsulation in the DC-net requires randomness from a CSPRNG seeded by a
// committed secret; blame assignment is not possible otherwise.
func (kx *KX) Encapsulate(prng io.Reader, pubkeys []*PQPublicKey, my int) ([]*PQCiphertext, error) {
	cts := make([]*[sntrup4591761.CiphertextSize]byte, len(pubkeys))
	kx.PQCleartexts = make([]*[32]byte, len(pubkeys))

	for i, pk := range pubkeys {
		ciphertext, cleartext, err := sntrup4591761.Encapsulate(prng, pk)
		if err != nil {
			return nil, err
		}
		cts[i] = ciphertext
		kx.PQCleartexts[i] = cleartext
	}

	return cts, nil
}

// SharedKeys creates the pairwise SR and DC shared secret keys for
// mcounts[myvk] mixes.  ecdhPubs, cts, and mcounts must all share the same
// slice length.
func SharedKeys(kx *KX, ecdhPubs []*x25519.Public, cts []*PQCiphertext, sid []byte, msize, run, myvk int, mcounts []int) (sr [][][]byte, dc [][]*Vec, err error) {
	if len(ecdhPubs) != len(mcounts) {
		panic("number of x25519 public keys must match total number of peers")
	}
	if len(cts) != len(mcounts) {
		panic("number of ciphertexts must match total number of peers")
	}

	mcount := mcounts[myvk]
	var mtot int
	for i := range mcounts {
		mtot += mcounts[i]
	}

	h := blake256.New()
	sr = make([][][]byte, mcount)
	dc = make([][]*Vec, mcount)

	for i := 0; i < mcount; i++ {
		sr[i] = make([][]byte, mtot)
		dc[i] = make([]*Vec, mtot)
		var m int
		for peer := 0; peer < len(mcounts); peer++ {
			if peer == myvk && mcount == 1 {
				m++
				continue
			}

			x25519Pub := ecdhPubs[peer]
			var sharedKey []byte
			sharedKey, err = kx.X25519.SharedKey(x25519Pub)
			if err != nil {
				return
			}
			pqSharedKey, ok := sntrup4591761.Decapsulate(cts[peer], kx.PQSecret)
			if ok != 1 {
				err = fmt.Errorf("sntrup4591761: decapsulate failure")
				return
			}

			// XOR x25519 and both sntrup4591761 keys into a single
			// shared key. If sntrup4591761 is discovered to be
			// broken in the future, the security only reduces to
			// that of x25519.
			// If the message belongs to our own peer, only XOR
			// the sntrup4591761 key once.  The decapsulated and
			// cleartext keys are equal in this case, and would
			// cancel each other out otherwise.
			xor := func(dst, src []byte) {
				if len(dst) != len(src) {
					panic("dcnet: different lengths in xor")
				}
				for i := range dst {
					dst[i] ^= src[i]
				}
			}
			xor(sharedKey, pqSharedKey[:])
			if peer != myvk {
				xor(sharedKey, kx.PQCleartexts[peer][:])
			}

			// Create the prefix of a PRNG seed preimage.  A counter
			// will be appended before creating each PRNG, one for
			// each message pair.
			prngSeedPreimage := make([]byte, len(sid)+len(sharedKey)+4)
			l := copy(prngSeedPreimage, sid)
			l += copy(prngSeedPreimage[l:], sharedKey)
			seedCounterBytes := prngSeedPreimage[l:]

			// Read from the PRNG to create shared keys for each
			// message the peer is mixing.
			for j := 0; j < mcounts[peer]; j++ {
				if myvk == peer && j == i {
					m++
					continue
				}

				// Create the PRNG seed using the combined shared key.
				// A unique seed is generated for each message pair,
				// determined using the message index of the peer with
				// the lower peer index.  The PRNG nonce is the message
				// number of the peer with the higher peer index.
				// When creating shared keys with our own peer, the PRNG
				// seed counter and nonce must be reversed for the second
				// half of our generated keys.
				seedCounter := i
				nonce := j
				if myvk > peer || (myvk == peer && j > i) {
					seedCounter = j
					nonce = i
				}
				binary.LittleEndian.PutUint32(seedCounterBytes, uint32(seedCounter))

				h.Reset()
				h.Write(prngSeedPreimage)
				prngSeed := h.Sum(nil)
				prng := chacha20prng.New(prngSeed, uint32(nonce))

				sr[i][m] = prng.Next(32)
				dc[i][m] = &Vec{
					N:     mtot,
					Msize: msize,
					Data:  prng.Next(mtot * msize),
				}

				m++
			}
		}
	}
	return
}

// DCMixPads creates the vector of DC-net pads from shared secrets with each mix
// participant.
func DCMixPads(kp []*Vec, msize, my int) *Vec {
	n := len(kp)
	pads := &Vec{
		N:     n,
		Msize: msize,
		Data:  make([]byte, n*msize),
	}
	for i := range kp {
		if i == my {
			continue
		}
		pads.Xor(pads, kp[i])
	}
	return pads
}

// DCMix creates the DC-net vector of message m xor'd into m's reserved
// anonymous slot position of the pads DC-net pads.  Panics if len(m) is not the
// vector's message size.
func DCMix(pads *Vec, m []byte, slot int) *Vec {
	dcmix := *pads
	dcmix.Data = make([]byte, len(pads.Data))
	copy(dcmix.Data, pads.Data)
	slotm := dcmix.M(slot)
	if len(m) != len(slotm) {
		panic("message sizes are not equal")
	}
	for i := range m {
		slotm[i] ^= m[i]
	}
	return &dcmix
}

// Xor writes the xor of each vector element of src1 and src2 into v.
// Source and destination vectors are allowed to be equal.
// Panics if vectors do not share identical dimensions.
func (v *Vec) Xor(src1, src2 *Vec) {
	switch {
	case v.N != src1.N, v.Msize != src1.Msize, len(v.Data) != len(src1.Data):
		fallthrough
	case v.N != src2.N, v.Msize != src2.Msize, len(v.Data) != len(src2.Data):
		panic("dcnet: vectors do not share identical dimensions")
	}
	for i := range v.Data {
		v.Data[i] = src1.Data[i] ^ src2.Data[i]
	}
}

// XorVectors calculates the xor of all vectors.
// Panics if vectors do not share identical dimensions.
func XorVectors(vs []*Vec) *Vec {
	msize := vs[0].Msize
	res := NewVec(len(vs), msize)
	for _, v := range vs {
		res.Xor(res, v)
	}
	return res
}
