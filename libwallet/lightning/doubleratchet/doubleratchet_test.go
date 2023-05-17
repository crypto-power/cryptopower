package doubleratchet

import (
	"testing"
	"time"

	"github.com/status-im/doubleratchet"
)

var defaultExpiry = uint64(time.Now().Add(time.Hour).Unix())

func TestSessionStore(t *testing.T) {
	if err := openDB("testDB"); err != nil {
		t.Error(err)
	}
	defer destroyDB()
	// sessionID, secret, err := NewSession()
	var rootChainKey [32]byte
	state := doubleratchet.DefaultState(rootChainKey)
	state.KeysCount = 10
	state.Step = 11
	state.PN = 12
	state.DHr = [32]byte{1, 2, 3}
	state.DHs = DHPair{privateKey: [32]byte{1, 1, 1}, publicKey: [32]byte{2, 2, 2}}
	state.RecvCh.N = 5
	state.RecvCh.CK = [32]byte{5, 5, 5}
	state.SendCh.N = 6
	state.SendCh.CK = [32]byte{6, 6, 6}

	store := &BoltDBSessionStorage{}
	if err := store.Save([]byte{1, 2, 3}, &state); err != nil {
		t.Error(err)
		return
	}
	loadedState, err := store.Load([]byte{1, 2, 3})
	if err != nil {
		t.Error(err)
		return
	}

	if loadedState.RootCh.CK != state.RootCh.CK {
		t.Errorf("loadedState.RootCh.CK != state.RootCh.CK %v, %v", loadedState.RootCh.CK, state.RootCh.CK)
	}
	if loadedState.KeysCount != state.KeysCount {
		t.Errorf("loadedState.KeysCount != state.KeysCount %v, %v", loadedState.KeysCount, state.KeysCount)
	}
	if loadedState.DHr != state.DHr {
		t.Errorf("loadedState.DHr != state.DHr %v, %v", loadedState.DHr, state.DHr)
	}
	if loadedState.HKr != state.HKr {
		t.Errorf("loadedState.HKr != state.HKr %v, %v", loadedState.HKr, state.HKr)
	}
	if loadedState.DHs.PrivateKey() != state.DHs.PrivateKey() {
		t.Errorf("loadedState.DHs.PrivateKey != state.DHs.PrivateKey %v, %v", loadedState.DHs.PrivateKey(), state.DHs.PrivateKey())
	}
	if loadedState.DHs.PublicKey() != state.DHs.PublicKey() {
		t.Errorf("loadedState.DHs.PublicKey != state.DHs.PublicKey %v, %v", loadedState.DHs.PublicKey(), state.DHs.PublicKey())
	}
	if loadedState.Step != state.Step {
		t.Errorf("loadedState.Step != state.Step %v, %v", loadedState.Step, state.Step)
	}
	if loadedState.PN != state.PN {
		t.Errorf("loadedState.PN != state.PN %v, %v", loadedState.PN, state.PN)
	}
	if loadedState.RecvCh.CK != state.RecvCh.CK {
		t.Errorf("loadedState.RecvCh.CK != state.RecvCh.CK %v, %v", loadedState.RecvCh.CK, state.RecvCh.CK)
	}
	if loadedState.RecvCh.N != state.RecvCh.N {
		t.Errorf("loadedState.RecvCh.N != state.RecvCh.N %v, %v", loadedState.RecvCh.N, state.RecvCh.N)
	}
	if loadedState.SendCh.CK != state.SendCh.CK {
		t.Errorf("loadedState.SendCh.CK != state.SendCh.CK %v, %v", loadedState.SendCh.CK, state.SendCh.CK)
	}
	if loadedState.SendCh.N != state.SendCh.N {
		t.Errorf("loadedState.SendCh.N != state.SendCh.N %v, %v", loadedState.SendCh.N, state.SendCh.N)
	}
}

func TestEncryptDecrypt(t *testing.T) {
	if err := openDB("testDB"); err != nil {
		t.Error(err)
	}
	defer destroyDB()
	initiatorID := "initiatorSession"
	secret, pubKey, err := NewSession(initiatorID, defaultExpiry)
	if err != nil {
		t.Error(err)
		return
	}
	receiverID := "receiverID"
	err = NewSessionWithRemoteKey(receiverID, secret, pubKey, defaultExpiry)
	if err != nil {
		t.Error(err)
		return
	}
	encrypted, err := RatchetEncrypt(initiatorID, "Hello from initiator")
	if err != nil {
		t.Error(err)
		return
	}

	decrypted, err := RatchetDecrypt(receiverID, encrypted)
	if err != nil {
		t.Error(err)
		return
	}
	if decrypted != "Hello from initiator" {
		t.Errorf("decrypted != Hello from initiator, value = %v", decrypted)
	}
}

