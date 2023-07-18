package assets

import (
	"fmt"
	"sync"

	"gioui.org/font"
	"gioui.org/font/opentype"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/gomediumitalic"
	"golang.org/x/image/font/gofont/goregular"
)

var (
	once       sync.Once
	collection []font.FontFace
)

// FontCollection registers the fonts to used in the app
func FontCollection() []font.FontFace {
	regularItalic, err := getFontByte("fonts/source_sans_pro_It.otf")
	if err != nil {
		regularItalic = goitalic.TTF
	}

	regular, err := getFontByte("fonts/source_sans_pro_regular.otf")
	if err != nil {
		regular = goregular.TTF
	}

	semibold, err := getFontByte("fonts/source_sans_pro_semibold.otf")
	if err != nil {
		semibold = gomedium.TTF
	}

	semiboldItalic, err := getFontByte("fonts/source_sans_pro_semiboldIt.otf")
	if err != nil {
		semiboldItalic = gomediumitalic.TTF
	}

	bold, err := getFontByte("fonts/source_sans_pro_bold.otf")
	if err != nil {
		bold = gobold.TTF
	}

	boldItalic, err := getFontByte("fonts/source_sans_pro_boldIt.otf")
	if err != nil {
		boldItalic = gobolditalic.TTF
	}

	once.Do(func() {
		register(font.Font{}, regular)
		register(font.Font{Style: font.Italic}, regularItalic)
		register(font.Font{Weight: font.Bold}, bold)
		register(font.Font{Style: font.Italic, Weight: font.Bold}, boldItalic)
		register(font.Font{Weight: font.Medium}, semibold)
		register(font.Font{Weight: font.Medium, Style: font.Italic}, semiboldItalic)
		// Ensure that any outside appends will not reuse the backing store.
		n := len(collection)
		collection = collection[:n:n]
	})
	return collection
}

func register(fnt font.Font, fontByte []byte) {
	face, err := opentype.Parse(fontByte)
	if err != nil {
		panic(fmt.Errorf("failed to parse font: %v", err))
	}
	fnt.Typeface = "Go"
	collection = append(collection, font.FontFace{Font: fnt, Face: face})
}

func getFontByte(path string) ([]byte, error) {
	return content.ReadFile(path)
}
