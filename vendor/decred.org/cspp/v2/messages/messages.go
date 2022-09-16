// Package messages implements the message types communicated between client and
// server.  The messaging in a successful run is sequenced as follows:
//
//   Client | Server
//      PR -->       Pair Request
//                   (wait for epoch)
//         <-- BR    Begin Run
//      KE -->       Key Exchange
//         <-- KEs   Server broadcasts all KE messages to all peers
//      CT -->       Post-Quantum ciphertext exchange
//         <-- CTs   Server broadcasts ciphertexts created by others for us
//      SR -->       Slot Reserve
//         <-- RM    Recovered Messages
//      DC -->       DC-net broadcast
//         <-- CM    Confirm Messages (unsigned)
//      CM -->       Confirm Messages (signed)
//                   (server joins all signatures)
//         <-- CM    Confirm Messages (with all signatures)
//
// If a peer fails to find their message after either the exponential slot
// reservation or XOR DC-net, the DC or CM message indicates to the server that
// blame must be assigned to remove malicious peers from the mix.  This process
// requires secrets committed to by the KE to be revealed.
//
//   Client | Server
//      PR -->       Pair Request
//                   (wait for epoch)
//         <-- BR    Begin Run
//      KE -->       Key Exchange
//         <-- KEs   Server broadcasts all KE messages to all peers
//      CT -->       Post-Quantum ciphertext exchange
//         <-- CTs   Server broadcasts ciphertexts created by others for us
//      SR -->       Slot Reserve
//         <-- RM    Recovered Messages
//      DC -->       DC-net broadcast (with RevealSecrets=true)
//         <-- CM    Confirm Messages (with RevealSecrets=true)
//      RS -->       Reveal Secrets
//                   (server discovers misbehaving peers)
//         <-- BR    Begin Run (with removed peers)
//         ...
//
// At any point, if the server times out receiving a client message, the
// following message contains a nonzero BR field, and a new run is performed,
// beginning with a new key exchange.
package messages

import (
	"encoding"
	"encoding/binary"
	"io"
	"math/big"
	"strconv"

	"decred.org/cspp/v2/dcnet"
	"decred.org/cspp/v2/x25519"
	"github.com/companyzero/sntrup4591761"
	"github.com/decred/dcrd/crypto/blake256"
	"golang.org/x/crypto/ed25519"
)

// ServerError describes an error message sent by the server.
// The peer cannot continue in the mix session if an error is received.
// The zero value indicates the absence of an error.
type ServerError int

// Server errors
const (
	ErrAbortedSession ServerError = iota + 1
	ErrInvalidUnmixed
	ErrTooFewPeers
)

func (e ServerError) Error() string {
	switch e {
	case 0:
		return "no error"
	case ErrAbortedSession:
		return "server aborted mix session"
	case ErrInvalidUnmixed:
		return "submitted unmixed data is invalid"
	case ErrTooFewPeers:
		return "too few peers remaining to continue mix"
	default:
		return "unknown server error code " + strconv.Itoa(int(e))
	}
}

var (
	msgPR      = []byte("PR")
	msgKE      = []byte("KE")
	msgCT      = []byte("CT")
	msgSR      = []byte("SR")
	msgDC      = []byte("DC")
	msgCM      = []byte("CM")
	msgSidH    = []byte("sidH")
	msgSidHPre = []byte("sidHPre")
	msgCommit  = []byte("COMMIT")
)

func putInt(scratch []byte, v int) []byte {
	binary.BigEndian.PutUint64(scratch, uint64(v))
	return scratch
}

func writeSignedBigInt(w io.Writer, scratch []byte, bi *big.Int) {
	scratch[0] = byte(bi.Sign())
	w.Write(scratch[:1])
	b := bi.Bytes()
	w.Write(putInt(scratch, len(b)))
	w.Write(b)
}

func writeSlice(w io.Writer, scratch []byte, len int, write func(n int)) {
	w.Write(putInt(scratch, len))
	for i := 0; i < len; i++ {
		write(i)
	}
}

func writeSignedByteSlice(w io.Writer, scratch []byte, data []byte) {
	w.Write(putInt(scratch, len(data)))
	w.Write(data)
}