func TestOutOfOrderMessages(t *testing.T) {
	if err := openDB("testDB"); err != nil {
		t.Error(err)
	}
	defer destroyDB()
	initiatorID := "initiatorSession"
	secret, pubKey, err := NewSession(initiatorID, defaultExpiry)
	if err != nil {
		t.Error(err)
		return
	}

	receiverID := "receiverID"
	err = NewSessionWithRemoteKey(receiverID, secret, pubKey, defaultExpiry)
	if err != nil {
		t.Error(err)
		return
	}
	encrypted, err := RatchetEncrypt(initiatorID, "Hello from initiator")
	if err != nil {
		t.Error(err)
		return
	}
	//fmt.Printf("encrypted = %v", encrypted)

	encrypted2, err := RatchetEncrypt(initiatorID, "Hello from initiator2")
	if err != nil {
		t.Error(err)
		return
	}

	decrypted2, err := RatchetDecrypt(receiverID, encrypted2)
	if err != nil {
		t.Error(err)
		return
	}
	if decrypted2 != "Hello from initiator2" {
		t.Errorf("decrypted != Hello from initiator2, value = %v", decrypted2)
	}

	decrypted, err := RatchetDecrypt(receiverID, encrypted)
	if err != nil {
		t.Error(err)
		return
	}
	if decrypted != "Hello from initiator" {
		t.Errorf("decrypted != Hello from initiator, value = %v", decrypted)
	}
}

func TestInitiatedSessions(t *testing.T) {
	if err := openDB("testDB"); err != nil {
		t.Error(err)
	}
	defer destroyDB()
	initiatorID := "initiatorSession"
	secret, pubKey, err := NewSession(initiatorID, defaultExpiry)
	if err != nil {
		t.Error(err)
		return
	}

	receiverID := "session"
	err = NewSessionWithRemoteKey(receiverID, secret, pubKey, defaultExpiry)
	if err != nil {
		t.Error(err)
		return
	}

	reply := RatchetSessionInfo(initiatorID)
	if reply.Initiated == false || reply.SessionID == "" {
		t.Errorf("initiated should be true")
	}

	reply = RatchetSessionInfo(receiverID)
	if reply.Initiated == true || reply.SessionID == "" {
		t.Errorf("initiated should be false")
	}
}

func TestSessionInfo(t *testing.T) {
	if err := openDB("testDB"); err != nil {
		t.Error(err)
	}
	defer destroyDB()
	initiatorID := "initiatorSession"
	secret, pubKey, err := NewSession(initiatorID, defaultExpiry)
	if err != nil {
		t.Error(err)
		return
	}

	receiverID := "session"
	err = NewSessionWithRemoteKey(receiverID, secret, pubKey, defaultExpiry)
	if err != nil {
		t.Error(err)
		return
	}

	if err = RatchetSessionSetInfo(initiatorID, "initiator user data"); err != nil {
		t.Error(err)
		return
	}

	if err = RatchetSessionSetInfo(receiverID, "receiver user data"); err != nil {
		t.Error(err)
		return
	}

	reply := RatchetSessionInfo(initiatorID)
	if reply.Initiated == false || reply.SessionID == "" {
		t.Errorf("initiated should be true")
	}

	if reply.UserInfo != "initiator user data" {
		t.Errorf("initiator user data is wrong!")
	}

	reply = RatchetSessionInfo(receiverID)
	if reply.Initiated == true || reply.SessionID == "" {
		t.Errorf("initiated should be false")
	}

	if reply.UserInfo != "receiver user data" {
		t.Errorf("receiver user data is wrong!")
	}
}

func TestExpiredSessions(t *testing.T) {
	if err := openDB("testDB"); err != nil {
		t.Error(err)
	}
	defer destroyDB()
	expiredTime := uint64(time.Now().Unix() - 10)
	initiatorID := "initiatorSession"
	secret, pubKey, err := NewSession(initiatorID, expiredTime)
	if err != nil {
		t.Error(err)
		return
	}

	receiverID := "session"
	err = NewSessionWithRemoteKey(receiverID, secret, pubKey, defaultExpiry)
	if err != nil {
		t.Error(err)
		return
	}

	deleteExpiredSessions()

	if err = RatchetSessionSetInfo(initiatorID, "initiator user data"); err == nil {
		t.Error(err) //this  session is expired therefore should have error
		return
	}

	if err = RatchetSessionSetInfo(receiverID, "receiver user data"); err != nil {
		t.Error(err)
		return
	}
}
