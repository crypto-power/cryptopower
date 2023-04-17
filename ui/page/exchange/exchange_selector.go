package exchange

import (
	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/text"

	"code.cryptopower.dev/group/cryptopower/app"
	"code.cryptopower.dev/group/cryptopower/libwallet/instantswap"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/values"
)

const ExchangeSelectorID = "ExchangeSelectorID"

// ExchangeSelector models a wiget for use for selecting exchanges.
type ExchangeSelector struct {
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
	eventQueue        event.Queue
	isCancelable      bool
}

// NewExchangeSelector creates an exchange selector component.
// It opens a modal to select a desired exchange.
func NewExchangeSelector(l *load.Load, server ...instantswap.Server) *ExchangeSelector {
	es := &ExchangeSelector{
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
func (es *ExchangeSelector) SupportedExchanges(server ...instantswap.Server) []*Exchange {
	// check if server is not nil
	if len(server) > 0 {
		exchng := &Exchange{
			Name: server[0].CapFirstLetter(),
			Server: instantswap.ExchangeServer{
				Server: server[0],
			},
			Icon: es.setServerIcon(server[0].ToString()),
		}

		return []*Exchange{exchng}
	}

	exchangeServers := es.WL.AssetsManager.InstantSwap.ExchangeServers()

	var exchange []*Exchange
	for _, exchangeServer := range exchangeServers {
		exchng := &Exchange{
			Name:   exchangeServer.Server.CapFirstLetter(),
			Server: exchangeServer,
			Icon:   es.setServerIcon(exchangeServer.Server.ToString()),
		}

		exchange = append(exchange, exchng)
	}

	return exchange
}

func (es *ExchangeSelector) setServerIcon(serverName string) *cryptomaterial.Image {
	switch serverName {
	case instantswap.Changelly.ToString():
		return es.Theme.Icons.ChangellyIcon
	case instantswap.ChangeNow.ToString():
		return es.Theme.Icons.ChangeNowIcon
	case instantswap.CoinSwitch.ToString():
		return es.Theme.Icons.CoinSwitchIcon
	case instantswap.FlypMe.ToString():
		return es.Theme.Icons.FlypMeIcon
	case instantswap.GoDex.ToString():
		return es.Theme.Icons.GodexIcon
	case instantswap.SimpleSwap.ToString():
		return es.Theme.Icons.SimpleSwapIcon
	case instantswap.SwapZone.ToString():
		return es.Theme.Icons.SwapzoneIcon
	default:
		return es.Theme.Icons.AddExchange
	}
}

// SelectedExchange returns the currently selected Exchange.
func (es *ExchangeSelector) SelectedExchange() *Exchange {
	return es.selectedExchange
}

// SetSelectedExchange sets exch as the current selected exchange.
func (es *ExchangeSelector) SetSelectedExchange(exch *Exchange) {
	es.selectedExchange = exch
}

// Title Sets the title of the exchange list dialog.
func (es *ExchangeSelector) Title(title string) *ExchangeSelector {
	es.dialogTitle = title
	return es
}

// ExchangeSelected sets the callback executed when an exchange is selected.
func (es *ExchangeSelector) ExchangeSelected(callback func(*Exchange)) *ExchangeSelector {
	es.exchangeCallback = callback
	return es
}

// SetSelectedExchangeName sets the exchange whose Name field is
// equals to {name} as the current selected exchange.
// If it can find exchange whose Name field equals name it returns silently.
func (es *ExchangeSelector) SetSelectedExchangeName(name string) {
	ex := es.SupportedExchanges()
	for _, v := range ex {
		if v.Name == name {
			es.SetSelectedExchange(v)
			return
		}
	}
}

func (es *ExchangeSelector) Handle(window app.WindowNavigator) {
	for es.openSelectorDialog.Clicked() {
		es.title(es.dialogTitle)
		window.ShowModal(es.exchangeModal)
	}
}

func (es *ExchangeSelector) Layout(window app.WindowNavigator, gtx C) D {
	es.Handle(window)

	bg := es.Theme.Color.White
	if es.WL.AssetsManager.IsDarkModeOn() {
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
		Modal:        l.Theme.ModalFloatTitle(values.String(values.StrSelectAServer)),
		exchangeList: layout.List{Axis: layout.Vertical},
		isCancelable: true,
		dialogTitle:  values.String(values.StrSelectAServer),
	}

	em.Modal.ShowScrollbar(true)
	return em
}

func (es *ExchangeSelector) buildExchangeItems(server ...instantswap.Server) []*exchangeItem {
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

func (em *exchangeModal) Handle() {
	if em.eventQueue != nil {
		for _, exchangeItem := range em.exchangeItems {
			for exchangeItem.clickable.Clicked() {
				em.onExchangeClicked(exchangeItem.item)
				em.Dismiss()
			}
		}
	}

	if em.Modal.BackdropClicked(em.isCancelable) {
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
	em.eventQueue = gtx
	w := []layout.Widget{
		func(gtx C) D {
			titleTxt := em.Theme.Label(values.TextSize20, em.dialogTitle)
			titleTxt.Color = em.Theme.Color.Text
			titleTxt.Font.Weight = text.SemiBold
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
			exchName.Font.Weight = text.Normal
			return exchName.Layout(gtx)
		}),
	)
}

func (em *exchangeModal) OnDismiss() {}
