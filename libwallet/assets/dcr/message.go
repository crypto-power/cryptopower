package dcr

import (
	"decred.org/dcrwallet/v2/errors"
	w "decred.org/dcrwallet/v2/wallet"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
	"gitlab.com/cryptopower/cryptopower/libwallet/utils"
)

func (asset *DCRAsset) SignMessage(passphrase, address, message string) ([]byte, error) {
	err := asset.UnlockWallet(passphrase)
	if err != nil {
		return nil, utils.TranslateError(err)
	}
	defer asset.LockWallet()

	return asset.signMessage(address, message)
}

func (asset *DCRAsset) signMessage(address, message string) ([]byte, error) {
	addr, err := stdaddr.DecodeAddress(address, asset.chainParams)
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

	ctx, _ := asset.ShutdownContextWithCancel()
	sig, err := asset.Internal().DCR.SignMessage(ctx, message, addr)
	if err != nil {
		return nil, utils.TranslateError(err)
	}

	return sig, nil
}

func (asset *DCRAsset) VerifyMessage(address, message, signatureBase64 string) (bool, error) {
	var valid bool

	addr, err := stdaddr.DecodeAddress(address, asset.chainParams)
	if err != nil {
		return false, utils.TranslateError(err)
	}

	signature, err := utils.DecodeBase64(signatureBase64)
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

	valid, err = w.VerifyMessage(message, addr, signature, asset.chainParams)
	if err != nil {
		return false, utils.TranslateError(err)
	}

	return valid, nil
}
