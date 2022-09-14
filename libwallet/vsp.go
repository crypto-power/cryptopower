package libwallet

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"strings"

	"decred.org/dcrwallet/v2/errors"
	"gitlab.com/raedah/cryptopower/libwallet/internal/vsp"
)

// VSPClient loads or creates a VSP client instance for the specified host.
func (wallet *Wallet) VSPClient(host string, pubKey []byte) (*vsp.Client, error) {
	wallet.vspClientsMu.Lock()
	defer wallet.vspClientsMu.Unlock()
	client, ok := wallet.vspClients[host]
	if ok {
		return client, nil
	}

	cfg := vsp.Config{
		URL:    host,
		PubKey: base64.StdEncoding.EncodeToString(pubKey),
		Dialer: nil, // optional, but consider providing a value
		Wallet: wallet.Internal(),
	}
	client, err := vsp.New(cfg)
	if err != nil {
		return nil, err
	}
	wallet.vspClients[host] = client
	return client, nil
}

// KnownVSPs returns a list of known VSPs. This list may be updated by calling
// ReloadVSPList. This method is safe for concurrent access.
func (mw *MultiWallet) KnownVSPs() []*VSP {
	mw.vspMu.RLock()
	defer mw.vspMu.RUnlock()
	return mw.vsps // TODO: Return a copy.
}

// SaveVSP marks a VSP as known and will be susbequently included as part of
// known VSPs.
func (mw *MultiWallet) SaveVSP(host string) (err error) {
	// check if host already exists
	vspDbData := mw.getVSPDBData()
	for _, savedHost := range vspDbData.SavedHosts {
		if savedHost == host {
			return fmt.Errorf("duplicate host %s", host)
		}
	}

	// validate host network
	info, err := vspInfo(host)
	if err != nil {
		return err
	}

	// TODO: defaultVSPs() uses strings.Contains(network, vspInfo.Network).
	if info.Network != mw.NetType() {
		return fmt.Errorf("invalid net %s", info.Network)
	}

	vspDbData.SavedHosts = append(vspDbData.SavedHosts, host)
	mw.updateVSPDBData(vspDbData)

	mw.vspMu.Lock()
	mw.vsps = append(mw.vsps, &VSP{Host: host, VspInfoResponse: info})
	mw.vspMu.Unlock()

	return
}

// LastUsedVSP returns the host of the last used VSP, as saved by the
// SaveLastUsedVSP() method.
func (mw *MultiWallet) LastUsedVSP() string {
	return mw.getVSPDBData().LastUsedVSP
}

// SaveLastUsedVSP saves the host of the last used VSP.
func (mw *MultiWallet) SaveLastUsedVSP(host string) {
	vspDbData := mw.getVSPDBData()
	vspDbData.LastUsedVSP = host
	mw.updateVSPDBData(vspDbData)
}

type vspDbData struct {
	SavedHosts  []string
	LastUsedVSP string
}

func (mw *MultiWallet) getVSPDBData() *vspDbData {
	vspDbData := new(vspDbData)
	mw.ReadUserConfigValue(KnownVSPsConfigKey, vspDbData)
	return vspDbData
}

func (mw *MultiWallet) updateVSPDBData(data *vspDbData) {
	mw.SaveUserConfigValue(KnownVSPsConfigKey, data)
}

// ReloadVSPList reloads the list of known VSPs.
// This method makes multiple network calls; should be called in a goroutine
// to prevent blocking the UI thread.
func (mw *MultiWallet) ReloadVSPList(ctx context.Context) {
	log.Debugf("Reloading list of known VSPs")
	defer log.Debugf("Reloaded list of known VSPs")

	vspDbData := mw.getVSPDBData()
	vspList := make(map[string]*VspInfoResponse)
	for _, host := range vspDbData.SavedHosts {
		vspInfo, err := vspInfo(host)
		if err != nil {
			// User saved this VSP. Log an error message.
			log.Errorf("get vsp info error for %s: %v", host, err)
		} else {
			vspList[host] = vspInfo
		}
		if ctx.Err() != nil {
			return // context canceled, abort
		}
	}

	otherVSPHosts, err := defaultVSPs(mw.NetType())
	if err != nil {
		log.Debugf("get default vsp list error: %v", err)
	}
	for _, host := range otherVSPHosts {
		if _, wasAdded := vspList[host]; wasAdded {
			continue
		}
		vspInfo, err := vspInfo(host)
		if err != nil {
			log.Debugf("vsp info error for %s: %v\n", host, err) // debug only, user didn't request this VSP
		} else {
			vspList[host] = vspInfo
		}
		if ctx.Err() != nil {
			return // context canceled, abort
		}
	}

	mw.vspMu.Lock()
	mw.vsps = make([]*VSP, 0, len(vspList))
	for host, info := range vspList {
		mw.vsps = append(mw.vsps, &VSP{Host: host, VspInfoResponse: info})
	}
	mw.vspMu.Unlock()
}

func vspInfo(vspHost string) (*VspInfoResponse, error) {
	vspInfoResponse := new(VspInfoResponse)
	resp, respBytes, err := HttpGet(vspHost+"/api/v3/vspinfo", vspInfoResponse)
	if err != nil {
		return nil, err
	}

	// Validate server response.
	sigStr := resp.Header.Get("VSP-Server-Signature")
	sig, err := base64.StdEncoding.DecodeString(sigStr)
	if err != nil {
		return nil, fmt.Errorf("error validating VSP signature: %v", err)
	}
	if !ed25519.Verify(vspInfoResponse.PubKey, respBytes, sig) {
		return nil, errors.New("bad signature from VSP")
	}

	return vspInfoResponse, nil
}

// defaultVSPs returns a list of known VSPs.
func defaultVSPs(network string) ([]string, error) {
	var vspInfoResponse map[string]*VspInfoResponse
	_, _, err := HttpGet("https://api.decred.org/?c=vsp", &vspInfoResponse)
	if err != nil {
		return nil, err
	}

	// The above API does not return the pubKeys for the
	// VSPs. Only return the host since we'll still need
	// to make another API call to get the VSP pubKeys.
	vsps := make([]string, 0)
	for url, vspInfo := range vspInfoResponse {
		if strings.Contains(network, vspInfo.Network) {
			vsps = append(vsps, "https://"+url)
		}
	}
	return vsps, nil
}
