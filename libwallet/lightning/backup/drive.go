package backup

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/breez/breez/tor"

	"github.com/btcsuite/btclog"
	"golang.org/x/oauth2"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

const (
	backupIDProperty             = "backupID"
	activeBackupFolderProperty   = "activeBackupFolder"
	backupEncryptedProperty      = "backupEncrypted" //legacy
	backupEncryptionTypeProperty = "backupEncryptionType"
)

// driveServiceError is the type of error this provider returns in case
// of API error. It also implements the ProviderError interface
type driveServiceError struct {
	err error
}

func (d *driveServiceError) Error() string {
	return d.err.Error()
}

func (d *driveServiceError) IsAuthError() bool {
	if ferr, ok := d.err.(*googleapi.Error); ok {
		return ferr.Code == 401 || ferr.Code == 403
	}
	if strings.Contains(d.err.Error(), "AuthError") {
		return true
	}
	return false
}

// TokenSource is a structure that implements the auth2.TokenSource interface to be used by
// GoogleDriveBackupProvider
type TokenSource struct {
	authService AuthService
	log         btclog.Logger
}

// Token retrieved a token from the AuthService provided
func (s *TokenSource) Token() (*oauth2.Token, error) {
	s.log.Infof("Token source before signIn")
	token, err := s.authService.SignIn()
	if err != nil {
		s.log.Infof("Token source got error %v", err)
		return nil, err
	}
	s.log.Infof("Token source successfully got token with length: %v", len(token))
	return &oauth2.Token{
		AccessToken: token,
		Expiry:      time.Now().Add(time.Second),
	}, nil
}

// GoogleDriveProvider is an implementation of backup.Provider interface.
// It uses app data in google drive as a storage mechanism.
// Backups are stored in the following manner:
//  1. Nodes backup will be saved directly under "appDataFolder", which is the user
//     data directory in google drive. Folders will be named by the convention:
//     "snapshot-<node_id>".
//  2. When a request for backup lands, this provider will do the following:
//     2.1 Create a nested folder under the node folder.
//     2.2 Save all the backup files under this created folder.
//     2.3 Update the "activeBackupFolderProperty" metadata property of the node folder
//     to be the newly created folder id.
//     2.4 Delete stale backup folder that are not in use anymore for this node id.
//  3. When a request for restore (download) lands, this provider will do the following:
//     3.1 Identify the "active backup folder" by reading the metadata property of the node folder.
//     3.2 Download the files
//     3.3 Mark this backup as "restored by this instance" by assign the value "backupID" to the node
//     metadata property "backupIDProperty". This value will be used to detect the problematic case
//     where two nodes are running the same backup...
type GoogleDriveProvider struct {
	driveService *drive.Service
	log          btclog.Logger
}

// NewGoogleDriveProvider creates a new instance of GoogleDriveProvider.
// It needs AuthService that will provide the access token, and refresh access token functionality
// It implements the backup.Provider interface by using google drive as storage.
func NewGoogleDriveProvider(authService AuthService, log btclog.Logger) (*GoogleDriveProvider, error) {
	tokenSource := &TokenSource{authService: authService, log: log}
	httpClient := oauth2.NewClient(context.Background(), tokenSource)
	driveService, err := drive.New(httpClient)
	if err != nil {
		return nil, fmt.Errorf("Error creating drive service, %v", err)
	}
	return &GoogleDriveProvider{driveService: driveService, log: log}, nil
}

