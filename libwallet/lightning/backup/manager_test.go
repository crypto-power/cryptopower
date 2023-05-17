package backup

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/breez/breez/config"
	"github.com/breez/breez/data"
	"github.com/breez/breez/tor"
	"github.com/btcsuite/btclog"
)

type MockAuthService struct{}

func (s *MockAuthService) SignIn() (string, error) {
	return "test-token", nil
}

type MockTester struct {
	uploadCounter           int
	downloadCounter         int
	preparer                DataPreparer
	lastNotification        data.NotificationEvent
	UploadBackupFilesImpl   func(file string, nodeID string, encryptionType string) (string, error)
	AvailableSnapshotsImpl  func() ([]SnapshotInfo, error)
	DownloadBackupFilesImpl func(nodeID, backupID string) ([]string, error)
	MsgChannel              chan data.NotificationEvent
	torConfig               *tor.TorConfig
}

func newDefaultMockTester() *MockTester {
	p := MockTester{}
	p.preparer = func() (paths []string, nodeID string, err error) {
		return []string{"file1"}, "test-node-id", nil
	}
	p.AvailableSnapshotsImpl = func() ([]SnapshotInfo, error) {
		return []SnapshotInfo{}, nil
	}
	p.DownloadBackupFilesImpl = func(nodeID, backupID string) ([]string, error) {
		p.downloadCounter++
		return []string{}, nil
	}
	p.UploadBackupFilesImpl = func(file string, nodeID string, encryptionType string) (string, error) {
		time.Sleep(time.Millisecond * 10)
		p.uploadCounter++
		return "", nil
	}
	p.MsgChannel = make(chan data.NotificationEvent, 100)
	return &p
}

func (m *MockTester) UploadBackupFiles(file string, nodeID string, encryptionType string) (string, error) {
	return m.UploadBackupFilesImpl(file, nodeID, "")
}
func (m *MockTester) AvailableSnapshots() ([]SnapshotInfo, error) {
	return m.AvailableSnapshotsImpl()
}
func (m *MockTester) DownloadBackupFiles(nodeID, backupID string) ([]string, error) {
	return m.DownloadBackupFilesImpl(nodeID, backupID)
}

func (m *MockTester) SetTor(torConfig *tor.TorConfig) {
	m.torConfig = torConfig
}

func (m *MockTester) TestAuth() {
	return
}

func prepareBackupData() (paths []string, nodeID string, err error) {
	return []string{"file1"}, "test-node-id", nil
}

func createTestManager(mp *MockTester) (manager *Manager, err error) {
	backupDelay = time.Duration(0)
	RegisterProvider("mock", func(providerFactoryInfo ProviderFactoryInfo) (Provider, error) { return mp, nil })

	ntfnChan := make(chan data.NotificationEvent, 100)
	dir := os.TempDir()
	fmt.Println("tempDir " + dir)

	config := &config.Config{
		Network: "mainnet",
	}

	onEvent := func(d data.NotificationEvent) {
		ntfnChan <- d
	}
	manager, err = NewManager("mock", nil, onEvent, mp.preparer, config, btclog.Disabled)
	if err != nil {
		return
	}
	go func() {
		for {
			select {
			case msg := <-ntfnChan:
				mp.lastNotification = msg
				mp.MsgChannel <- msg
			case <-manager.quitChan:
				os.RemoveAll(path.Join(dir, "backup.db"))
				fmt.Println("Remove db from go routine")
				return
			}
		}
	}()
	return
}

type mockAuthError struct{}

func (me *mockAuthError) Error() string {
	return ""
}
func (me *mockAuthError) IsAuthError() bool {
	return true
}

func TestCreateDefaultProvider(t *testing.T) {
	tempDir := os.TempDir()
	fmt.Println("tempDir " + tempDir)
	ntfnChan := make(chan data.NotificationEvent)
	config := &config.Config{
		Network: "mainnet",
	}
	onEvent := func(d data.NotificationEvent) {
		ntfnChan <- d
	}
	manager, err := NewManager("gdrive", nil, onEvent, prepareBackupData, config, btclog.Disabled)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.destroy()

	if _, ok := manager.provider.(*GoogleDriveProvider); !ok {
		t.Error("default provider is not google drive")
	}
}

