package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

const (
	keyLength = 32
	nonceSize = 12
)

func encryptFile(source, dest string, key []byte) error {

	if(len(key) != keyLength) {
		return fmt.Errorf("wrong key length given: %d, expected: %d", len(key), keyLength)
	}

	fileContent, err := ioutil.ReadFile(source)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	nonce := make([]byte, nonceSize )
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	encryptedContent := aesgcm.Seal(nonce, nonce, fileContent, nil)

	if err = ioutil.WriteFile(dest, encryptedContent, os.ModePerm); err != nil {
		return err
	}

	return nil
}

func decryptFile(source, dest string, key []byte) error {

	if(len(key) != keyLength) {
		return fmt.Errorf("wrong key length given: %d, expected: %d", len(key), keyLength)
	}

	fileContent, err := ioutil.ReadFile(source)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonceSize := aesgcm.NonceSize()
	nonce, cipherContent := fileContent[:nonceSize], fileContent[nonceSize:]

	decryptedContent, err := aesgcm.Open(nil, nonce, cipherContent, nil)
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(dest, decryptedContent, os.ModePerm); err != nil {
		return err
	}

	return nil
}
