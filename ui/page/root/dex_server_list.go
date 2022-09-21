package root

import (
	"sort"
	"strings"

	"decred.org/dcrdex/client/core"
	"gioui.org/layout"
	"gioui.org/widget/material"

	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/modal"
	"gitlab.com/raedah/cryptopower/ui/page/components"
	"gitlab.com/raedah/cryptopower/ui/values"
)

func (pg *WalletDexServerSelector) initDexServerSelectorOption() {
	pg.knownDexServers = pg.Theme.NewClickableList(layout.Vertical)
}

// isLoadingDexClient check for Dexc start, initialized, loggedin status,
// since Dex client UI not required for app password, IsInitialized and IsLoggedIn should be done at libwallet.
func (pg *WalletDexServerSelector) isLoadingDexClient() bool {
	return pg.Dexc().Core() == nil || !pg.Dexc().Core().IsInitialized() || !pg.Dexc().IsLoggedIn()
}

// startDexClient do start DEX client,
// initialize and login to DEX,
// since Dex client UI not required for app password, initialize and login should be done at libwallet.
func (pg *WalletDexServerSelector) startDexClient() {
	var err error
	defer func() {
		if err != nil {
			errModal := modal.NewErrorModal(pg.Load, err.Error(), modal.DefaultClickFunc())
			pg.ParentWindow().ShowModal(errModal)
		}
	}()

	if _, err = pg.WL.MultiWallet.StartDexClient(); err != nil {
		return
	}

	// TODO: move to libwallet sine bypass Dex password by DEXClientPass
	if !pg.Dexc().Initialized() {
		if err = pg.Dexc().InitializeWithPassword([]byte(values.DEXClientPass)); err != nil {
			return
		}
	}

	if !pg.Dexc().IsLoggedIn() {
		if err = pg.Dexc().Login([]byte(values.DEXClientPass)); err != nil {
			// todo fix  dex password error
			// pg.Toast.NotifyError(err.Error())
			return
		}
	}
}

func (pg *WalletDexServerSelector) dexServersLayout(gtx C) D {
	if pg.isLoadingDexClient() {
		return layout.Center.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = 50
			return material.Loader(pg.Theme.Base).Layout(gtx)
		})
	}

	knownDexServers := pg.mapKnowDexServers()
	dexServers := sortDexExchanges(knownDexServers)
	return pg.knownDexServers.Layout(gtx, len(dexServers), func(gtx C, i int) D {
		dexServer := dexServers[i]
		hostport := strings.Split(dexServer, ":")
		host := hostport[0]
		pg.shadowBox.SetShadowRadius(14)

		return cryptomaterial.LinearLayout{
			Width:      cryptomaterial.WrapContent,
			Height:     cryptomaterial.WrapContent,
			Padding:    layout.UniformInset(values.MarginPadding18),
			Background: pg.Theme.Color.Surface,
			Alignment:  layout.Middle,
			Shadow:     pg.shadowBox,
			Margin:     layout.UniformInset(values.MarginPadding5),
			Border:     cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
		}.Layout(gtx,
			layout.Flexed(1, pg.Theme.Label(values.TextSize16, host).Layout),
		)
	})
}

// sortDexExchanges convert map cert into a sorted slice
func sortDexExchanges(mapCert map[string][]byte) []string {
	servers := make([]string, 0, len(mapCert))
	for host := range mapCert {
		servers = append(servers, host)
	}
	sort.Slice(servers, func(i, j int) bool {
		return servers[i] < servers[j]
	})
	return servers
}

func (pg *WalletDexServerSelector) mapKnowDexServers() map[string][]byte {
	knownDexServers := core.CertStore[pg.Dexc().Core().Network()]
	dexServer := new(components.DexServer)
	err := pg.Load.WL.MultiWallet.ReadUserConfigValue(load.KnownDexServersConfigKey, &dexServer)
	if err != nil {
		return knownDexServers
	}
	if dexServer.SavedHosts == nil {
		return knownDexServers
	}
	for host, cert := range dexServer.SavedHosts {
		knownDexServers[host] = cert
	}
	return knownDexServers
}
