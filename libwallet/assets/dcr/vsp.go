package dcr

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"decred.org/dcrwallet/v4/vsp"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	vspdClient "github.com/decred/vspd/client/v3"
	vspd "github.com/decred/vspd/types/v2"
)

const (
	defaultVSPsURL = "https://api.decred.org/?c=vsp"
)

// VSPClient loads or creates a VSP client instance for the specified host.
func (asset *Asset) VSPClient(account int32, host string, pubKey []byte) (*vsp.Client, error) {
	if !asset.WalletOpened() {
		return nil, utils.ErrDCRNotInitialized
	}

	asset.vspMu.Lock()
	defer asset.vspMu.Unlock()
	if client, ok := asset.vspClients[host]; ok {
		return client, nil
	}

	client, err := asset.createVspClient(account, host, pubKey)
	if err != nil {
		return nil, err
	}

	asset.vspClients[host] = client
	return client, nil
}

func (asset *Asset) createVspClient(account int32, host string, pubKey []byte) (*vsp.Client, error) {
	cfg := vsp.Config{
		URL:    host,
		PubKey: base64.StdEncoding.EncodeToString(pubKey),
		Dialer: nil, // optional, but consider providing a value
		Wallet: asset.Internal().DCR,
		Params: asset.Internal().DCR.ChainParams(),
	}

	// When the account number provided is greater than -1, the provided account
	// will be used to purchase tickets otherwise the default tickets purchase
	// account will be used.
	if account != -1 {
		if !asset.IsTicketBuyerAccountSet() {
			return nil, utils.ErrTicketPurchaseAccMissing
		}
		account = asset.AutoTicketsBuyerConfig().PurchaseAccount
	}

	cfg.Policy = &vsp.Policy{
		MaxFee:     0.2e8,
		FeeAcct:    uint32(account),
		ChangeAcct: uint32(account),
	}

	return vsp.New(cfg, log)
}

// KnownVSPs returns a list of known VSPs. This list may be updated by calling
// ReloadVSPList. This method is safe for concurrent access.
func (asset *Asset) KnownVSPs() []*VSP {
	asset.vspMu.RLock()
	defer asset.vspMu.RUnlock()
	return asset.vsps
}

// SaveVSP marks a VSP as known and will be susbequently included as part of
// known VSPs.
func (asset *Asset) SaveVSP(host string) (err error) {
	// check if host already exists
	vspDbData := asset.getVSPDBData()
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
	if info.Network != string(asset.NetType()) {
		return fmt.Errorf("invalid net %s", info.Network)
	}

	vspDbData.SavedHosts = append(vspDbData.SavedHosts, host)
	asset.updateVSPDBData(vspDbData)

	asset.vspMu.Lock()
	asset.vsps = append(asset.vsps, &VSP{Host: host, VspInfoResponse: info})
	asset.vspMu.Unlock()

	return
}

// LastUsedVSP returns the host of the last used VSP, as saved by the
// SaveLastUsedVSP() method.
func (asset *Asset) LastUsedVSP() string {
	return asset.getVSPDBData().LastUsedVSP
}

// SaveLastUsedVSP saves the host of the last used VSP.
func (asset *Asset) SaveLastUsedVSP(host string) {
	vspDbData := asset.getVSPDBData()
	vspDbData.LastUsedVSP = host
	asset.updateVSPDBData(vspDbData)
}

type vspDbData struct {
	SavedHosts  []string
	LastUsedVSP string
}

func (asset *Asset) getVSPDBData() *vspDbData {
	vspDbData := new(vspDbData)
	_ = asset.ReadUserConfigValue(sharedW.KnownVSPsConfigKey, vspDbData)
	return vspDbData
}

func (asset *Asset) updateVSPDBData(data *vspDbData) {
	asset.SaveUserConfigValue(sharedW.KnownVSPsConfigKey, data)
}

// ReloadVSPList reloads the list of known VSPs.
// This method makes multiple network calls; should be called in a goroutine
// to prevent blocking the UI thread.
func (asset *Asset) ReloadVSPList(ctx context.Context) {
	log.Debugf("Reloading list of known VSPs")
	defer log.Debugf("Reloaded list of known VSPs")

	vspDbData := asset.getVSPDBData()
	vspList := make(map[string]*vspd.VspInfoResponse)
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

	network := string(asset.NetType())
	otherVSPHosts, err := defaultVSPs()
	if err != nil {
		log.Debugf("get default vsp list error: %v", err)
	}

	for url, VSPInfo := range otherVSPHosts {
		if !strings.Contains(network, VSPInfo.Network) {
			continue
		}

		host := "https://" + url
		if _, wasAdded := vspList[host]; wasAdded {
			continue
		}

		vspList[host] = VSPInfo
		if ctx.Err() != nil {
			return // context canceled, abort
		}
	}

	asset.vspMu.Lock()
	asset.vsps = make([]*VSP, 0, len(vspList))
	for host, info := range vspList {
		asset.vsps = append(asset.vsps, &VSP{Host: host, VspInfoResponse: info})
	}
	asset.vspMu.Unlock()
}

func vspInfo(vspHost string) (*vspd.VspInfoResponse, error) {
	req := &utils.ReqConfig{
		Method:    http.MethodGet,
		HTTPURL:   vspHost + "/api/v3/vspinfo",
		IsRetByte: true,
	}

	respBytes := []byte{}
	resp, err := utils.HTTPRequest(req, &respBytes)
	if err != nil {
		return nil, err
	}

	vspInfoResponse := new(vspd.VspInfoResponse)
	if err := json.Unmarshal(respBytes, vspInfoResponse); err != nil {
		return nil, err
	}

	// Validate server response.
	err = vspdClient.ValidateServerSignature(resp, respBytes, vspInfoResponse.PubKey)
	return vspInfoResponse, err
}

// defaultVSPs returns a list of known VSPs.
func defaultVSPs() (map[string]*vspd.VspInfoResponse, error) {
	var vspInfoResponse map[string]*vspd.VspInfoResponse
	req := &utils.ReqConfig{
		Method:  http.MethodGet,
		HTTPURL: defaultVSPsURL,
	}

	if _, err := utils.HTTPRequest(req, &vspInfoResponse); err != nil {
		return nil, err
	}

	// The above API does not return the pubKeys for the VSPs.
	return vspInfoResponse, nil
}