func sign(sk ed25519.PrivateKey, m Signed) []byte {
	if len(sk) == 0 {
		return nil
	}
	h := blake256.New()
	m.writeSigned(h)
	return ed25519.Sign(sk, h.Sum(nil))
}

func verify(pk ed25519.PublicKey, m Signed, sig []byte) bool {
	if len(sig) != ed25519.SignatureSize {
		return false
	}
	h := blake256.New()
	m.writeSigned(h)
	return ed25519.Verify(pk, h.Sum(nil), sig)
}

// Signed indicates a session message carries an ed25519 signature that
// must be checked.
type Signed interface {
	writeSigned(w io.Writer)
	VerifySignature(pub ed25519.PublicKey) bool
}

// Session describes a current mixing session and run.
type Session struct {
	sid     []byte
	sk      ed25519.PrivateKey
	vk      []ed25519.PublicKey
	run     int
	sidH    []byte
	sidHPre []byte
}

// NewSession creates a run session from a unique session identifier and peer
// ed25519 pubkeys ordered by peer index.
// If sk is non-nil, signed message types created using this session will contain
// a valid signature.
func NewSession(sid []byte, run int, sk ed25519.PrivateKey, vk []ed25519.PublicKey) *Session {
	runBytes := putInt(make([]byte, 8), run)

	h := blake256.New()
	h.Write(msgSidH)
	h.Write(sid)
	for _, k := range vk {
		if l := len(k); l != ed25519.PublicKeySize {
			panic("messages: bad ed25519 public key length: " + strconv.Itoa(l))
		}
		h.Write(k)
	}
	h.Write(runBytes)
	sidH := h.Sum(nil)

	h.Reset()
	h.Write(msgSidHPre)
	h.Write(sid)
	h.Write(runBytes)
	sidHPre := h.Sum(nil)

	return &Session{
		sid:     sid,
		sk:      sk,
		vk:      vk,
		run:     run,
		sidH:    sidH,
		sidHPre: sidHPre,
	}
}

// BinaryRepresentable is a union of the BinaryMarshaler and BinaryUnmarshaler
// interfaces.
type BinaryRepresentable interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

// PR is the client's pairing request message.
// It is only seen at the start of the protocol.
type PR struct {
	Identity       ed25519.PublicKey // Ephemeral session public key
	PairCommitment []byte            // Requirements for compatible mixes, e.g. same output amounts, tx versions, ...
	Unmixed        []byte            // Unmixed data contributed to a run result, e.g. transaction inputs and change outputs
	MessageCount   int               // Number of messages being mixed
	Signature      []byte
}

// PairRequest creates a signed request to be paired in a mix described by
// commitment, with possible initial unmixed data appearing in the final result.
// Ephemeral session keys pk and sk are used throughout the protocol.
func PairRequest(pk ed25519.PublicKey, sk ed25519.PrivateKey, commitment, unmixed []byte, mixes int) *PR {
	pr := &PR{
		Identity:       pk,
		PairCommitment: commitment,
		Unmixed:        unmixed,
		MessageCount:   mixes,
	}
	pr.Signature = sign(sk, pr)
	return pr
}

func (pr *PR) writeSigned(w io.Writer) {
	scratch := make([]byte, 8)
	w.Write(msgPR)
	writeSignedByteSlice(w, scratch, pr.Identity)
	writeSignedByteSlice(w, scratch, pr.PairCommitment)
	writeSignedByteSlice(w, scratch, pr.Unmixed)
	w.Write(putInt(scratch, pr.MessageCount))
}

func (pr *PR) VerifySignature(pub ed25519.PublicKey) bool {
	return verify(pub, pr, pr.Signature)
}

// BR is the begin run message.
// It is sent to all remaining valid peers when a new run begins.
type BR struct {
	Vk            []ed25519.PublicKey
	MessageCounts []int
	Sid           []byte
	Err           ServerError
}

// BeginRun creates the begin run message.
func BeginRun(vk []ed25519.PublicKey, mixes []int, sid []byte) *BR {
	return &BR{
		Vk:            vk,
		MessageCounts: mixes,
		Sid:           sid,
	}
}

func (br *BR) ServerError() error {
	if br.Err == 0 {
		return nil
	}
	return br.Err
}

