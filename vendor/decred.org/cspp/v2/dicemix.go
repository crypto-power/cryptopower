// Package cspp implements a DiceMix Light and CoinShuffle++ client.
package cspp

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"math/big"
	"math/bits"
	"net"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"decred.org/cspp/v2/chacha20prng"
	"decred.org/cspp/v2/dcnet"
	"decred.org/cspp/v2/messages"
	"decred.org/cspp/v2/x25519"
	"github.com/companyzero/sntrup4591761"
	"golang.org/x/crypto/ed25519"
)

// MessageSize is the size of messages returned by Gen.
const MessageSize = 20

const (
	sendTimeout = 5 * time.Second
	recvTimeout = 20 * time.Second
)

// GenConfirmer is a generator of fresh messages to mix in a DiceMix run and a
// signable message that can include the mixed messages.  This will often be a
// transaction in wire encoding when CoinShuffle++ is used by applying DiceMix
// to create a CoinJoin transaction.  Inputs spent by the CoinJoin must set the
// input amount.  A single output for change is allowed, and if included, must
// pay the required share of fee.
//
// The CoinShuffle++ server must combine inputs from all peers at the start of
// the protocol and share the combined message with each peer.  Conforming
// implementations must return errors from Confirm if any of their inputs or
// anonymous outputs are missing after unmarshaling.  See MissingMessage for
// documentation on how to return errors.
type GenConfirmer interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
	Gen() ([][]byte, error)
	Confirm() error
}

// MissingMessage describes a UnmarshalBinary or Confirm error where an
// anonymized message is missing from the mix.  It results in a failed run,
// revealing secrets, and assigning blame.  If the mix is instead missing
// non-anonymized messages, this error should not be used, and any other error
// will cause the client to abort the protocol due to the server being
// malicious.
type MissingMessage interface {
	error
	MissingMessage()
}

// Session represents a DiceMix Light session (which consists of one or more
// runs to remove unresponsive or malicious peers).
type Session struct {
	Pk ed25519.PublicKey  // Session pubkey
	Sk ed25519.PrivateKey // Session signing key

	rand     io.Reader
	genConf  GenConfirmer
	mcount   int
	freshGen bool // Whether next run must generate fresh KX keys, SR/DC messages

	kx       *dcnet.KX
	prngSeed []byte
	prng     *chacha20prng.Reader

	srMsg []*big.Int // random numbers to be exponential slot reservation mix
	dcMsg [][]byte   // anonymized messages to publish

	client *client
	sid    []byte

	log        Logger
	commitment []byte
}

type run struct {
	session *Session
	run     int

	// Peer information
	vk      []ed25519.PublicKey // session pubkeys
	mcounts []int
	my      int // this client's non-anonymous index
	mtot    int

	// Exponential slot reservation mix
	srKP  [][][]byte // shared keys for exp dc-net
	srMix [][]*big.Int

	// XOR DC-net
	dcKP  [][]*dcnet.Vec
	dcNet []*dcnet.Vec
}

type client struct {
	conn net.Conn
	dec  *gob.Decoder
	enc  *gob.Encoder
}

func newClient(conn net.Conn) *client {
	return &client{
		conn: conn,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(conn),
	}
}

func (c *client) send(msg interface{}, timeout time.Duration) (err error) {
	defer func() {
		if err != nil {
			_, file, line, _ := runtime.Caller(2)
			file = filepath.Base(file)
			err = fmt.Errorf("%v: send %T (%v:%v): %v", c.conn.LocalAddr(), msg, file, line, err)
		}
	}()
	if err = c.conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	return c.enc.Encode(msg)
}

func (c *client) recv(out interface{}, timeout time.Duration) error {
	var deadline time.Time
	if timeout != 0 {
		deadline = time.Now().Add(timeout)
	}
	if err := c.conn.SetReadDeadline(deadline); err != nil {
		return err
	}
	err := c.dec.Decode(out)
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		file = filepath.Base(file)
		return fmt.Errorf("%v: read %T (%v:%v): %v", c.conn.LocalAddr(), out, file, line, err)
	}
	// Return server error if message carries an error code
	if se, ok := out.(interface{ ServerError() error }); ok {
		if err := se.ServerError(); err != nil {
			return err
		}
	}
	return nil
}

