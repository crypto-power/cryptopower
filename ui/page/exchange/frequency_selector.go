package exchange

import (
	"time"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/values"
)

const FrequencySelectorID = "FrequencySelectorID"

// FrequencySelector models a wiget for use for selecting exchanges.
type FrequencySelector struct {
	openSelectorDialog *cryptomaterial.Clickable
	*frequencyModal
	changed bool
}

// frequencyItem wraps the frequency in a clickable.
type frequencyItem struct {
	name      string
	item      time.Duration
	clickable *cryptomaterial.Clickable
}

type frequencyModal struct {
	*load.Load
	*cryptomaterial.Modal

	selectedFrequency  *frequencyItem
	frequencyCallback  func(*frequencyItem)
	dialogTitle        string
	onFrequencyClicked func(*frequencyItem)
	frequencyList      layout.List
	frequencyItems     []*frequencyItem
	isCancelable       bool
}

// NewFrequencySelector creates an frequency selector component.
// It opens a modal to select a desired frequency.
func NewFrequencySelector(l *load.Load) *FrequencySelector {
	fs := &FrequencySelector{
		openSelectorDialog: l.Theme.NewClickable(true),
	}

	fs.frequencyModal = newFrequencyModal(l).
		frequencyClicked(func(fi *frequencyItem) {
			if fs.selectedFrequency != nil {
				if fs.selectedFrequency.name != fi.name {
					fs.changed = true
				}
			}
			fs.setSelectedFrequency(fi)
			if fs.frequencyCallback != nil {
				fs.frequencyCallback(fi)
			}
		})
	fs.frequencyItems = fs.buildFrequencyItems()
	return fs
}

// setSelectedFrequency sets fi as the current selected frequency.
func (fs *FrequencySelector) setSelectedFrequency(fi *frequencyItem) {
	fs.selectedFrequency = fi
}

func (fs *FrequencySelector) handle(gtx C, window app.WindowNavigator) {
	for fs.openSelectorDialog.Clicked(gtx) {
		fs.title(fs.dialogTitle)
		window.ShowModal(fs.frequencyModal)
	}
}

func (fs *FrequencySelector) Layout(window app.WindowNavigator, gtx C) D {
	fs.handle(gtx, window)

	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.MatchParent,
		Height:     cryptomaterial.WrapContent,
		Padding:    layout.UniformInset(values.MarginPadding12),
		Background: fs.Theme.Color.White,
		Border: cryptomaterial.Border{
			Width:  values.MarginPadding2,
			Color:  fs.Theme.Color.Gray2,
			Radius: cryptomaterial.Radius(8),
		},
		Clickable: fs.openSelectorDialog,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			txt := fs.Theme.Label(values.TextSize16, values.String(values.StrFrequency))
			txt.Color = fs.Theme.Color.Gray7
			if fs.selectedFrequency != nil {
				txt = fs.Theme.Label(values.TextSize16, fs.selectedFrequency.name)
				txt.Color = fs.Theme.Color.Text
			}
			return txt.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						ic := cryptomaterial.NewIcon(fs.Theme.Icons.DropDownIcon)
						ic.Color = fs.Theme.Color.Gray1
						return ic.Layout(gtx, values.MarginPadding20)
					}),
				)
			})
		}),
	)
}

// newFrequencyModal return a modal used for drawing the frequency list.
func newFrequencyModal(l *load.Load) *frequencyModal {
	fm := &frequencyModal{
		Load:          l,
		Modal:         l.Theme.ModalFloatTitle(values.String(values.StrSelectFrequency), l.IsMobileView(), nil),
		frequencyList: layout.List{Axis: layout.Vertical},
		isCancelable:  true,
		dialogTitle:   values.String(values.StrSelectFrequency),
	}

	fm.Modal.ShowScrollbar(true)
	return fm
}

func (fs *FrequencySelector) buildFrequencyItems() []*frequencyItem {
	return []*frequencyItem{
		{
			name:      "Fastest",
			item:      time.Duration(1),
			clickable: fs.Theme.NewClickable(true),
		},
		{
			name:      "1x/3 hr",
			item:      time.Duration(3),
			clickable: fs.Theme.NewClickable(true),
		},
		{
			name:      "1x/6 hr",
			item:      time.Duration(6),
			clickable: fs.Theme.NewClickable(true),
		},
		{
			name:      "1x/12 hr",
			item:      time.Duration(12),
			clickable: fs.Theme.NewClickable(true),
		},
		{
			name:      "1x/day",
			item:      time.Duration(24),
			clickable: fs.Theme.NewClickable(true),
		},
	}
}

func (fm *frequencyModal) OnResume() {}

func (fm *frequencyModal) Handle(gtx C) {
	for _, frequencyItem := range fm.frequencyItems {
		if frequencyItem.clickable.Clicked(gtx) {
			fm.onFrequencyClicked(frequencyItem)
			fm.Dismiss()
		}
	}

	if fm.Modal.BackdropClicked(gtx, fm.isCancelable) {
		fm.Dismiss()
	}
}

func (fm *frequencyModal) title(title string) *frequencyModal {
	fm.dialogTitle = title
	return fm
}

func (fm *frequencyModal) frequencyClicked(callback func(*frequencyItem)) *frequencyModal {
	fm.onFrequencyClicked = callback
	return fm
}

func (fm *frequencyModal) Layout(gtx C) D {
	w := []layout.Widget{
		func(gtx C) D {
			titleTxt := fm.Theme.Label(values.TextSize20, fm.dialogTitle)
			titleTxt.Color = fm.Theme.Color.Text
			titleTxt.Font.Weight = font.SemiBold
			return layout.Inset{
				Top: values.MarginPaddingMinus15,
			}.Layout(gtx, titleTxt.Layout)
		},
		func(gtx C) D {
			return layout.Stack{Alignment: layout.NW}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return fm.frequencyList.Layout(gtx, len(fm.frequencyItems), func(gtx C, index int) D {
						return fm.modalListItemLayout(gtx, fm.frequencyItems[index])
					})
				}),
			)
		},
	}

	return fm.Modal.Layout(gtx, w)
}

func (fm *frequencyModal) modalListItemLayout(gtx C, frequencyItem *frequencyItem) D {
	return cryptomaterial.LinearLayout{
		Width:     cryptomaterial.MatchParent,
		Height:    cryptomaterial.WrapContent,
		Margin:    layout.Inset{Bottom: values.MarginPadding4},
		Padding:   layout.Inset{Top: values.MarginPadding8, Bottom: values.MarginPadding8},
		Clickable: frequencyItem.clickable,
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			exchName := fm.Theme.Label(values.TextSize18, frequencyItem.name)
			exchName.Color = fm.Theme.Color.Text
			exchName.Font.Weight = font.Normal
			return exchName.Layout(gtx)
		}),
	)
}

func (fm *frequencyModal) OnDismiss() {}
