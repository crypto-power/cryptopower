// Package coinjoin defines a builder type for creating Decred CoinJoin transactions.
package coinjoin

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	mathrand "math/rand"
	"strings"
	"sync"
	"time"

	"github.com/decred/dcrd/wire"
	"golang.org/x/sync/errgroup"
)

var shuffleRand *mathrand.Rand
var shuffleMu sync.Mutex

func init() {
	buf := make([]byte, 8)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	seed := int64(binary.LittleEndian.Uint64(buf))
	shuffleRand = mathrand.New(mathrand.NewSource(seed))
}

func shuffle(n int, swap func(i, j int)) {
	shuffleMu.Lock()
	shuffleRand.Shuffle(n, swap)
	shuffleMu.Unlock()
}

// Caller performs a RPC method call.
type Caller interface {
	Call(ctx context.Context, method string, res interface{}, args ...interface{}) error
}

// Tx is a Decred CoinJoin transaction builder.  It is intended for usage by
// CoinJoin servers, not clients.
type Tx struct {
	Tx        wire.MsgTx
	c         Caller
	sc        ScriptClass
	inputPids []int
	mixValue  int64
	feeRate   int64
	txVersion uint16
	lockTime  uint32
	expiry    uint32
}

type ScriptClass int

// Recognized script classes
const (
	P2PKHv0 ScriptClass = iota
	P2SHv0
	numScriptClasses
)

func (sc ScriptClass) Match(script []byte, vers uint16) bool {
	// Only recognize v0 scripts currently
	if vers != 0 {
		return false
	}

	var match bool
	switch sc {
	case P2PKHv0:
		match = len(script) == 25
		match = match && script[0] == 0x76  // DUP
		match = match && script[1] == 0xa9  // HASH160
		match = match && script[2] == 20    // DATA20
		match = match && script[23] == 0x88 // EQUALVERIFY
		match = match && script[24] == 0xac // CHECKSIG
	case P2SHv0:
		match = len(script) == 23
		match = match && script[0] == 0xa9  // HASH160
		match = match && script[1] == 20    // DATA20
		match = match && script[22] == 0x87 // EQUAL
	}
	return match
}

func (sc ScriptClass) script(message []byte) []byte {
	switch sc {
	case P2PKHv0:
		s := []byte{
			0:  0x76, // DUP
			1:  0xa9, // HASH160
			2:  20,   // DATA20
			23: 0x88, // EQUALVERIFY
			24: 0xac, // CHECKSIG
		}
		copy(s[3:23], message)
		return s
	case P2SHv0:
		s := []byte{
			0:  0xa9, // HASH160
			1:  20,   // DATA20
			22: 0x87, // EQUALVERIFY
		}
		copy(s[2:22], message)
		return s
	default:
		panic("unreachable")
	}
}

func (sc ScriptClass) scriptSize() int {
	switch sc {
	case P2PKHv0:
		return 25
	case P2SHv0:
		return 23
	default:
		panic("unreachable")
	}
}

func (sc ScriptClass) version() uint16 { return 0 }

const descname = "coinjoin-decred-v2"

// EncodeDesc encodes a description defining the parameters for a Decred coinjoin.
func EncodeDesc(sc ScriptClass, amount int64, txVersion uint16, lockTime, expiry uint32) []byte {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	encode := func(value interface{}) {
		if err := enc.Encode(value); err != nil {
			panic(err)
		}
	}
	encode(descname)
	encode(int(sc))
	encode(amount)
	encode(txVersion)
	encode(lockTime)
	encode(expiry)
	return buf.Bytes()
}

