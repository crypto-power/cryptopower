// This code is available on the terms of the project LICENSE.md file,
// also available online at https://blueoakcouncil.org/license/1.0.0.
// Copy-pasta from dcrdex/client/webserver/http.go#L469

package dcrdex

import (
	"decred.org/dcrdex/client/asset"
	"decred.org/dcrdex/client/core"
	"decred.org/dcrdex/dex"
)

var (
	unbip = dex.BipIDSymbol
	bip   = dex.BipSymbolID
)

const defaultConversionFactor = 1e8

func defaultUnitInfo(symbol string) dex.UnitInfo {
	return dex.UnitInfo{
		AtomicUnit: "atoms",
		Conventional: dex.Denomination{
			ConversionFactor: defaultConversionFactor,
			Unit:             symbol,
		},
	}
}

func (pg *DEXMarketPage) orderReader(ord *core.Order) *core.OrderReader {
	unitInfo := func(assetID uint32, symbol string) dex.UnitInfo {
		unitInfo, err := asset.UnitInfo(assetID)
		if err == nil {
			return unitInfo
		}
		xc := pg.dexc.Exchanges()[ord.Host]
		a, found := xc.Assets[assetID]
		if !found || a.UnitInfo.Conventional.ConversionFactor == 0 {
			return defaultUnitInfo(symbol)
		}
		return a.UnitInfo
	}

	feeAssetInfo := func(assetID uint32, symbol string) (string, dex.UnitInfo) {
		return unbip(assetID), unitInfo(assetID, symbol)
	}

	baseFeeAssetSymbol, baseFeeUintInfo := feeAssetInfo(ord.BaseID, ord.BaseSymbol)
	quoteFeeAssetSymbol, quoteFeeUnitInfo := feeAssetInfo(ord.QuoteID, ord.QuoteSymbol)

	return &core.OrderReader{
		Order:               ord,
		BaseUnitInfo:        unitInfo(ord.BaseID, ord.BaseSymbol),
		BaseFeeUnitInfo:     baseFeeUintInfo,
		BaseFeeAssetSymbol:  baseFeeAssetSymbol,
		QuoteUnitInfo:       unitInfo(ord.QuoteID, ord.QuoteSymbol),
		QuoteFeeUnitInfo:    quoteFeeUnitInfo,
		QuoteFeeAssetSymbol: quoteFeeAssetSymbol,
	}
}