type Sntrup4591761PublicKey = [sntrup4591761.PublicKeySize]byte
type Sntrup4591761Ciphertext = [sntrup4591761.CiphertextSize]byte

// KE is the client's opening key exchange message of a run.
type KE struct {
	Run        int // 0, 1, ...
	ECDH       *x25519.Public
	PQPK       *Sntrup4591761PublicKey
	Commitment []byte // Hash of RS (reveal secrets) message contents
	Signature  []byte
}

func (ke *KE) writeSigned(w io.Writer) {
	scratch := make([]byte, 8)
	w.Write(msgKE)
	w.Write(putInt(scratch, ke.Run))
	writeSignedByteSlice(w, scratch, ke.ECDH[:])
	writeSignedByteSlice(w, scratch, ke.PQPK[:])
	writeSignedByteSlice(w, scratch, ke.Commitment)
}

func (ke *KE) VerifySignature(pub ed25519.PublicKey) bool {
	return verify(pub, ke, ke.Signature)
}

// KeyExchange creates a signed key exchange message to verifiably provide the
// x25519 and sntrup4591761 public keys.
func KeyExchange(kx *dcnet.KX, commitment []byte, ses *Session) *KE {
	ke := &KE{
		Run:        ses.run,
		ECDH:       &kx.X25519.Public,
		PQPK:       kx.PQPublic,
		Commitment: commitment,
	}
	ke.Signature = sign(ses.sk, ke)
	return ke
}

// KEs is the server's broadcast of all received key exchange messages.
type KEs struct {
	KEs []*KE
	BR  // Indicates to begin new run after peer exclusion
	Err ServerError
}

func (kes *KEs) ServerError() error {
	if kes.Err == 0 {
		return nil
	}
	return kes.Err
}

// CT is the client's exchange of post-quantum shared key ciphertexts with all
// other peers in the run.
type CT struct {
	Ciphertexts []*Sntrup4591761Ciphertext
	Signature   []byte
}

func (ct *CT) writeSigned(w io.Writer) {
	scratch := make([]byte, 8)
	w.Write(msgCT)
	w.Write(putInt(scratch, len(ct.Ciphertexts)))
	for _, ciphertext := range ct.Ciphertexts {
		var ct []byte
		if ciphertext != nil {
			ct = ciphertext[:]
		}
		writeSignedByteSlice(w, scratch, ct)
	}
}

func (ct *CT) VerifySignature(pub ed25519.PublicKey) bool {
	return verify(pub, ct, ct.Signature)
}

// Ciphertexts creates the ciphertext message.
func Ciphertexts(ciphertexts []*Sntrup4591761Ciphertext, ses *Session) *CT {
	ct := &CT{
		Ciphertexts: ciphertexts,
	}
	ct.Signature = sign(ses.sk, ct)
	return ct
}

// CTs is the server's broadcast of encapsulated shared key ciphertexts created
// by all other peers for our client.
type CTs struct {
	Ciphertexts []*Sntrup4591761Ciphertext
	BR          // Indicates to begin a new run after peer exclusion
	Err         ServerError
}

func (cts *CTs) ServerError() error {
	if cts.Err == 0 {
		return nil
	}
	return cts.Err
}

// SR is the slot reservation broadcast.
type SR struct {
	Run       int
	DCMix     [][]*big.Int
	Signature []byte
}

func (sr *SR) writeSigned(w io.Writer) {
	scratch := make([]byte, 8)
	w.Write(msgSR)
	w.Write(putInt(scratch, sr.Run))
	w.Write(putInt(scratch, len(sr.DCMix)))
	for i := range sr.DCMix {
		writeSlice(w, scratch, len(sr.DCMix[i]), func(j int) {
			writeSignedBigInt(w, scratch, sr.DCMix[i][j])
		})
	}
}

func (sr *SR) VerifySignature(pub ed25519.PublicKey) bool {
	return verify(pub, sr, sr.Signature)
}

// SlotReserve creates a slot reservation message to discover random, anonymous
// slot assignments for an XOR DC-net by mixing random data in a exponential
// DC-mix.
func SlotReserve(dcmix [][]*big.Int, s *Session) *SR {
	sr := &SR{
		Run:   s.run,
		DCMix: dcmix,
	}
	sr.Signature = sign(s.sk, sr)
	return sr
}

