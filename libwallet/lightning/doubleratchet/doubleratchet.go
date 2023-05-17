package doubleratchet

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/status-im/doubleratchet"
)

//RatchetSessionDetails represents the info of existing session
type RatchetSessionDetails struct {
	SessionID string
	Initiated bool
	UserInfo  string
}

var (
	byteOrder = binary.BigEndian
	mu        sync.Mutex
)

//Start starts the doubleratchet service and makes it ready for action
func Start(dbpath string) error {
	err := openDB(dbpath)
	if err != nil {
		return err
	}
	return deleteExpiredSessions()
}

//Stop stops the doubleratchet service and release not needed resources
func Stop() error {
	if db == nil {
		return nil
	}
	return closeDB()
}

/*
NewSession is used by the initiator of the encrypted session.
This function takes no paramters and returns:

sessionID: A unique string that identified the session for later use
secret:    A shared secret to be shared with the other side

This function creates a session and stores its state in the SessionStore
Following this operation the caller can imediately use Encrypt/Decrypt by
providing the sessionID as identifier.
*/
func NewSession(sessionID string, expiry uint64) (secret, pubKey string, err error) {
	crypto := doubleratchet.DefaultCrypto{}

	var sk [32]byte
	_, err = rand.Read(sk[:])
	if err != nil {
		return
	}

	if err = createSessionContext([]byte(sessionID), true, expiry); err != nil {
		return
	}

	keyPair, err := crypto.GenerateDH()
	if err != nil {
		return
	}
	_, err = doubleratchet.New([]byte(sessionID), sk, keyPair, &BoltDBSessionStorage{})
	if err != nil {
		return
	}

	publicKey := keyPair.PublicKey()
	return hex.EncodeToString(sk[:]), hex.EncodeToString(publicKey[:]), nil
}

/*
NewSessionWithRemoteKey is used by the recepient side  of the encrypted session.
This function takes the following parameters:

secret:    		Shared secret that was agreed with the initiator side
remotePubKey 	The initiator side public key.

This function creates a session and stores its state in the SessionStore
Following this operation the caller can imediately use Encrypt/Decrypt by
providing the sessionID as identifier.
*/
func NewSessionWithRemoteKey(sessionID, secret, remotePubKey string, expiry uint64) error {

	skBytes, err := hex.DecodeString(secret)
	if err != nil {
		return err
	}

	pubKeyBytes, err := hex.DecodeString(remotePubKey)
	if err != nil {
		return err
	}

	if err = createSessionContext([]byte(sessionID), false, expiry); err != nil {
		return err
	}

	_, err = doubleratchet.NewWithRemoteKey(
		[]byte(sessionID),
		toKey(skBytes),
		toKey(pubKeyBytes),
		&BoltDBSessionStorage{},
		doubleratchet.WithKeysStorage(&BoltDBKeysStorage{}))
	if err != nil {
		return err
	}

	return nil
}

/*
RatchetSessionInfo checks if a session matches the sessionID and returns its details.
*/
func RatchetSessionInfo(sessionID string) *RatchetSessionDetails {
	session, err := doubleratchet.Load(
		[]byte(sessionID),
		&BoltDBSessionStorage{},
		doubleratchet.WithKeysStorage(&BoltDBKeysStorage{}))
	if err != nil || session == nil {
		fmt.Printf("session %v is nil", sessionID)
		return nil
	}
	fmt.Printf("session %v is NOT nil", sessionID)
	sessionInfo := fetchSessionInfo([]byte(sessionID))
	initiated, _ := fetchSessionContext([]byte(sessionID))
	return &RatchetSessionDetails{
		SessionID: sessionID,
		UserInfo:  string(sessionInfo),
		Initiated: initiated,
	}
}

/*
RatchetSessionSetInfo checks if a session matches the sessionID and set its details.
*/
func RatchetSessionSetInfo(sessionID, info string) error {
	session, err := doubleratchet.Load(
		[]byte(sessionID),
		&BoltDBSessionStorage{},
		doubleratchet.WithKeysStorage(&BoltDBKeysStorage{}))
	if err != nil || session == nil {
		fmt.Printf("session %v is nil", sessionID)
		return fmt.Errorf("Session %v does not exist", sessionID)
	}

	return setSessionInfo([]byte(sessionID), []byte(info))
}