func TestRequestBackup(t *testing.T) {
	tester := newDefaultMockTester()
	manager, err := createTestManager(tester)
	if err != nil {
		t.Fatal(err)
	}
	manager.Start()
	defer manager.destroy()

	manager.RequestNodeBackup()
	waitForBackupEnd(tester.MsgChannel)
	if tester.uploadCounter != 1 {
		t.Error("Backup was not called after backup requested")
	}
}

func TestMultipleRequestBackup(t *testing.T) {
	tester := newDefaultMockTester()
	manager, err := createTestManager(tester)
	if err != nil {
		t.Fatal(err)
	}
	manager.Start()
	defer manager.destroy()

	for i := 0; i < 10; i++ {
		manager.RequestNodeBackup()
	}
	time.Sleep(300 * time.Millisecond)
	if tester.uploadCounter > 2 {
		t.Error("Backup was called too much times")
	}
}

func TestErrorInPrepareBackup(t *testing.T) {
	preparer := func() (paths []string, nodeID string, err error) {
		return nil, "", errors.New("failed to prepare")
	}
	tester := newDefaultMockTester()
	tester.preparer = preparer
	manager, err := createTestManager(tester)
	if err != nil {
		t.Fatal(err)
	}
	manager.Start()
	defer manager.destroy()

	manager.RequestNodeBackup()
	waitForBackupEnd(tester.MsgChannel)
	if tester.uploadCounter > 0 {
		t.Error("Backup was called despite error in preparing the data")
	}
}

func TestErrorInUpload(t *testing.T) {
	tester := newDefaultMockTester()
	tester.UploadBackupFilesImpl = func(files string, nodeID string, encryptionType string) (string, error) {
		return "", errors.New("failed to upload files")
	}
	manager, err := createTestManager(tester)
	if err != nil {
		t.Fatal(err)
	}
	manager.Start()
	defer manager.destroy()

	manager.RequestNodeBackup()
	waitForBackupEnd(tester.MsgChannel)
	if tester.uploadCounter > 0 {
		t.Error("Backup was called despite error in preparing the data")
	}
	if tester.lastNotification.Type != data.NotificationEvent_BACKUP_FAILED {
		t.Error("wrong fail notification type")
	}
}

func TestAuthError(t *testing.T) {
	tester := newDefaultMockTester()
	tester.UploadBackupFilesImpl = func(files string, nodeID string, encryptionType string) (string, error) {
		return "", &mockAuthError{}
	}
	manager, err := createTestManager(tester)
	if err != nil {
		t.Fatal(err)
	}
	manager.Start()
	defer manager.destroy()

	manager.RequestNodeBackup()
	waitForBackupEnd(tester.MsgChannel)
	if tester.uploadCounter > 0 {
		t.Error("Backup was called despite error in preparing the data")
	}
	if tester.lastNotification.Type != data.NotificationEvent_BACKUP_AUTH_FAILED {
		t.Error("wrong fail notification type")
	}
}

func TestRestoreWrongNumberOfFiles(t *testing.T) {
	tester := newDefaultMockTester()

	tester.DownloadBackupFilesImpl = func(nodeID, backupID string) ([]string, error) {
		return []string{"file1"}, nil
	}

	manager, err := createTestManager(tester)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.destroy()

	if _, err := manager.Restore("test-node-id", nil); err == nil {
		t.Fatal(err)
	}
}

