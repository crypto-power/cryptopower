package cryptomaterial

import (
	"image"

	"code.cryptopower.dev/group/cryptopower/ui/values"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"golang.org/x/image/draw"
)

type Image struct {
	image.Image
}

func NewImage(src image.Image) *Image {
	return &Image{
		// Modify a copy of the original image.
		Image: cloneToNRGBA(src),
	}
}

func (img *Image) layout(gtx C) D {
	newImg := &widget.Image{
		Src:   paint.NewImageOp(img.Image),
		Scale: 1,
	}
	return newImg.Layout(gtx)
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

func cloneToNRGBA(src image.Image) draw.Image {
	b := src.Bounds()
	dst := image.NewNRGBA(b)
	draw.Draw(dst, b, src, b.Min, draw.Src)
	return dst
}

func (img *Image) LayoutSize(gtx C, size unit.Dp) D {
	imgsize := img.Bounds().Size()
	heightWidthRatio := float32(imgsize.Y) / float32(imgsize.X)
	height := float32(size) * heightWidthRatio
	width := float32(size)

	// Set the expected size of the final image needed:
	dst := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)))

	// Resize to the icon: https://pkg.go.dev/golang.org/x/image/draw#pkg-variables:
	draw.CatmullRom.Scale(dst, dst.Rect, img, img.Bounds(), draw.Over, nil)

	img.Image = dst
	return img.layout(gtx)
}