// RM is the recovered messages result of collecting all SR messages and solving for
// the mixed original messages.
type RM struct {
	Run           int
	Roots         []*big.Int
	RevealSecrets bool
	BR            // Indicates to begin new run after peer exclusion
	Err           ServerError
}

func (rm *RM) ServerError() error {
	if rm.Err == 0 {
		return nil
	}
	return rm.Err
}

// RecoveredMessages creates a recovered messages message.
func RecoveredMessages(roots []*big.Int, s *Session) *RM {
	return &RM{
		Run:   s.run,
		Roots: roots,
	}
}

// DC is the DC-net broadcast.
type DC struct {
	Run           int
	DCNet         []*dcnet.Vec
	RevealSecrets bool
	Signature     []byte
}

func (dc *DC) writeSigned(w io.Writer) {
	scratch := make([]byte, 8)
	w.Write(msgDC)
	w.Write(putInt(scratch, dc.Run))
	writeSlice(w, scratch, len(dc.DCNet), func(i int) {
		w.Write(putInt(scratch, dc.DCNet[i].N))
		w.Write(putInt(scratch, dc.DCNet[i].Msize))
		w.Write(dc.DCNet[i].Data)
	})
	var rs byte
	if dc.RevealSecrets {
		rs = 1
	}
	scratch[0] = rs
	w.Write(scratch[:1])
}

func (dc *DC) VerifySignature(pub ed25519.PublicKey) bool {
	return verify(pub, dc, dc.Signature)
}

// DCNet creates a message containing the previously-committed DC-mix vector and
// the shared keys of peers we have chosen to exclude.
func DCNet(dcs []*dcnet.Vec, s *Session) *DC {
	dc := &DC{
		Run:   s.run,
		DCNet: dcs,
	}
	dc.Signature = sign(s.sk, dc)
	return dc
}

// CM is the confirmed mix message.
type CM struct {
	Mix           BinaryRepresentable
	RevealSecrets bool
	BR            // Indicates to begin new run after peer exclusion
	Err           ServerError
	Signature     []byte
}

func (cm *CM) writeSigned(w io.Writer) {
	w.Write(msgCM)
	// Only the RevealSecrets field must be signed by clients, as Mix
	// already contains signatures, and RevealSecrets is the only other data
	// sent by clients in this message.
	var rs byte
	if cm.RevealSecrets {
		rs = 1
	}
	w.Write([]byte{rs})
}

func (cm *CM) VerifySignature(pub ed25519.PublicKey) bool {
	return verify(pub, cm, cm.Signature)
}

func (cm *CM) ServerError() error {
	if cm.Err == 0 {
		return nil
	}
	return cm.Err
}

// ConfirmedMix creates the confirmed mix message, sending either the confirmed
// mix or indication of a confirmation failure to the server.
func ConfirmMix(sk ed25519.PrivateKey, mix BinaryRepresentable) *CM {
	cm := &CM{Mix: mix}
	cm.Signature = sign(sk, cm)
	return cm
}

// RS is the reveal secrets message.  It reveals a run's PRNG seed, SR
// and DC secrets at the end of a failed run for blame assignment and
// misbehaving peer removal.
type RS struct {
	Seed []byte
	SR   []*big.Int
	M    [][]byte
}

// RevealSecrets creates the reveal secrets message.
func RevealSecrets(prngSeed []byte, sr []*big.Int, m [][]byte, s *Session) *RS {
	rs := &RS{
		Seed: prngSeed,
		SR:   sr,
		M:    m,
	}

	return rs
}

// Commit commits to the contents of the reveal secrets message.
func (rs *RS) Commit(ses *Session) []byte {
	scratch := make([]byte, 8)
	h := blake256.New()
	h.Write(msgCommit)
	h.Write(ses.sid)
	binary.LittleEndian.PutUint32(scratch, uint32(ses.run))
	h.Write(scratch)
	writeSignedByteSlice(h, scratch, rs.Seed)
	writeSlice(h, scratch, len(rs.SR), func(j int) {
		writeSignedBigInt(h, scratch, rs.SR[j])
	})
	writeSlice(h, scratch, len(rs.M), func(j int) {
		writeSignedByteSlice(h, scratch, rs.M[j])
	})
	return h.Sum(nil)
}