func TestRestoreSuccess(t *testing.T) {
	tester := newDefaultMockTester()
	dir := os.TempDir()

	downloads := []string{"wallet.db", "channel.db", "breez.db"}
	sourceFiles := []string{}
	for _, d := range downloads {
		sourceFiles = append(sourceFiles, path.Join(dir, d))
	}

	tester.DownloadBackupFilesImpl = func(nodeID, backupID string) ([]string, error) {
		if nodeID != "test-node-id" {
			return nil, errors.New("node id not found")
		}

		for _, d := range sourceFiles {
			_, err := os.Create(d)
			if err != nil {
				t.Fatal(err)
			}
		}

		return sourceFiles, nil
	}

	manager, err := createTestManager(tester)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.destroy()

	restoredFiles, err := manager.Restore("test-node-id", nil)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		for _, f := range sourceFiles {
			os.Remove(f)
		}
		for _, f := range restoredFiles {
			os.Remove(f)
		}
	}()

	for _, f := range restoredFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Fatalf("file was not restored to the right location %v", f)
		}
	}
}

func TestBackupConflict(t *testing.T) {
	tester := newDefaultMockTester()

	tester.AvailableSnapshotsImpl = func() ([]SnapshotInfo, error) {
		return []SnapshotInfo{
			SnapshotInfo{
				NodeID:       "test-node-id",
				BackupID:     "conflict-backup-id",
				ModifiedTime: time.Now().String(),
			},
		}, nil
	}

	manager, err := createTestManager(tester)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.destroy()

	safe, err := manager.IsSafeToRunNode("test-node-id")
	if safe {
		t.Fatal("returned safe to run even though it isn't")
	}
}

func TestEncryptDecryptFiles(t *testing.T) {
	key := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	originalContent := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	dir := os.TempDir()
	file := path.Join(dir, "sourceFile")
	encryptedFile := path.Join(dir, "encryptedFile")
	decryptedFile := path.Join(dir, "decryptedFile")
	if err := ioutil.WriteFile(file, originalContent, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := encryptFile(file, encryptedFile, key); err != nil {
		t.Fatal(err)
	}

	if err := decryptFile(encryptedFile, decryptedFile, key); err != nil {
		t.Fatal(err)
	}
	content, err := ioutil.ReadFile(decryptedFile)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Compare(content, originalContent) != 0 {
		t.Error("Failed to decrypt file", content)
	}
}

func TestEncryptedBackup(t *testing.T) {
	securityPIN := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	originalContent := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	dir := os.TempDir()
	downloads := []string{
		path.Join(dir, "wallet.db"),
		path.Join(dir, "channel.db"),
		path.Join(dir, "breez.db"),
	}
	defer func() {
		for _, backupFile := range downloads {
			os.Remove(backupFile)
		}
	}()

	preparer := func() (paths []string, nodeID string, err error) {
		for _, backupFile := range downloads {
			if err := ioutil.WriteFile(backupFile, originalContent, os.ModePerm); err != nil {
				t.Fatal(err)
			}
		}
		return downloads, "test-node-id", nil
	}
	tester := newDefaultMockTester()
	tester.preparer = preparer
	tester.DownloadBackupFilesImpl = func(nodeID, backupID string) ([]string, error) {
		time.Sleep(100 * time.Millisecond)
		if nodeID != "test-node-id" {
			return nil, errors.New("node id not found")
		}

		return []string{path.Join(dir, "backup.zip")}, nil
	}

	manager, err := createTestManager(tester)
	if err != nil {
		t.Fatal(err)
	}
	manager.SetEncryptionKey(securityPIN, "PIN")
	manager.Start()
	defer manager.destroy()

	manager.RequestNodeBackup()
	waitForBackupEnd(tester.MsgChannel)
	paths, err := manager.Restore("test-node-id", securityPIN)
	if err != nil {
		t.Fatal("Restore failed", err)
	}
	defer func() {
		for _, f := range paths {
			os.Remove(f)
		}
	}()

	content, err := ioutil.ReadFile(paths[0])
	if err != nil {
		t.Error("Restore failed", err)
	}
	if bytes.Compare(content, originalContent) != 0 {
		t.Error("Restore failed to decreypt file", err, content)
	}
}

func waitForBackupEnd(msgChan chan data.NotificationEvent) {
	for {
		event := <-msgChan
		fmt.Println("waitForBackupEnd ", event)
		if event.Type != data.NotificationEvent_BACKUP_REQUEST {
			break
		}
	}
}