// AvailableSnapshots fetches all existing backup snapshots that exist in the user app data.
// There is only one snapshot per node id.
func (p *GoogleDriveProvider) AvailableSnapshots() ([]SnapshotInfo, error) {
	p.log.Info("GoogleDriveProvider, AvailableSnapshots started")
	r, err := p.driveService.Files.List().Spaces("appDataFolder").
		Fields("files(name)", "files(modifiedTime)", "files(appProperties)").
		Q("'appDataFolder' in parents and name contains 'snapshot-'").
		Do()
	if err != nil {
		p.log.Infof("GoogleDriveProvider unable to fetch available snapshot: %v", err)
		return nil, &driveServiceError{err}
	}
	var backups []SnapshotInfo

	// We are going over all nodes that have a snapshot.
	// For each snapshot we extract the NodeID, Modifiedtime
	// and BackupID (it will only exist if this node has been restored)
	p.log.Infof("GoogleDriveProvider going over fetched snapshots")
	for _, f := range r.Files {
		p.log.Infof("GoogleDriveProvider checking snapshot, name:%v, id:%v, size=%v", f.Name, f.Id, f.Size)
		var backupID string
		for k, v := range f.AppProperties {
			if k == backupIDProperty {
				backupID = v
			}
		}
		legacyEncrypted := f.AppProperties[backupEncryptedProperty]
		encryptionType, hasEncryptionType := f.AppProperties[backupEncryptionTypeProperty]
		backups = append(backups, SnapshotInfo{
			NodeID:         string(f.Name[9:]),
			ModifiedTime:   f.ModifiedTime,
			Encrypted:      !hasEncryptionType && legacyEncrypted == "true" || encryptionType != "",
			EncryptionType: encryptionType,
			BackupID:       backupID,
		})
	}

	return backups, nil
}

// UploadBackupFiles uploads the backup files related to a specific node to google drive.
// It does the following:
//  1. Creates a fresh new folder under the node folder.
//  2. Uploads all files to the newly created folder.
//  3. update the node folder metadata property "activeBackupFolderProperty" to contain the
//     id of the new folder.
func (p *GoogleDriveProvider) UploadBackupFiles(file string, nodeID string, encryptionType string) (string, error) {
	p.log.Infof("uploadBackupFiles started, nodeID=%v", nodeID)

	// Fetch the node folder
	nodeFolder, err := p.nodeFolder(nodeID)
	if err != nil {
		p.log.Infof("uploadBackupFiles failed to fetch node folder:%v", nodeID)
		return "", &driveServiceError{err}
	}

	// Get account information.
	a, err := p.driveService.About.Get().Fields("user").Do()
	if err != nil {
		fmt.Printf("An error occurred: %v\n", err)
		return "", &driveServiceError{err}
	}

	successChan := make(chan struct{})
	defer close(successChan)
	errorChan := make(chan error)
	defer close(errorChan)

	var backupFolderID string
	p.log.Infof("uploadBackupFiles backup folder created for node:%v", nodeFolder.Id)
	// Upload the files
	go func(filePath string) {
		fileName := path.Base(filePath)
		file, err := os.Open(filePath)
		if err != nil {
			errorChan <- err
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			errorChan <- err
			return
		}

		var uploadedFile *drive.File

		// get the current backup folder id
		currentFolderID, ok := nodeFolder.AppProperties[activeBackupFolderProperty]
		if !ok {
			// Create the backup folder
			newBackupFolder, err := p.createFolder(nodeFolder.Id, "backup")
			if err != nil {
				p.log.Infof("uploadBackupFiles failed to create backkup folder:%v", nodeFolder.Id)
				errorChan <- err
				return
			}

			currentFolderID = newBackupFolder.Id
		}

		// List all filed under the backup folder
		r, err := p.driveService.Files.List().Spaces("appDataFolder").Q(fmt.Sprintf("'%v' in parents", currentFolderID)).Do()

		if err != nil {
			errorChan <- err
			return
		}

		// Find our file if exists
		for _, f := range r.Files {
			if f.Name == fileName {
				uploadedFile = f
				break
			}
		}

		// If the file exists we need to update its content
		if uploadedFile != nil {
			p.log.Infof("Updating file %v size: %v", fileName, info.Size())
			uploadedFile, err = p.driveService.Files.Update(uploadedFile.Id, &drive.File{}).Media(file).Fields("md5Checksum").Do()
			if err != nil {
				p.log.Infof("uploadBackupFiles failed to update file at folder:%v", currentFolderID)
				errorChan <- &driveServiceError{err}
				return
			}
			p.log.Infof("uploadBackupFiles succeeded to update file at folder:%v", currentFolderID)
		} else {
			// At this case we need to upload a new file
			p.log.Infof("Uploading file %v size: %v", fileName, info.Size())
			uploadedFile, err = p.driveService.Files.Create(&drive.File{
				Name:    fileName,
				Parents: []string{currentFolderID}},
			).Media(file).Fields("md5Checksum").Do()
			if err != nil {
				p.log.Infof("uploadBackupFiles failed to upload file at folder:%v", currentFolderID)
				errorChan <- &driveServiceError{err}
				return
			}
			p.log.Infof("uploadBackupFiles succeeded to upload file at folder:%v", currentFolderID)
		}

		checksum, err := fileChecksum(filePath)
		if err != nil {
			p.log.Infof("failed to calculate checksum for path: %v", filePath)
			errorChan <- errors.New("failed to calculate checksum")
			return
		}
		if uploadedFile.Md5Checksum != checksum {
			p.log.Infof("backup file checksum mismatch: %v", filePath)
			errorChan <- errors.New("uploaded file checksum doesn't match")
			return
		}
		backupFolderID = currentFolderID
		successChan <- struct{}{}
	}(file)

	var uploadErr error
	select {
	case uploadErr = <-errorChan:
	case <-successChan:
	}

	if uploadErr != nil {
		return "", uploadErr
	}

	// Update active backup folder for this specific node folder
	folderUpdate := &drive.File{AppProperties: map[string]string{
		activeBackupFolderProperty:   backupFolderID,
		backupEncryptionTypeProperty: encryptionType,
	}}
	_, err = p.driveService.Files.Update(nodeFolder.Id, folderUpdate).Do()
	if err != nil {
		p.log.Infof("backup update backup folder for nodeID: %v", nodeFolder.Id)
		return "", &driveServiceError{err}
	}
	if err := p.deleteStaleSnapshots(nodeFolder.Id, backupFolderID); err != nil {
		p.log.Errorf("failed to delete stale snapshots %v", err)
	}
	p.log.Infof("UploadBackupFiles finished successfully")
	return a.User.EmailAddress, nil
}

