package cryptomaterial

import (
	"image"
	"sync"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/crypto-power/cryptopower/ui/values"
	"golang.org/x/image/draw"
)

type Image struct {
	image.Image

	// Keep a cache for scaled images to reduce resource use.
	layoutSizeMtx sync.Mutex
	layoutSizeDp  unit.Dp
	layoutSizeImg *image.RGBA

	layoutSize2Mtx                 sync.Mutex
	layoutSize2DpX, layoutSize2DpY unit.Dp
	layoutSize2Img                 *image.RGBA
}

func NewImage(src image.Image) *Image {
	return &Image{
		Image: src,
	}
}

// reduced the image original scale of 1 by half to 0.5 fix blurry images
// this in turn reduced the image layout size by half. Multiplying the
// layout size by 2 to give the original image size to scale ratio.
func (img *Image) Layout8dp(gtx C) D {
	return img.LayoutSize(gtx, values.MarginPadding8)
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
	var dst *image.RGBA
	img.layoutSizeMtx.Lock()
	if img.layoutSizeDp == size {
		dst = img.layoutSizeImg
	} else {
		dst = image.NewRGBA(image.Rectangle{Max: image.Point{X: int(size * 2), Y: int(size * 2)}})
		img.layoutSizeImg = dst
		img.layoutSizeDp = size
		draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
	}
	img.layoutSizeMtx.Unlock()

	i := widget.Image{Src: paint.NewImageOp(dst)}
	i.Scale = .5 // reduced the original scale of 1 by half to fix blurry images
	return i.Layout(gtx)
}

func (img *Image) LayoutSize2(gtx C, width, height unit.Dp) D {
	var dst *image.RGBA
	img.layoutSize2Mtx.Lock()
	if img.layoutSize2DpX == width && img.layoutSize2DpY == height {
		dst = img.layoutSize2Img
	} else {
		dst = image.NewRGBA(image.Rectangle{Max: image.Point{X: int(width * 2), Y: int(height * 2)}})
		img.layoutSize2Img = dst
		img.layoutSize2DpX = width
		img.layoutSize2DpY = height
		draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
	}
	img.layoutSize2Mtx.Unlock()

	i := widget.Image{Src: paint.NewImageOp(dst)}
	i.Scale = .5 // reduced the original scale of 1 by half to fix blurry images
	return i.Layout(gtx)
}

func (img *Image) LayoutSizeWithRadius(gtx C, width, height unit.Dp, radius int) D {
	m := op.Record(gtx.Ops)
	dims := img.LayoutSize2(gtx, width, height)
	call := m.Stop()
	defer clip.RRect{
		Rect: image.Rectangle{Max: image.Point{X: gtx.Dp(unit.Dp(width)), Y: gtx.Dp(unit.Dp(height))}},
		NE:   radius, NW: radius, SE: radius, SW: radius,
	}.Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return dims
}
