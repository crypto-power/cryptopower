package exchange

import (
	"gioui.org/font"
	"gioui.org/io/input"
	"gioui.org/layout"

	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet/instantswap"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

const ExSelectorID = "ExSelectorID"

// ExSelector models a widget for use for selecting exchanges.
type ExSelector struct {
	openSelectorDialog *cryptomaterial.Clickable
	*exchangeModal
	changed bool
}

// Exchange models exchanges.
type Exchange struct {
	Name   string
	Server instantswap.ExchangeServer
	Icon   *cryptomaterial.Image
}

// exchangeItem wraps the exchangeserver in a clickable.
type exchangeItem struct {
	item      *Exchange
	clickable *cryptomaterial.Clickable
}

type exchangeModal struct {
	*load.Load
	*cryptomaterial.Modal

	selectedExchange  *Exchange
	exchangeCallback  func(*Exchange)
	dialogTitle       string
	onExchangeClicked func(*Exchange)
	exchangeList      layout.List
	exchangeItems     []*exchangeItem
	eventSource       input.Source
	isCancelable      bool
}

// NewExSelector creates an exchange selector component.
// It opens a modal to select a desired exchange.
func NewExSelector(l *load.Load, server ...instantswap.Server) *ExSelector {
	es := &ExSelector{
		openSelectorDialog: l.Theme.NewClickable(true),
	}

	es.exchangeModal = newExchangeModal(l).
		exchangeClicked(func(exch *Exchange) {
			if es.selectedExchange != nil {
				if es.selectedExchange.Name != exch.Name {
					es.changed = true
				}
			}
			es.SetSelectedExchange(exch)
			if es.exchangeCallback != nil {
				es.exchangeCallback(exch)
			}
		})
	es.exchangeItems = es.buildExchangeItems(server...)
	return es
}

// SupportedExchanges returns a slice containing all the exchanges
// Currently supported. If the server param is passed, it returns
// a slice  containing the filtered server only.
func (es *ExSelector) SupportedExchanges(server ...instantswap.Server) []*Exchange {
	// check if server is not nil
	if len(server) > 0 {
		exchng := &Exchange{
			Name: server[0].CapFirstLetter(),
			Server: instantswap.ExchangeServer{
				Server: server[0],
			},
			Icon: components.GetServerIcon(es.Theme, server[0].ToString()),
		}

		return []*Exchange{exchng}
	}

	exchangeServers := es.AssetsManager.InstantSwap.ExchangeServers()

	var exchange []*Exchange
	for _, exchangeServer := range exchangeServers {
		exchng := &Exchange{
			Name:   exchangeServer.Server.CapFirstLetter(),
			Server: exchangeServer,
			Icon:   components.GetServerIcon(es.Theme, exchangeServer.Server.ToString()),
		}

		exchange = append(exchange, exchng)
	}

	return exchange
}

// SelectedExchange returns the currently selected Exchange.
func (es *ExSelector) SelectedExchange() *Exchange {
	return es.selectedExchange
}

// SetSelectedExchange sets exch as the current selected exchange.
func (es *ExSelector) SetSelectedExchange(exch *Exchange) {
	es.selectedExchange = exch
}

// Title Sets the title of the exchange list dialog.
func (es *ExSelector) Title(title string) *ExSelector {
	es.dialogTitle = title
	return es
}

// ExchangeSelected sets the callback executed when an exchange is selected.
func (es *ExSelector) ExchangeSelected(callback func(*Exchange)) *ExSelector {
	es.exchangeCallback = callback
	return es
}

// SetSelectedExchangeName sets the exchange whose Name field is
// equals to {name} as the current selected exchange.
// If it can find exchange whose Name field equals name it returns silently.
func (es *ExSelector) SetSelectedExchangeName(name string) {
	ex := es.SupportedExchanges()
	for _, v := range ex {
		if v.Name == name {
			es.SetSelectedExchange(v)
			return
		}
	}
}

func (es *ExSelector) Handle(gtx C, window app.WindowNavigator) {
	for es.openSelectorDialog.Clicked(gtx) {
		es.title(es.dialogTitle)
		window.ShowModal(es.exchangeModal)
	}
}

func (es *ExSelector) Layout(window app.WindowNavigator, gtx C) D {
	es.Handle(gtx, window)

	bg := es.Theme.Color.White
	if es.AssetsManager.IsDarkModeOn() {
		bg = es.Theme.Color.Background
	}

	return cryptomaterial.LinearLayout{
		Width:      cryptomaterial.MatchParent,
		Height:     cryptomaterial.WrapContent,
		Padding:    layout.UniformInset(values.MarginPadding12),
		Background: bg,
		Border: cryptomaterial.Border{
			Width:  values.MarginPadding2,
			Color:  es.Theme.Color.Gray2,
			Radius: cryptomaterial.Radius(8),
		},
		Clickable: es.openSelectorDialog,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if es.selectedExchange == nil {
				return D{}
			}
			return layout.Inset{
				Right: values.MarginPadding8,
			}.Layout(gtx, es.selectedExchange.Icon.Layout24dp)
		}),
		layout.Rigid(func(gtx C) D {
			txt := es.Theme.Label(values.TextSize16, values.String(values.StrServer))
			txt.Color = es.Theme.Color.Gray7
			if es.selectedExchange != nil {
				txt = es.Theme.Label(values.TextSize16, es.selectedExchange.Name)
				txt.Color = es.Theme.Color.Text
			}
			return txt.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						ic := cryptomaterial.NewIcon(es.Theme.Icons.DropDownIcon)
						ic.Color = es.Theme.Color.Gray1
						return ic.Layout(gtx, values.MarginPadding20)
					}),
				)
			})
		}),
	)
}