// DownloadBackupFiles is responsible for download a specific node backup and updating.
// that this backup was now restored by this instance represented by "backupID"
func (p *GoogleDriveProvider) DownloadBackupFiles(nodeID, backupID string) ([]string, error) {
	// fetch the node folder
	nodeFolder, err := p.nodeFolder(nodeID)
	if err != nil {
		return nil, &driveServiceError{err}
	}

	// Fetch thte backup folder
	if nodeFolder.AppProperties == nil {
		return nil, fmt.Errorf("can't find active backup for node %v", nodeID)
	}
	folderID, ok := nodeFolder.AppProperties[activeBackupFolderProperty]
	if !ok {
		return nil, fmt.Errorf("can't find active backup for node %v", nodeID)
	}

	// Query all the files under the backup folder
	r, err := p.driveService.Files.List().Spaces("appDataFolder").Q(fmt.Sprintf("'%v' in parents", folderID)).Do()
	if err != nil {
		return nil, &driveServiceError{err}
	}

	dir, err := ioutil.TempDir("", "gdrive")
	if err != nil {
		return nil, err
	}
	// Download all the files in parallel
	var downloaded []string

	successChan := make(chan string)
	defer close(successChan)
	errorChan := make(chan error)
	defer close(errorChan)

	for _, f := range r.Files {
		go func(file *drive.File) {
			path, err := p.downloadFile(file, dir)
			if err != nil {
				errorChan <- &driveServiceError{err}
			} else {
				successChan <- path
			}
		}(f)
	}

	var downloadErr error
	for i := 0; i < len(r.Files); i++ {
		select {
		case path := <-successChan:
			downloaded = append(downloaded, path)
		case downloadErr = <-errorChan:
			continue
		}
	}
	if downloadErr != nil {
		return nil, downloadErr
	}

	// Upate this backup as restored by this instance.
	folderUpdate := &drive.File{AppProperties: map[string]string{backupIDProperty: backupID}}
	_, err = p.driveService.Files.Update(nodeFolder.Id, folderUpdate).Do()
	if err != nil {
		return nil, &driveServiceError{err}
	}
	return downloaded, nil
}