// Logger writes client logs.
type Logger interface {
	Print(args ...interface{})
	Printf(format string, args ...interface{})
}

// NewSession creates a new Session for a mix session described by
// pairCommitment, which will submit mixes number of mixed messages.
func NewSession(random io.Reader, log Logger, pairCommitment []byte, mixes int) (*Session, error) {
	pk, sk, err := ed25519.GenerateKey(random)
	if err != nil {
		return nil, err
	}
	ses := &Session{
		Pk:         pk,
		Sk:         sk,
		rand:       random,
		mcount:     mixes,
		log:        log,
		commitment: pairCommitment,
	}
	return ses, nil
}

// DiceMix performs the DiceMix Light client protocol with a server connected
// through conn.
func (s *Session) DiceMix(ctx context.Context, conn net.Conn, genConf GenConfirmer) error {
	defer conn.Close()
	s.client = newClient(conn)
	s.genConf = genConf

	unmixed, err := s.genConf.MarshalBinary()
	if err != nil {
		return err
	}
	pr := messages.PairRequest(s.Pk, s.Sk, s.commitment, unmixed, s.mcount)
	err = s.client.send(pr, sendTimeout)
	if err != nil {
		s.log.Print(err)
		return err
	}

	var br *messages.BR
	for i := 0; ; i++ {
		err := s.run(ctx, i, br)
		br = nil
		if err != nil {
			var e beginRunner
			if errors.As(err, &e) {
				br = e.BR()
				s.log.Printf("rerunning")
				continue
			}
			if errors.Is(err, errRerun) {
				s.log.Printf("rerunning")
				continue
			}
			return err
		}
		return nil
	}
}

