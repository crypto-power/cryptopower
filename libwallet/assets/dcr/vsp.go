package dcr

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	sharedW "code.cryptopower.dev/group/cryptopower/libwallet/assets/wallet"
	"code.cryptopower.dev/group/cryptopower/libwallet/internal/vsp"
	"code.cryptopower.dev/group/cryptopower/libwallet/utils"
	"decred.org/dcrwallet/v2/errors"
)

const (
	defaultVSPsUrl = "https://api.decred.org/?c=vsp"
)

// VSPClient loads or creates a VSP client instance for the specified host.
func (asset *DCRAsset) VSPClient(host string, pubKey []byte) (*vsp.Client, error) {
	asset.vspClientsMu.Lock()
	defer asset.vspClientsMu.Unlock()
	client, ok := asset.vspClients[host]
	if ok {
		return client, nil
	}

	cfg := vsp.Config{
		URL:    host,
		PubKey: base64.StdEncoding.EncodeToString(pubKey),
		Dialer: nil, // optional, but consider providing a value
		Wallet: asset.Internal().DCR,
	}
	client, err := vsp.New(cfg)
	if err != nil {
		return nil, err
	}
	asset.vspClients[host] = client
	return client, nil
}

// KnownVSPs returns a list of known VSPs. This list may be updated by calling
// ReloadVSPList. This method is safe for concurrent access.
func (asset *DCRAsset) KnownVSPs() []*VSP {
	asset.vspMu.RLock()
	defer asset.vspMu.RUnlock()
	return asset.vsps // TODO: Return a copy.
}

// SaveVSP marks a VSP as known and will be susbequently included as part of
// known VSPs.
func (asset *DCRAsset) SaveVSP(host string) (err error) {
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
func (asset *DCRAsset) LastUsedVSP() string {
	return asset.getVSPDBData().LastUsedVSP
}

// SaveLastUsedVSP saves the host of the last used VSP.
func (asset *DCRAsset) SaveLastUsedVSP(host string) {
	vspDbData := asset.getVSPDBData()
	vspDbData.LastUsedVSP = host
	asset.updateVSPDBData(vspDbData)
}

type vspDbData struct {
	SavedHosts  []string
	LastUsedVSP string
}

func (asset *DCRAsset) getVSPDBData() *vspDbData {
	vspDbData := new(vspDbData)
	asset.ReadUserConfigValue(sharedW.KnownVSPsConfigKey, vspDbData)
	return vspDbData
}

func (asset *DCRAsset) updateVSPDBData(data *vspDbData) {
	asset.SaveUserConfigValue(sharedW.KnownVSPsConfigKey, data)
}

// ReloadVSPList reloads the list of known VSPs.
// This method makes multiple network calls; should be called in a goroutine
// to prevent blocking the UI thread.
func (asset *DCRAsset) ReloadVSPList(ctx context.Context) {
	log.Debugf("Reloading list of known VSPs")
	defer log.Debugf("Reloaded list of known VSPs")

	vspDbData := asset.getVSPDBData()
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

	otherVSPHosts, err := defaultVSPs(string(asset.NetType()))
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

	asset.vspMu.Lock()
	asset.vsps = make([]*VSP, 0, len(vspList))
	for host, info := range vspList {
		asset.vsps = append(asset.vsps, &VSP{Host: host, VspInfoResponse: info})
	}
	asset.vspMu.Unlock()
}

func vspInfo(vspHost string) (*VspInfoResponse, error) {
	req := &utils.ReqConfig{
		Method:    http.MethodGet,
		HttpUrl:   vspHost + "/api/v3/vspinfo",
		IsRetByte: true,
	}

	var respBytes = []byte{}
	resp, err := utils.HttpRequest(req, &respBytes)
	if err != nil {
		return nil, err
	}

	vspInfoResponse := new(VspInfoResponse)
	if err := json.Unmarshal(respBytes, vspInfoResponse); err != nil {
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
	req := &utils.ReqConfig{
		Method:  http.MethodGet,
		HttpUrl: defaultVSPsUrl,
	}

	if _, err := utils.HttpRequest(req, &vspInfoResponse); err != nil {
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
