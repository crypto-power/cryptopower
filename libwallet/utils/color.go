package utils

import "image/color"

type ColorScheme struct {
	R uint8   // Red Subpixel
	G uint8   // Green Subpixel
	B uint8   // Blue Subpixel
	O float64 // Opacity; value range 0-1
}

type GradientColorScheme struct {
	Color1 ColorScheme
	Color2 ColorScheme
	Blend1 float64 // percent of position along X axis where color1 blend ends.
	Blend2 float64 // percent of position along X axis where color2 blend ends.
}

// NRGBAColor converts figma color scheme to gioui nrgba color scheme.
func (c *ColorScheme) NRGBAColor() color.NRGBA {
	transparency := 127.0 - (127.0 * c.O) // opacity = (127 - transparency) / 127
	return color.NRGBA{
		R: c.R,
		G: c.G,
		B: c.B,
		A: uint8(transparency),
	}
}

func GradientColorSchemes() map[AssetType]GradientColorScheme {
	return map[AssetType]GradientColorScheme{
		BTCWalletAsset: {
			Color1: ColorScheme{R: 196, G: 203, B: 210, O: 0.3}, // rgba(196, 203, 210, 0.3)
			Blend1: 34.76,                                       // 34.76%
			Color2: ColorScheme{R: 248, G: 152, B: 36, O: 0.3},  // rgba(248, 152, 36, 0.3)
			Blend2: 65.88,                                       // 65.88 %
		},
		DCRWalletAsset: {
			Color1: ColorScheme{R: 41, G: 112, B: 255, O: 0.3}, // rgba(41, 112, 255, 0.3)
			Blend1: 34.76,                                      // 34.76%
			Color2: ColorScheme{R: 45, G: 216, B: 163, O: 0.3}, // rgba(45, 216, 163, 0.3)
			Blend2: 65.88,                                      // 65.88 %
		},
		LTCWalletAsset: {
			Color1: ColorScheme{R: 224, G: 224, B: 224, O: 0.3}, // rgba(224, 224, 224, 0.3)
			Blend1: 34.76,                                       // 34.76%
			Color2: ColorScheme{R: 56, G: 115, B: 223, O: 0.3},  // rgba(56, 115, 223, 0.3)
			Blend2: 65.88,                                       // 65.88 %
		},
		BCHWalletAsset: {
			Color1: ColorScheme{R: 24, G: 171, B: 89, O: 0.3}, // rgba(247, 147, 26, 0.3)
			Blend1: 34.76,                                       // 34.76%
			Color2: ColorScheme{R: 54, G: 115, B: 36, O: 0.3},   // rgba(54, 115, 36, 0.3)
			Blend2: 65.88,                                       // 65.88 %
		},
	}
}
