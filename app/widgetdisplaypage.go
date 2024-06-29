package app

import (
	"gioui.org/layout"
)

const WidgetDisplayPageID = "widgetdisplaypage"

// WidgetDisplayPage is a page that takes a widget to layout and does nothing
// more than displaying the widget.
type WidgetDisplayPage struct {
	*GenericPageModal
	widget layout.Widget
}

func NewWidgetDisplayPage(widget layout.Widget) *WidgetDisplayPage {
	return &WidgetDisplayPage{
		GenericPageModal: NewGenericPageModal(WidgetDisplayPageID),
		widget:           widget,
	}
}

// OnNavigatedTo implements Page.
func (*WidgetDisplayPage) OnNavigatedTo() {}

// HandleUserInteractions implements Page.
func (*WidgetDisplayPage) HandleUserInteractions(_ layout.Context) {}

// Layout implements Page.
func (pg *WidgetDisplayPage) Layout(gtx layout.Context) layout.Dimensions {
	return pg.widget(gtx)
}

// OnNavigatedFrom implements Page.
func (*WidgetDisplayPage) OnNavigatedFrom() {}