// DecodeDesc decodes a description defining the parameters for a compatible coinjoin.
func DecodeDesc(desc []byte) (sc ScriptClass, amount int64, txVersion uint16, lockTime, expiry uint32, err error) {
	buf := bytes.NewBuffer(desc)
	dec := gob.NewDecoder(buf)
	decode := func(out interface{}) {
		if err == nil {
			err = dec.Decode(out)
		}
	}
	var ty string
	decode(&ty)
	if ty != descname {
		err = errors.New("incompatible coinjoin")
		return
	}
	decode(&sc)
	decode(&amount)
	decode(&txVersion)
	decode(&lockTime)
	decode(&expiry)
	return
}

func NewTx(caller Caller, sc ScriptClass, amount, feeRate int64, txVersion uint16,
	lockTime, expiry uint32) (*Tx, error) {
	if sc >= numScriptClasses {
		return nil, errors.New("unknown script class")
	}
	tx := &Tx{
		Tx:        wire.MsgTx{Version: txVersion},
		c:         caller,
		sc:        sc,
		mixValue:  amount,
		feeRate:   feeRate,
		txVersion: txVersion,
		lockTime:  lockTime,
		expiry:    expiry,
	}
	return tx, nil
}

// MarshalBinary marshals the transaction in wire encoding.
func (t *Tx) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.Grow(t.Tx.SerializeSize())
	err := t.Tx.Serialize(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary unmarshals the transaction in wire encoding.
func (t *Tx) UnmarshalBinary(b []byte) error {
	return t.Tx.Deserialize(bytes.NewReader(b))
}

func feeForSerializeSize(relayFeePerKb int64, txSerializeSize int) int64 {
	fee := relayFeePerKb * int64(txSerializeSize) / 1000

	if fee == 0 && relayFeePerKb > 0 {
		fee = relayFeePerKb
	}

	const maxAmount = 21e6 * 1e8
	if fee < 0 || fee > maxAmount {
		fee = maxAmount
	}

	return fee
}

func (t *Tx) ValidateUnmixed(unmixed []byte, mcount int) error {
	var fee int64
	other := new(wire.MsgTx)
	err := other.Deserialize(bytes.NewReader(unmixed))
	if err != nil {
		return err
	}
	if other.Version != t.txVersion {
		return errors.New("coinjoin: different tx versions")
	}
	for _, out := range other.TxOut {
		if !t.sc.Match(out.PkScript, out.Version) {
			return errors.New("coinjoin: different script class")
		}
		fee -= out.Value
	}
	var g errgroup.Group
	for i := range other.TxIn {
		in := other.TxIn[i]
		g.Go(func() error {
			return verifyOutput(t.c, &in.PreviousOutPoint, in.ValueIn)
		})
		fee += in.ValueIn
	}
	err = g.Wait()
	if err != nil {
		var e *blameError
		if errors.As(err, &e) {
			return errors.New(e.s)
		}
		return err
	}
	fee -= int64(mcount) * t.mixValue
	bogusMixedOut := &wire.TxOut{
		Value:    t.mixValue,
		Version:  t.sc.version(),
		PkScript: make([]byte, t.sc.scriptSize()),
	}
	for i := 0; i < mcount; i++ {
		other.AddTxOut(bogusMixedOut)
	}
	requiredFee := feeForSerializeSize(t.feeRate, other.SerializeSize())
	if fee < requiredFee {
		return errors.New("coinjoin: unmixed transaction does not pay enough network fees")
	}
	return nil
}

// Join adds the inputs and outputs of a transaction in wire encoding to t.
// Inputs and outputs are associated with a peer id for blame assignment.
func (t *Tx) Join(unmixed []byte, pid int) error {
	other := new(wire.MsgTx)
	err := other.Deserialize(bytes.NewReader(unmixed))
	if err != nil {
		return err
	}
	if other.Version != t.txVersion {
		return errors.New("coinjoin: different tx versions")
	}
	for _, out := range other.TxOut {
		if !t.sc.Match(out.PkScript, out.Version) {
			return errors.New("coinjoin: different script class")
		}
	}
	tx := &t.Tx
	for _, in := range other.TxIn {
		t.inputPids = append(t.inputPids, pid)
		tx.TxIn = append(tx.TxIn, in)
	}
	tx.TxOut = append(tx.TxOut, other.TxOut...)
	return nil
}

const rpcTimeout = 10 * time.Second

func verifyOutput(c Caller, outpoint *wire.OutPoint, value int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	var res struct {
		Value float64 `json:"value"`
	}
	err := c.Call(ctx, "gettxout", &res, outpoint.Hash.String(), outpoint.Index, outpoint.Tree)
	if err != nil {
		return err
	}
	if v := atoms(res.Value); v != value {
		return &blameError{
			fmt.Sprintf("output %v has wrong value %d (expected %d)", outpoint, value, v),
			nil,
		}
	}
	return nil
}

func atoms(f float64) int64 {
	return int64(f*1e8 + 0.5)
}

// Mix adds an output with m as the PkScript.  The output will use the builder's
// output value and script version.
func (t *Tx) Mix(m []byte) {
	t.Tx.TxOut = append(t.Tx.TxOut, &wire.TxOut{
		Value:    t.mixValue,
		Version:  t.sc.version(),
		PkScript: t.sc.script(m),
	})
}

// Confirm extracts the signatures from peer pid in confirm and adds them the
// receiver.
//
// confirm must have type *Tx.
func (t *Tx) Confirm(confirm interface{}, pid int) error {
	t2, ok := confirm.(*Tx)
	if !ok {
		return errors.New("coinjoin: confirmation is not a *Tx")
	}
	if len(t.Tx.TxIn) != len(t2.Tx.TxIn) {
		return errors.New("coinjoin: peer added or removed inputs")
	}
	for i, inputPid := range t.inputPids {
		if pid != inputPid {
			continue
		}
		t.Tx.TxIn[i].SignatureScript = t2.Tx.TxIn[i].SignatureScript
	}
	return nil
}

// Shuffle randomly shuffles the transaction inputs and outputs.
// Randomness is obtained from a cryptographically-seeded PRNG.
// Must only be used before signing.
func (t *Tx) Shuffle() {
	tx := &t.Tx
	shuffle(len(tx.TxIn), func(i, j int) {
		t.inputPids[i], t.inputPids[j] = t.inputPids[j], t.inputPids[i]
		tx.TxIn[i], tx.TxIn[j] = tx.TxIn[j], tx.TxIn[i]
	})
	shuffle(len(tx.TxOut), func(i, j int) {
		tx.TxOut[i], tx.TxOut[j] = tx.TxOut[j], tx.TxOut[i]
	})
}

// PublishMix publishes a transaction using the dcrd sendrawtransaction RPC.
func (t *Tx) PublishMix(ctx context.Context) error {
	b := new(strings.Builder)
	err := t.Tx.Serialize(hex.NewEncoder(b))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	err = t.c.Call(ctx, "sendrawtransaction", nil, b.String())
	if err != nil {
		// TODO: try to blame peers of the bad inputs/outputs
		return err
	}
	return nil
}

type report struct {
	TxHash       string
	Denomination int64
	TotalInput   int64
	Fee          int64
}

// Report returns an object that can be marshaled with reflection-based encoders
// such as encoding/json and encoding/gob.  The object includes details about
// the current CoinJoin transaction.
func (t *Tx) Report() interface{} {
	r := &report{
		TxHash:       t.Tx.TxHash().String(),
		Denomination: t.mixValue,
	}
	var input, output int64
	for _, in := range t.Tx.TxIn {
		input += in.ValueIn
	}
	for _, out := range t.Tx.TxOut {
		output += out.Value
	}
	r.TotalInput = input
	r.Fee = input - output
	return r
}

// Blamer describes misbehaving peer IDs.
type Blamer interface {
	Blame() []int
}

type blameError struct {
	s string
	b []int
}

func (e *blameError) Error() string { return e.s }
func (e *blameError) Blame() []int  { return e.b }
