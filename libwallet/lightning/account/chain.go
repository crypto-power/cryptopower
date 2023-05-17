package account

import (
	"context"
	"errors"

	breezservice "github.com/breez/breez/breez"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/walletrpc"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
)

const (
	defaultSatPerByteFee = 50
)

/*
ValidateAddress validates a bitcoin address based on the network type
*/
func (a *Service) ValidateAddress(address string) error {
	var network *chaincfg.Params

	if a.cfg.Network == "testnet" {
		network = &chaincfg.TestNet3Params
	} else if a.cfg.Network == "simnet" {
		network = &chaincfg.SimNetParams
	} else if a.cfg.Network == "mainnet" {
		network = &chaincfg.MainNetParams
	} else {
		return errors.New("unknown network type " + a.cfg.Network)
	}

	addr, err := btcutil.DecodeAddress(address, network)
	if err != nil {
		a.log.Errorf("Error parsing %s as address\t", address)
		return err
	}
	if _, ok := addr.(*btcutil.AddressPubKey); ok {
		a.log.Errorf("Using pay-to-pubkey address %s is considered as an error\t", address)
		return btcutil.ErrUnknownAddressType
	}

	return nil
}

/*
SendWalletCoins executes a request to send wallet coins to a particular address.
*/
func (a *Service) SendWalletCoins(address string, satPerByteFee int64) (string, error) {
	lnclient := a.daemonAPI.APIClient()
	res, err := lnclient.SendCoins(context.Background(), &lnrpc.SendCoinsRequest{
		Addr: address, SatPerByte: satPerByteFee, SendAll: true})
	if err != nil {
		return "", err
	}
	return res.Txid, nil
}

/*
GetDefaultSatPerByteFee returns the default sat per byte fee for on chain transactions
*/
func (a *Service) GetDefaultSatPerByteFee() (int64, error) {
	walletKityClient := a.daemonAPI.WalletKitClient()
	if walletKityClient == nil {
		return 0, errors.New("API not ready")
	}
	feeResponse, err := walletKityClient.EstimateFee(context.Background(),
		&walletrpc.EstimateFeeRequest{ConfTarget: 6})
	if err != nil {
		return 0, err
	}
	return int64(chainfee.SatPerKWeight(feeResponse.SatPerKw).FeePerKVByte() / 1000), nil
}

/*
RegisterPeriodicSync registeres this token for periodic sync notifications.
*/
func (a *Service) RegisterPeriodicSync(token string) error {
	c, ctx, cancel := a.breezAPI.NewSyncNotifierClient()
	defer cancel()
	_, err := c.RegisterPeriodicSync(ctx, &breezservice.RegisterPeriodicSyncRequest{NotificationToken: token})
	if err != nil {
		a.log.Errorf("fail to register for periodic sync: %v", err)
	} else {
		a.log.Info("registered successfuly for periodic sync")
	}
	return err
}