func (s *Session) run(ctx context.Context, n int, br *messages.BR) error {
	if br == nil {
		br = new(messages.BR)
		readBR := make(chan error, 1)
		var brTimeout time.Duration
		if n != 0 {
			brTimeout = recvTimeout
		}
		go func() { readBR <- s.client.recv(br, brTimeout) }()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-readBR:
			if err != nil {
				s.log.Print(err)
				return err
			}
		}
		if len(br.Vk) != len(br.MessageCounts) {
			return errors.New("inequivalent key and mix counts")
		}
	}

	vk := br.Vk
	sid := br.Sid
	var totalMessages int
	myVk := -1
	myStart := -1
	mcounts := make([]int, len(vk))
	for i := range vk {
		if bytes.Equal(vk[i], s.Pk) {
			myVk = i
			myStart = totalMessages
			if br.MessageCounts[i] != s.mcount {
				return errors.New("incorrect self message count")
			}
		}
		if br.MessageCounts[i] <= 0 {
			return errors.New("non-positive message count")
		}
		totalMessages += br.MessageCounts[i]
		mcounts[i] = br.MessageCounts[i]
	}
	if myVk == -1 {
		return errors.New("my index is not in vk slice")
	}
	s.sid = sid

	ses := messages.NewSession(s.sid, n, s.Sk, vk)

	r := &run{
		session: s,
		run:     n,
		mtot:    totalMessages,
		mcounts: mcounts,
		vk:      vk,
		my:      myVk,
	}
	s.prngSeed = make([]byte, 32)
	_, err := io.ReadFull(s.rand, s.prngSeed)
	if err != nil {
		return err
	}
	s.prng = chacha20prng.New(s.prngSeed, uint32(r.run))
	if n == 0 || s.freshGen {
		s.freshGen = false

		// Generate fresh x25519, sntrup4591761 keys from this run's PRNG
		var err error
		s.kx, err = dcnet.NewKX(s.prng)
		if err != nil {
			return err
		}

		// Generate fresh SR messages
		s.srMsg = make([]*big.Int, s.mcount)
		for i := range s.srMsg {
			s.srMsg[i], err = rand.Int(s.rand, dcnet.F)
			if err != nil {
				return err
			}
		}

		// Generate fresh DC messages
		s.dcMsg, err = s.genConf.Gen()
		if err != nil {
			return err
		}
		if len(s.dcMsg) != s.mcount {
			return errors.New("cspp: Gen returned wrong message count")
		}
		for _, m := range s.dcMsg {
			if len(m) != MessageSize {
				return errors.New("cspp: Gen returned bad message length")
			}
		}
	}
	s.log.Printf("SR msg: %x; DC msg: %x", s.srMsg, s.dcMsg)

	// Perform key exchange
	rs := messages.RevealSecrets(s.prngSeed, s.srMsg, s.dcMsg, ses)
	ke := messages.KeyExchange(s.kx, rs.Commit(ses), ses)
	err = s.client.send(ke, sendTimeout)
	if err != nil {
		return err
	}
	var kes messages.KEs
	err = s.client.recv(&kes, recvTimeout)
	if err != nil {
		s.log.Print(err)
		return err
	}
	if len(kes.BR.Vk) != 0 {
		return &beginRerun{&kes.BR}
	}
	s.log.Printf("received KEs")
	ecdh := make([]*x25519.Public, 0, len(r.vk))
	pqpk := make([]*[sntrup4591761.PublicKeySize]byte, 0, len(r.vk))
	for _, ke := range kes.KEs {
		if ke == nil {
			continue
		}
		ecdh = append(ecdh, ke.ECDH)
		pqpk = append(pqpk, ke.PQPK)
	}
	if len(ecdh) != len(r.vk) {
		return errors.New("wrong total ECDH public count")
	}
	if len(pqpk) != len(r.vk) {
		return errors.New("wrong total PQ public key count")
	}

	// Create shared key and ciphertexts for each peer
	pqct, err := s.kx.Encapsulate(s.prng, pqpk, r.my)
	if err != nil {
		return err
	}

	// Send and receive ciphertext messages.
	ct := messages.Ciphertexts(pqct, ses)
	err = s.client.send(ct, sendTimeout)
	if err != nil {
		return err
	}
	var cts messages.CTs
	err = s.client.recv(&cts, recvTimeout)
	if err != nil {
		return err
	}
	if len(cts.BR.Vk) != 0 {
		return &beginRerun{&cts.BR}
	}

	// Derive shared secret keys
	r.srKP, r.dcKP, err = dcnet.SharedKeys(s.kx, ecdh, cts.Ciphertexts, s.sid,
		MessageSize, r.run, r.my, r.mcounts)
	if err != nil {
		return err
	}

	// Calculate slot reservation DC-net vectors
	r.srMix = make([][]*big.Int, s.mcount)
	for i := 0; i < s.mcount; i++ {
		pads := dcnet.SRMixPads(r.srKP[i], myStart+i)
		r.srMix[i] = dcnet.SRMix(s.srMsg[i], pads)
	}

	// Broadcast message commitment and exponential DC-mix vectors for slot
	// reservations.
	sr := messages.SlotReserve(r.srMix, ses)
	err = s.client.send(sr, sendTimeout)
	if err != nil {
		return err
	}
	var rm messages.RM
	err = s.client.recv(&rm, recvTimeout)
	if err != nil {
		return err
	}
	if len(rm.BR.Vk) != 0 {
		return &beginRerun{&rm.BR}
	}
	if rm.RevealSecrets {
		return s.serverRunFail(ctx, rs)
	}

	// Find reserved slots.
	slots := make([]int, 0, s.mcount)
	sort.Slice(rm.Roots, func(i, j int) bool {
		return rm.Roots[i].Cmp(rm.Roots[j]) < 0
	})
	for _, root := range rm.Roots {
		if !dcnet.InField(root) {
			return errors.New("solved root is not in field")
		}
	}
	for _, m := range s.srMsg {
		slot := constTimeSlotSearch(m, rm.Roots)
		if slot == -1 {
			s.log.Printf("run failed: didn't find all slots")
			return s.clientRunFail(ctx, rs)
		}
		slots = append(slots, slot)
	}
	s.log.Printf("reserved slots %v", slots)

	r.dcNet = make([]*dcnet.Vec, s.mcount)
	for i, slot := range slots {
		my := myStart + i
		pads := dcnet.DCMixPads(r.dcKP[i], MessageSize, my)
		r.dcNet[i] = dcnet.DCMix(pads, s.dcMsg[i], slot)
	}

	// Broadcast and wait for exponential DC-net vectors.
	dc := messages.DCNet(r.dcNet, ses)
	err = s.client.send(dc, sendTimeout)
	if err != nil {
		return err
	}

	// Receive mix and confirm.  Confirm must check that our mixed messages
	// are present.
	cm := &messages.CM{Mix: s.genConf}
	err = s.client.recv(cm, recvTimeout)
	if err == nil {
		if len(cm.BR.Vk) != 0 {
			return &beginRerun{&cm.BR}
		}
		if cm.RevealSecrets {
			return s.serverRunFail(ctx, rs)
		}
		err = s.genConf.Confirm()
	}
	if err != nil {
		var errMM MissingMessage
		if errors.As(err, &errMM) {
			s.log.Printf("missing message: %v", err)
			return s.clientRunFail(ctx, rs)
		}
		return err
	}
	cm = messages.ConfirmMix(s.Sk, s.genConf)
	err = s.client.send(cm, sendTimeout)
	if err != nil {
		return err
	}

	// Receive mix with merged confirmations
	cm = &messages.CM{Mix: s.genConf}
	err = s.client.recv(cm, recvTimeout)
	if err != nil {
		return err
	}
	if len(cm.BR.Vk) != 0 {
		return &beginRerun{&cm.BR}
	}
	if cm.RevealSecrets {
		return s.serverRunFail(ctx, rs)
	}

	s.log.Printf("got confirmed message")
	return nil
}

