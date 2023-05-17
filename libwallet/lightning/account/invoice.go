package account

import (
	"context"
	"fmt"
	"log"

	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnrpc/signrpc"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zpay32"
)

func (a *Service) generateInvoiceWithNewAmount(payReq string, newAmount int64) (string, []byte, error) {
	invoice, err := zpay32.Decode(payReq, a.activeParams)
	if err != nil {
		return "", nil, fmt.Errorf("zpay32.Decode() error: %w", err)
	}

	signerClient := a.daemonAPI.SignerClient()
	if signerClient == nil {
		return "", nil, fmt.Errorf("API is not ready")
	}

	m := lnwire.MilliSatoshi(newAmount)
	invoice.MilliSat = &m
	signer := zpay32.MessageSigner{SignCompact: func(msg []byte) ([]byte, error) {
		kl := signrpc.KeyLocator{
			KeyFamily: int32(keychain.KeyFamilyNodeKey),
			KeyIndex:  0,
		}

		r, err := signerClient.SignMessage(context.Background(),
			&signrpc.SignMessageReq{
				Msg:        msg,
				KeyLoc:     &kl,
				CompactSig: true,
				DoubleHash: false,
			})
		if err != nil {
			return nil, fmt.Errorf("m.client.SignMessage() error: %w", err)
		}
		return r.Signature, nil
	}}
	newInvoice, err := invoice.Encode(signer)
	if err != nil {
		log.Printf("invoice.Encode() error: %v", err)
	}
	return newInvoice, (*invoice.PaymentAddr)[:], err
}