// newExchangeModal return a modal used for drawing the exchange list.
func newExchangeModal(l *load.Load) *exchangeModal {
	em := &exchangeModal{
		Load:         l,
		Modal:        l.Theme.ModalFloatTitle(values.String(values.StrSelectAServer), l.IsMobileView()),
		exchangeList: layout.List{Axis: layout.Vertical},
		isCancelable: true,
		dialogTitle:  values.String(values.StrSelectAServer),
	}

	em.Modal.ShowScrollbar(true)
	return em
}

func (es *ExSelector) buildExchangeItems(server ...instantswap.Server) []*exchangeItem {
	exList := es.SupportedExchanges(server...)
	exItems := make([]*exchangeItem, 0)
	for _, v := range exList {
		exItems = append(exItems, &exchangeItem{
			item:      v,
			clickable: es.Theme.NewClickable(true),
		})
	}
	return exItems
}

func (em *exchangeModal) OnResume() {}

func (em *exchangeModal) Handle(gtx C) {
	// if em.eventQueue != nil {
	for _, exchangeItem := range em.exchangeItems {
		for exchangeItem.clickable.Clicked(gtx) {
			em.onExchangeClicked(exchangeItem.item)
			em.Dismiss()
		}
	}
	// }

	if em.Modal.BackdropClicked(gtx, em.isCancelable) {
		em.Dismiss()
	}
}

func (em *exchangeModal) title(title string) *exchangeModal {
	em.dialogTitle = title
	return em
}

func (em *exchangeModal) exchangeClicked(callback func(*Exchange)) *exchangeModal {
	em.onExchangeClicked = callback
	return em
}

func (em *exchangeModal) Layout(gtx C) D {
	em.eventSource = gtx.Source
	w := []layout.Widget{
		func(gtx C) D {
			titleTxt := em.Theme.Label(values.TextSize20, em.dialogTitle)
			titleTxt.Color = em.Theme.Color.Text
			titleTxt.Font.Weight = font.SemiBold
			return layout.Inset{
				Top: values.MarginPaddingMinus15,
			}.Layout(gtx, titleTxt.Layout)
		},
		func(gtx C) D {
			return layout.Stack{Alignment: layout.NW}.Layout(gtx,
				layout.Expanded(func(gtx C) D {
					return em.exchangeList.Layout(gtx, len(em.exchangeItems), func(gtx C, index int) D {
						return em.modalListItemLayout(gtx, em.exchangeItems[index])
					})
				}),
			)
		},
	}

	return em.Modal.Layout(gtx, w)
}

func (em *exchangeModal) modalListItemLayout(gtx C, exchangeItem *exchangeItem) D {
	return cryptomaterial.LinearLayout{
		Width:     cryptomaterial.MatchParent,
		Height:    cryptomaterial.WrapContent,
		Margin:    layout.Inset{Bottom: values.MarginPadding4},
		Padding:   layout.Inset{Top: values.MarginPadding8, Bottom: values.MarginPadding8},
		Clickable: exchangeItem.clickable,
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{
				Right: values.MarginPadding18,
			}.Layout(gtx, exchangeItem.item.Icon.Layout24dp)
		}),
		layout.Rigid(func(gtx C) D {
			exchName := em.Theme.Label(values.TextSize18, exchangeItem.item.Name)
			exchName.Color = em.Theme.Color.Text
			exchName.Font.Weight = font.Normal
			return exchName.Layout(gtx)
		}),
	)
}

func (em *exchangeModal) OnDismiss() {}
