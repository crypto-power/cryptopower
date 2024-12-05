package root

import (
	"context"
	"path"

	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/crypto-power/cryptopower/app"
	"github.com/crypto-power/cryptopower/libwallet"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/page/components"
	"github.com/crypto-power/cryptopower/ui/values"
)

const (
	MigratePageID = "MigratePage"
)

type MigrationPage struct {
	*app.GenericPageModal
	*load.Load
	ctx                  context.Context
	migrateButton        cryptomaterial.Button
	cancelButton         cryptomaterial.Button
	list                 *widget.List
	scroll               cryptomaterial.ListStyle
	walletMigrators      []*libwallet.WalletMigrator
	inputPasswordButtons []cryptomaterial.Button
}

func NewMigrationPage(ctx context.Context, l *load.Load) *MigrationPage {
	p := &MigrationPage{
		GenericPageModal: app.NewGenericPageModal(MigratePageID),
		Load:             l,
		ctx:              ctx,
		migrateButton:    l.Theme.Button(values.String(values.StrMigrateNow)),
		cancelButton:     l.Theme.OutlineButton(values.String(values.StrDoNotMigrate)),
		list:             &widget.List{List: layout.List{Axis: layout.Vertical, Alignment: layout.Middle}},
	}
	p.scroll = p.Theme.List(p.list)

	allWallet := p.AssetsManager.AllWallets()
	p.walletMigrators = make([]*libwallet.WalletMigrator, 0)
	for _, wallet := range allWallet {
		p.walletMigrators = append(p.walletMigrators, libwallet.NewWalletMigrator(wallet))
		p.inputPasswordButtons = append(p.inputPasswordButtons, p.Theme.Button(values.String(values.StrMigrateNow)))
	}
	return p
}

func (mp *MigrationPage) ID() string {
	return MigratePageID
}

func (mp *MigrationPage) OnNavigatedTo() {
}

func (mp *MigrationPage) OnNavigatedFrom() {
}

func (mp *MigrationPage) HandleUserInteractions(gtx C) {
	if mp.migrateButton.Clicked(gtx) {
		mp.AssetsManager.RemoveRootDir()
		newmgr, err := libwallet.NewAssetsManager(path.Dir(mp.AssetsManager.RootDir()), mp.AssetsManager.ParamLogDir(), mp.AssetsManager.NetType(), mp.AssetsManager.DEXTestAddr())
		if err != nil {
			log.Errorf("Error create new asset manager: %v", err)
		}

		for _, w := range mp.walletMigrators {
			err := w.Migrate(newmgr)
			if err != nil {
				log.Errorf("Error migrate wallet: %v", err)
			}
		}

		mp.AssetsManager = newmgr
		mp.ParentWindow().ClearStackAndDisplay(NewHomePage(mp.ctx, mp.Load))
	}

	if mp.cancelButton.Clicked(gtx) {
		mp.ParentWindow().ClearStackAndDisplay(NewHomePage(mp.ctx, mp.Load))
	}

	for i := range mp.inputPasswordButtons {
		w := mp.walletMigrators[i]
		if mp.inputPasswordButtons[i].Clicked(gtx) {

			if w.IsWatchingOnlyWallet() {
				inputSeedModal := modal.NewTextInputModal(mp.Load).
					Hint(values.String(values.StrInputSeed)).
					PositiveButtonStyle(mp.Load.Theme.Color.Primary, mp.Load.Theme.Color.InvText).
					SetPositiveButtonCallback(func(seed string, m *modal.TextInputModal) bool {
						err := w.SetSeed(seed)
						if err != nil {
							m.SetError(err.Error())
							return false
						}
						m.Dismiss()
						return true
					})
				inputSeedModal.Title(values.String(values.StrInputSeed)).
					SetPositiveButtonText(values.String(values.StrMigrateNow))

				mp.ParentWindow().ShowModal(inputSeedModal)
			} else {
				inputPasswordModal := modal.NewCreatePasswordModal(mp.Load).
					EnableName(false).
					EnableConfirmPassword(false).
					Title(values.String(values.StrInputPassword)).
					PasswordHint(values.String(values.StrInputPassword)).
					SetNegativeButtonText(values.String(values.StrCancel)).
					SetNegativeButtonCallback(func() {
					}).
					SetCancelable(false).
					SetPositiveButtonText(values.String(values.StrMigrateNow)).
					SetPositiveButtonCallback(func(_, password string, m *modal.CreatePasswordModal) bool {
						err := w.SetPrivatePassphrase(password)
						if err != nil {
							m.SetError(err.Error())
							return false
						}
						m.Dismiss()
						return true
					})
				mp.ParentWindow().ShowModal(inputPasswordModal)
			}
		}
	}
}

func (mp *MigrationPage) Layout(gtx C) D {
	body := func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.MatchParent,
			Height:      cryptomaterial.MatchParent,
			Orientation: layout.Vertical,
			Alignment:   layout.Middle,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return mp.headerLayout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				return mp.bodyLayout(gtx)
			}),
		)
	}
	return cryptomaterial.UniformPadding(gtx, body, mp.IsMobileView())
}

func (mp *MigrationPage) headerLayout(gtx C) D {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(mp.Load.Theme.SemiBoldLabelWithSize(mp.ConvertTextSize(values.TextSize20), values.String(values.StrMigrateWallet)).Layout),
			)
		}),
	)
}

func (mp *MigrationPage) bodyLayout(gtx C) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return mp.scroll.Layout(gtx, len(mp.walletMigrators), func(gtx C, i int) D {
				w := mp.walletMigrators[i]
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return cryptomaterial.LinearLayout{
							Orientation: layout.Vertical,
							Width:       cryptomaterial.MatchParent,
							Height:      cryptomaterial.WrapContent,
							Background:  mp.Theme.Color.Surface,
							Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
							Padding:     layout.UniformInset(values.MarginPadding15),
							Margin:      layout.Inset{Bottom: values.MarginPadding5},
						}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										return layout.Inset{Right: values.MarginPadding5}.Layout(gtx, components.CoinImageBySymbol(mp.Load, w.GetAssetType(), w.IsWatchingOnlyWallet()).Layout24dp)
									}),
									layout.Flexed(1, func(gtx C) D {
										return mp.Theme.Label(values.TextSize14, w.GetWalletName()).Layout(gtx)
									}),
									layout.Rigid(func(gtx C) D {
										if w.GetIsMigrate() {
											return mp.Theme.Body1(values.String(values.StrWillMigrate)).Layout(gtx)
										}
										return mp.inputPasswordButtons[i].Layout(gtx)
									}),
								)
							}),
						)
					}),
				)
			})
		}),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return mp.cancelButton.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					return mp.migrateButton.Layout(gtx)
				}),
			)
		}),
	)
}