/*
RatchetEncrypt is used to encrypt a message providing a sessionID and a message to encrypt
This function loads the session from the session store and use it to encrypt the message.
This way the user is free of managing sessions state and only need to provide the sessionID.
*/
func RatchetEncrypt(sessionID, message string) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	session, err := doubleratchet.Load(
		[]byte(sessionID),
		&BoltDBSessionStorage{},
		doubleratchet.WithKeysStorage(&BoltDBKeysStorage{}))
	if err != nil {
		return "", err
	}
	msg, err := session.RatchetEncrypt([]byte(message), nil)
	if err != nil {
		return "", err
	}

	rawMessage, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}
	return string(rawMessage), nil
}

/*
RatchetDecrypt is used to decrypt a message providing a sessionID and a message to decrypt
This function loads the session from the session store and use it to decrypt the message.
This way the user is free of managing sessions state and only need to provide the sessionID.
*/
func RatchetDecrypt(sessionID, message string) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	var msg doubleratchet.Message
	if err := json.Unmarshal([]byte(message), &msg); err != nil {
		return "", err
	}

	session, err := doubleratchet.Load(
		[]byte(sessionID),
		&BoltDBSessionStorage{},
		doubleratchet.WithKeysStorage(&BoltDBKeysStorage{}))
	if err != nil {
		return "", err
	}
	decrypted, err := session.RatchetDecrypt(msg, nil)
	if err != nil {
		return "", err
	}
	return string(decrypted), nil
}

//DHPair is a key pair structure that implements the doubleratchet.DHPair interface
type DHPair struct {
	privateKey [32]byte
	publicKey  [32]byte
}

//PrivateKey is part of the doubleratchets.DHPair interaface
func (p DHPair) PrivateKey() doubleratchet.Key {
	return p.privateKey
}

//PublicKey is part of the doubleratchets.DHPair interaface
func (p DHPair) PublicKey() doubleratchet.Key {
	return p.publicKey
}

//BoltDBSessionStorage is a structure that implements the SessionStore interface.
//It uses boltdb to save sessions
type BoltDBSessionStorage struct{}

// Save the session to the session store
func (s *BoltDBSessionStorage) Save(id []byte, state *doubleratchet.State) error {

	var buf bytes.Buffer

	if err := binary.Write(&buf, byteOrder, state.RootCh.CK[:]); err != nil {
		return err
	}
	if _, err := buf.Write(state.DHr[:]); err != nil {
		return err
	}

	pubKey := state.DHs.PublicKey()
	if _, err := buf.Write(pubKey[:]); err != nil {
		return err
	}

	privateKey := state.DHs.PrivateKey()
	if _, err := buf.Write(privateKey[:]); err != nil {
		return err
	}

	if err := binary.Write(&buf, byteOrder, state.PN); err != nil {
		return err
	}
	if err := binary.Write(&buf, byteOrder, uint32(state.Step)); err != nil {
		return err
	}
	if err := binary.Write(&buf, byteOrder, uint32(state.KeysCount)); err != nil {
		return err
	}

	//send chain
	if _, err := buf.Write(state.SendCh.CK[:]); err != nil {
		return err
	}
	if err := binary.Write(&buf, byteOrder, state.SendCh.N); err != nil {
		return err
	}

	//receive chain
	if _, err := buf.Write(state.RecvCh.CK[:]); err != nil {
		return err
	}
	if err := binary.Write(&buf, byteOrder, state.RecvCh.N); err != nil {
		return err
	}

	return saveEncryptedSession(id, buf.Bytes())
}