type beginRunner interface {
	BR() *messages.BR
}

type beginRerun struct {
	br *messages.BR
}

func (b *beginRerun) BR() *messages.BR { return b.br }
func (b *beginRerun) Error() string    { return "begin rerun" }

var errRerun = errors.New("rerun")
var runFailed = &struct{ RevealSecrets bool }{true}

func (s *Session) serverRunFail(ctx context.Context, rs *messages.RS) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	s.freshGen = true

	// Reveal secrets
	s.log.Print("server indicated run failure; revealing run secrets")
	err := s.client.send(rs, sendTimeout)
	if err != nil {
		return err
	}
	return errRerun
}

// XXX must send the proper message type, with valid signature for that type
func (s *Session) clientRunFail(ctx context.Context, rs *messages.RS) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	s.freshGen = true

	// Inform server of client-detected failure
	err := s.client.send(runFailed, sendTimeout)
	if err != nil {
		return err
	}

	// Reveal secrets
	s.log.Print("revealing run secrets")
	err = s.client.send(rs, sendTimeout)
	if err != nil {
		return err
	}
	return errRerun
}

var fieldLen = uint(len(dcnet.F.Bytes()))

// constTimeSlotSearch searches for the index of secret in roots in constant time.
// Returns -1 if the secret is not found.
func constTimeSlotSearch(secret *big.Int, roots []*big.Int) int {
	paddedSecret := make([]byte, fieldLen)
	secretBytes := secret.Bytes()
	off, _ := bits.Sub(fieldLen, uint(len(secretBytes)), 0)
	copy(paddedSecret[off:], secretBytes)

	slot := -1
	buf := make([]byte, fieldLen)
	for i := range roots {
		rootBytes := roots[i].Bytes()
		off, _ := bits.Sub(fieldLen, uint(len(rootBytes)), 0)
		copy(buf[off:], rootBytes)
		cmp := subtle.ConstantTimeCompare(paddedSecret, buf)
		slot = subtle.ConstantTimeSelect(cmp, i, slot)
		for j := range buf {
			buf[j] = 0
		}
	}
	return slot
}
