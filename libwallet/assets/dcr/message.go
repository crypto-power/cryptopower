package dcr

import (
	"decred.org/dcrwallet/v2/errors"
	w "decred.org/dcrwallet/v2/wallet"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
	"gitlab.com/raedah/cryptopower/libwallet/utils"
)

func (wallet *Wallet) SignMessage(passphrase []byte, address string, message string) ([]byte, error) {
	err := wallet.UnlockWallet(passphrase)
	if err != nil {
		return nil, utils.TranslateError(err)
	}
	defer wallet.LockWallet()

	return wallet.signMessage(address, message)
}

func (wallet *Wallet) signMessage(address string, message string) ([]byte, error) {
	addr, err := stdaddr.DecodeAddress(address, wallet.chainParams)
	if err != nil {
		return nil, utils.TranslateError(err)
	}

	// Addresses must have an associated secp256k1 private key and therefore
	// must be P2PK or P2PKH (P2SH is not allowed).
	switch addr.(type) {
	case *stdaddr.AddressPubKeyEcdsaSecp256k1V0:
	case *stdaddr.AddressPubKeyHashEcdsaSecp256k1V0:
	default:
		return nil, errors.New(utils.ErrInvalidAddress)
	}

	ctx, _ := wallet.ShutdownContextWithCancel()
	sig, err := wallet.Internal().DCR.SignMessage(ctx, message, addr)
	if err != nil {
		return nil, utils.TranslateError(err)
	}

	return sig, nil
}

func (wallet *Wallet) VerifyMessage(address string, message string, signatureBase64 string) (bool, error) {
	var valid bool

	addr, err := stdaddr.DecodeAddress(address, wallet.chainParams)
	if err != nil {
		return false, utils.TranslateError(err)
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
		return false, errors.New(utils.ErrInvalidAddress)
	}

	valid, err = w.VerifyMessage(message, addr, signature, wallet.chainParams)
	if err != nil {
		return false, utils.TranslateError(err)
	}

	return valid, nil
}
