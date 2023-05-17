package swapfunds

import (
	breezservice "github.com/breez/breez/breez"
)

func (s *Service) redeemAllRemovedFunds() error {
	s.log.Infof("redeemAllRemovedFunds")
	if !s.lightningTransfersReady() {
		s.log.Infof("Skipping redeemAllRemovedFunds HasActiveChannel=%v", s.daemonAPI.HasActiveChannel())
		return nil
	}
	hashes, err := s.breezDB.FetchRedeemablePaymentHashes()
	if err != nil {
		s.log.Errorf("failed to fetchRedeemablePaymentHashes, %v", err)
		return err
	}

	for _, hash := range hashes {
		s.log.Infof("Redeeming transaction for has %v", hash)
		paid, err := s.breezDB.IsInvoiceHashPaid(hash)
		if err != nil {
			s.log.Infof("Skipping payment hash %v as couldn't fetch payment from db %v", hash, err)
			continue
		}
		if !paid {
			s.log.Infof("Skipping payment hash %v as it was not paid by this client")
			continue
		}

		txID, err := s.redeemRemovedFundsForHash(hash)
		if err != nil {
			s.log.Errorf("failed to redeem funds for hash %v, %v", hash, err)
		} else {
			s.log.Infof("successfully redeemed funds for hash %v, txid=%v", hash, txID)
		}
	}
	return err
}

func (s *Service) redeemRemovedFundsForHash(hash string) (string, error) {
	fundManager, ctx, cancel := s.breezAPI.NewFundManager()
	defer cancel()
	redeemReply, err := fundManager.RedeemRemovedFunds(ctx, &breezservice.RedeemRemovedFundsRequest{Paymenthash: hash})
	if err != nil {
		s.log.Errorf("RedeemRemovedFunds failed for hash: %v,   %v", hash, err)
		return "", err
	}
	return redeemReply.Txid, s.breezDB.UpdateRedeemTxForPayment(hash, redeemReply.Txid)
}
