package dcr

import (
	"decred.org/dcrwallet/v2/errors"
	w "decred.org/dcrwallet/v2/wallet"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
)

func (wallet *Wallet) SignMessage(passphrase []byte, address string, message string) ([]byte, error) {
	err := wallet.UnlockWallet(passphrase)
	if err != nil {
		return nil, translateError(err)
	}
	defer wallet.LockWallet()

	return wallet.SignMessageDirect(address, message)
}

func (wallet *Wallet) SignMessageDirect(address string, message string) ([]byte, error) {
	addr, err := stdaddr.DecodeAddress(address, wallet.chainParams)
	if err != nil {
		return nil, translateError(err)
	}

	// Addresses must have an associated secp256k1 private key and therefore
	// must be P2PK or P2PKH (P2SH is not allowed).
	switch addr.(type) {
	case *stdaddr.AddressPubKeyEcdsaSecp256k1V0:
	case *stdaddr.AddressPubKeyHashEcdsaSecp256k1V0:
	default:
		return nil, errors.New(ErrInvalidAddress)
	}

	sig, err := wallet.Internal().SignMessage(wallet.ShutdownContext(), message, addr)
	if err != nil {
		return nil, translateError(err)
	}

	return sig, nil
}

func (wallet *Wallet) VerifyMessage(address string, message string, signatureBase64 string) (bool, error) {
	var valid bool

	addr, err := stdaddr.DecodeAddress(address, wallet.chainParams)
	if err != nil {
		return false, translateError(err)
	}

	signature, err := DecodeBase64(signatureBase64)
	if err != nil {
		return false, err
	}

	// Addresses must have an associated secp256k1 private key and therefore
	// must be P2PK or P2PKH (P2SH is not allowed).
	switch addr.(type) {
	case *stdaddr.AddressPubKeyEcdsaSecp256k1V0:
	case *stdaddr.AddressPubKeyHashEcdsaSecp256k1V0:
	default:
		return false, errors.New(ErrInvalidAddress)
	}

	valid, err = w.VerifyMessage(message, addr, signature, wallet.chainParams)
	if err != nil {
		return false, translateError(err)
	}

	return valid, nil
}