// Load session by id
func (s *BoltDBSessionStorage) Load(id []byte) (*doubleratchet.State, error) {
	sessionData := fetchEncryptedSession(id)

	buf := bytes.NewBuffer(sessionData)
	rootChainKey, err := readKey(buf)
	if err != nil {
		return nil, err
	}

	state := doubleratchet.DefaultState(rootChainKey)
	if _, err = buf.Read(state.DHr[:]); err != nil {
		return nil, err
	}

	dhsPublic, err := readKey(buf)
	if err != nil {
		return nil, err
	}

	dhsPrivate, err := readKey(buf)
	if err != nil {
		return nil, err
	}

	state.DHs = DHPair{privateKey: dhsPrivate, publicKey: dhsPublic}
	if err = binary.Read(buf, byteOrder, &state.PN); err != nil {
		return nil, err
	}
	var uint32Val uint32

	if err = binary.Read(buf, byteOrder, &uint32Val); err != nil {
		return nil, err
	}
	state.Step = uint(uint32Val)

	if err = binary.Read(buf, byteOrder, &uint32Val); err != nil {
		return nil, err
	}
	state.KeysCount = uint(uint32Val)

	if _, err = buf.Read(state.SendCh.CK[:]); err != nil {
		return nil, err
	}
	if err = binary.Read(buf, byteOrder, &state.SendCh.N); err != nil {
		return nil, err
	}

	if _, err = buf.Read(state.RecvCh.CK[:]); err != nil {
		return nil, err
	}
	if err = binary.Read(buf, byteOrder, &state.RecvCh.N); err != nil {
		return nil, err
	}

	return &state, nil
}

//BoltDBKeysStorage is a structure that implements the KeysStorge interface.
//Keys are saved for skipped messages that may come later.
//It uses boltdb to save the keys.
type BoltDBKeysStorage struct{}

// Get returns a message key by the given key and message number.
func (*BoltDBKeysStorage) Get(k doubleratchet.Key, msgNum uint) (mk doubleratchet.Key, ok bool, err error) {
	key, err := fetchEncryptedMessageKey(k[:], msgNum)
	return toKey(key), key != nil, err
}

// Put saves the given mk under the specified key and msgNum.
func (*BoltDBKeysStorage) Put(sessionID []byte, k doubleratchet.Key, msgNum uint, mk doubleratchet.Key, keySeqNum uint) error {
	return saveEncryptedMessageKey(sessionID, k[:], msgNum, mk[:])
}

// DeleteMk ensures there's no message key under the specified key and msgNum.
func (*BoltDBKeysStorage) DeleteMk(k doubleratchet.Key, msgNum uint) error {
	return nil
}

// DeleteOldMks deletes old message keys for a session.
func (*BoltDBKeysStorage) DeleteOldMks(sessionID []byte, deleteUntilSeqKey uint) error {
	return nil
}

// TruncateMks truncates the number of keys to maxKeys.
//We have short live sessions so currently we don't implemented that
func (*BoltDBKeysStorage) TruncateMks(sessionID []byte, maxKeys int) error {
	return nil
}

// Count returns number of message keys stored under the specified key.
func (*BoltDBKeysStorage) Count(k doubleratchet.Key) (uint, error) {
	return countMessageKeys(k[:])
}

// All returns all the keys
func (*BoltDBKeysStorage) All() (map[doubleratchet.Key]map[uint]doubleratchet.Key, error) {
	all, err := allMessageKeys()
	if err != nil {
		return nil, err
	}

	var keysMap map[doubleratchet.Key]map[uint]doubleratchet.Key
	for k, v := range all {
		keysMap[k] = make(map[uint]doubleratchet.Key)
		for mk, mv := range v {
			keysMap[k][mk] = mv
		}
	}
	return keysMap, err
}

func readKey(buf *bytes.Buffer) (doubleratchet.Key, error) {
	var keyBuf [32]byte
	_, err := buf.Read(keyBuf[:])
	return keyBuf, err
}

func toKey(a []byte) doubleratchet.Key {
	var k [32]byte
	copy(k[:], a)
	return k
}

func generateSessionID() (string, error) {
	var sessionIDBytes [32]byte
	_, err := rand.Read(sessionIDBytes[:])
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sessionIDBytes[:]), nil
}
