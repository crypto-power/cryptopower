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

func (img *Image) Layout12dp(gtx C) D {
	return img.LayoutSize(gtx, values.MarginPadding12)
}

func (img *Image) Layout16dp(gtx C) D {
	return img.LayoutSize(gtx, values.MarginPadding16)
}

func (img *Image) Layout20dp(gtx C) D {
	return img.LayoutSize(gtx, values.MarginPadding20)
}

func (img *Image) Layout24dp(gtx C) D {
	return img.LayoutSize(gtx, values.MarginPadding24)
}

func (img *Image) Layout36dp(gtx C) D {
	return img.LayoutSize(gtx, values.MarginPadding36)
}

func (img *Image) Layout48dp(gtx C) D {
	return img.LayoutSize(gtx, values.MarginPadding48)
}

func (img *Image) LayoutSize(gtx C, size unit.Dp) D {
	dst := image.NewRGBA(image.Rectangle{Max: image.Point{X: int(size), Y: int(size)}})
	draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)

	i := widget.Image{Src: paint.NewImageOp(dst)}
	i.Scale = 1
	return i.Layout(gtx)
}
