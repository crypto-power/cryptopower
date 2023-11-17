package account

import (
	"gioui.org/layout"
	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/ui/load"
)

const AccountID = "Account"

type (
	C = layout.Context
	D = layout.Dimensions
)

type AccountWallet struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal
}

func NewAccountPage(l *load.Load) *AccountWallet {
	pg := &AccountWallet{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(AccountID),
	}

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *AccountWallet) OnNavigatedTo() {}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *AccountWallet) Layout(gtx C) D {
	return D{}
}