// nodeFolder is responsible for fetching the node folder. If it doesn't exist it creates it.
func (p *GoogleDriveProvider) nodeFolder(nodeID string) (*drive.File, error) {
	r, err := p.driveService.Files.List().Spaces("appDataFolder").
		Fields("files(id)", "files(appProperties)", "files(name)").
		Q(fmt.Sprintf("'appDataFolder' in parents and name = 'snapshot-%v'", nodeID)).
		Do()
	if err != nil {
		return nil, err
	}
	if len(r.Files) == 0 {
		return p.createFolder("appDataFolder", "snapshot-"+nodeID)
	}
	if len(r.Files) > 1 {
		return nil, fmt.Errorf("there is more than one folder with name %v", nodeID)
	}
	return r.Files[0], nil
}

// createFolder creates a folder in drive under "parent"
func (p *GoogleDriveProvider) createFolder(parent string, name string) (*drive.File, error) {
	return p.driveService.Files.Create(&drive.File{
		Name:     name,
		Parents:  []string{parent},
		MimeType: "application/vnd.google-apps.folder"}).Do()
}

// downloadFile download a file from drive to a specific dir.
// the file name is preserved.
func (p *GoogleDriveProvider) downloadFile(file *drive.File, dir string) (string, error) {
	res, err := p.driveService.Files.Get(file.Id).Download()
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	targetPath := path.Join(dir, file.Name)
	out, err := os.Create(targetPath)
	if err != nil {
		return "", err
	}
	defer out.Close()
	_, err = io.Copy(out, res.Body)
	return targetPath, err
}

// deleteStaleSnapshots delete all snapshots for a specific node except for the active one.
func (p *GoogleDriveProvider) deleteStaleSnapshots(nodeFolderID, activeBackupFolderID string) error {

	r, err := p.driveService.Files.List().Spaces("appDataFolder").
		Q(fmt.Sprintf("'%v' in parents", nodeFolderID)).
		Do()
	if err != nil {
		return err
	}
	p.log.Infof("going over snapshots, looking for %v", activeBackupFolderID)
	for _, f := range r.Files {
		p.log.Infof("going over snapshot %v size=%v", f.Id, f.Size)
		if f.Id == activeBackupFolderID {
			continue
		}
		backupFiles, err := p.driveService.Files.List().Spaces("appDataFolder").
			Fields("files(name)", "files(id)").
			Q(fmt.Sprintf("'%v' in parents", f.Id)).
			Do()
		if err != nil {
			return err
		}

		go func() {
			for _, b := range backupFiles.Files {
				p.log.Infof("deleting file %v - name = %v", b.Id, b.Name)
				if err := p.driveService.Files.Delete(b.Id).Do(); err != nil {
					p.log.Infof("failed to delete file %v - name = %v", b.Id, b.Name)
					return
				}
			}
			if err := p.driveService.Files.Delete(f.Id).Do(); err != nil {
				p.log.Infof("failed to folder %v - name = %v", f.Id, f.Name)
				return
			}
		}()
	}
	return nil
}

func (p *GoogleDriveProvider) SetTor(torConfig *tor.TorConfig) {
	return
}

func (p *GoogleDriveProvider) TestAuth() error {
	return nil
}

func fileChecksum(filePath string) (string, error) {
	hash := md5.New()
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", err
	}
	checksum := hash.Sum(nil)
	return hex.EncodeToString(checksum), nil
}
