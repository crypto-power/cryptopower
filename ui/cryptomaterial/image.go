package cryptomaterial

import (
	"image"

	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/crypto-power/cryptopower/ui/values"
	"golang.org/x/image/draw"
)

type Image struct {
	image.Image
}

func NewImage(src image.Image) *Image {
	return &Image{
		Image: src,
	}
}

// reduced the image original scale of 1 by half to 0.5 fix blurry images
// this in turn reduced the imahe layout size by half. Multiplying the
// layout size by 2 to give the original image size to scale ratio.
func (img *Image) Layout12dp(gtx C) D {
	return img.LayoutSize(gtx, values.MarginPadding12*2)
}

func (img *Image) Layout16dp(gtx C) D {
	return img.LayoutSize(gtx, values.MarginPadding16*2)
}

func (img *Image) Layout20dp(gtx C) D {
	return img.LayoutSize(gtx, values.MarginPadding20*2)
}

func (img *Image) Layout24dp(gtx C) D {
	return img.LayoutSize(gtx, values.MarginPadding24*2)
}

func (img *Image) Layout36dp(gtx C) D {
	return img.LayoutSize(gtx, values.MarginPadding36*2)
}

func (img *Image) Layout48dp(gtx C) D {
	return img.LayoutSize(gtx, values.MarginPadding48*2)
}

func (img *Image) LayoutSize(gtx C, size unit.Dp) D {
	dst := image.NewRGBA(image.Rectangle{Max: image.Point{X: int(size), Y: int(size)}})
	draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)

	i := widget.Image{Src: paint.NewImageOp(dst)}
	i.Scale = .5 // reduced the original scale of 1 by half to fix blurry images
	return i.Layout(gtx)
}
