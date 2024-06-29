package components

import (
	"image/color"
	"testing"

	"gioui.org/op"
	"github.com/crypto-power/cryptopower/ui/assets"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
)

// TestFormatBalance currently only tests for panics.
func TestFormatBalance(t *testing.T) {
	th := cryptomaterial.NewTheme(assets.FontCollection(), assets.DecredIcons, false)
	ld := &load.Load{Theme: th}
	gtx := C{Ops: new(op.Ops)}
	tests := []struct {
		name, amount string
	}{{
		name:   "normal",
		amount: "1 DCR",
	}, {
		name:   "one d",
		amount: "1 D",
	}, {
		name:   "decimals",
		amount: "1.1 DCR",
	}, {
		name: "blank amount",
	}, {
		name:   "just the coin",
		amount: "DCR",
	}}
	for _, test := range tests {
		t.Run(test.name, func(_ *testing.T) {
			formatBalance(gtx, ld, test.amount, 1, color.NRGBA{}, false, false)
		})
	}
}
