package cryptomaterial

import (
	"gioui.org/widget"

	"golang.org/x/exp/shiny/materialdesign/icons"

	"github.com/crypto-power/cryptopower/ui/assets"
)

type Icons struct {
	ContentAdd, NavigationCheck, NavigationMore, ActionCheckCircle, ActionInfo, NavigationArrowBack,
	NavigationArrowForward, ActionCheck, NavigationCancel, NavMoreIcon,
	DotIcon, ContentClear, DropDownIcon, Cached, ContentRemove, SearchIcon, PlayIcon,
	ActionSettings, ActionSwapHoriz, ActionSwapVertical, NavigationRefresh, ContentCopy, MenuIcon, CopyIcon, ArrowDropDown, ArrowDropUp,
	ChevronLeft, ChevronRight, ChevronUp, ChevronDown, DeleteIcon, VisibilityIcon, VisibilityOffIcon *widget.Icon

	OverviewIcon, OverviewIconInactive, WalletIcon, WalletIconInactive, TradeIconActive, TradeIconInactive, RedAlert, AlertIcon,
	ReceiveIcon, Transferred, TransactionsIcon, TransactionsIconInactive, SendIcon,
	PendingIcon, Logo, RedirectIcon, ConfirmIcon, NewWalletIcon, ArrowForward, AccountIcon,
	EditIcon, expandIcon, MixedTx, Mixer, SettingsIcon,
	ArrowDownIcon, SyncingIcon, TransactionFingerprint, DocumentationIcon, TimerIcon, StakeIcon, StakeIconInactive, StakeyIcon,
	GovernanceActiveIcon, GovernanceInactiveIcon, LogoDarkMode, TimerDarkMode, Rebroadcast, Notification, SuccessIcon, FailedIcon,
	MixedTxIcon, UnmixedTxIcon, MixerIcon, NotSynced, InfoAction, LightMode, DarkMode, AddExchange, FlypMeIcon, ChangellyIcon,
	SimpleSwapIcon, SwapzoneIcon, ShapeShiftIcon, GodexIcon, CoinSwitchIcon, ChangeNowIcon, TrocadorIcon,
	LTCBackground, LTCGroupIcon, DCRBackground, LogoDCRSlide, BTCBackground, BTCGroupIcon, CrossPlatformIcon,
	IntegratedExchangeIcon, MultiWalletIcon, Dot, TradeExchangeIcon, FilterImgIcon, FilterOffImgIcon, ShareIcon,
	CircleBTC, CircleLTC, CircleDCR, TelegramIcon, MatrixIcon, WebsiteIcon, TwitterIcon, OrangeAlert, ImportedAccountIcon *Image

	TicketImmatureIcon,
	TicketLiveIcon,
	TicketVotedIcon,
	TicketMissedIcon,
	TicketExpiredIcon,
	TicketRevokedIcon,
	TicketUnminedIcon *Image

	BTC, DCR, DCRBlue, LTC, DcrWatchOnly, BtcWatchOnly, LtcWatchOnly, DcrDex *Image
	AppIcon                                                                  *Image
}

// TODO: Deprecate some of these standard icons eg ActionInfo
func (i *Icons) StandardMaterialIcons() *Icons {
	i.ContentAdd = MustIcon(widget.NewIcon(icons.ContentAdd))
	i.NavigationCheck = MustIcon(widget.NewIcon(icons.NavigationCheck))
	i.NavigationMore = MustIcon(widget.NewIcon(icons.NavigationMoreHoriz))
	i.ActionCheckCircle = MustIcon(widget.NewIcon(icons.ActionCheckCircle))
	i.NavigationArrowBack = MustIcon(widget.NewIcon(icons.NavigationArrowBack))
	i.NavigationArrowForward = MustIcon(widget.NewIcon(icons.NavigationArrowForward))
	i.ActionInfo = MustIcon(widget.NewIcon(icons.ActionInfo))
	i.ActionCheck = MustIcon(widget.NewIcon(icons.ActionCheckCircle))
	i.NavigationCancel = MustIcon(widget.NewIcon(icons.NavigationCancel))
	i.DotIcon = MustIcon(widget.NewIcon(icons.ImageBrightness1))
	i.ContentClear = MustIcon(widget.NewIcon(icons.ContentClear))
	i.DropDownIcon = MustIcon(widget.NewIcon(icons.NavigationArrowDropDown))
	i.Cached = MustIcon(widget.NewIcon(icons.ActionCached))
	i.ContentRemove = MustIcon(widget.NewIcon(icons.ContentRemove))
	i.SearchIcon = MustIcon(widget.NewIcon(icons.ActionSearch))
	i.PlayIcon = MustIcon(widget.NewIcon(icons.AVPlayArrow))
	i.ActionSettings = MustIcon(widget.NewIcon(icons.ActionSettings))
	i.ActionSwapHoriz = MustIcon(widget.NewIcon(icons.ActionSwapHoriz))
	i.ActionSwapVertical = MustIcon(widget.NewIcon(icons.ActionSwapVert))
	i.NavigationRefresh = MustIcon(widget.NewIcon(icons.NavigationRefresh))
	i.ContentCopy = MustIcon(widget.NewIcon(icons.ContentContentPaste))
	i.MenuIcon = MustIcon(widget.NewIcon(icons.NavigationMenu))
	i.CopyIcon = MustIcon(widget.NewIcon(icons.ContentContentCopy))
	i.ArrowDropUp = MustIcon(widget.NewIcon(icons.NavigationArrowDropUp))
	i.ArrowDropDown = MustIcon(widget.NewIcon(icons.NavigationArrowDropDown))
	i.ChevronLeft = MustIcon(widget.NewIcon(icons.NavigationChevronLeft))
	i.ChevronRight = MustIcon(widget.NewIcon(icons.NavigationChevronRight))
	i.ChevronUp = MustIcon(widget.NewIcon(icons.HardwareKeyboardArrowUp))
	i.ChevronDown = MustIcon(widget.NewIcon(icons.HardwareKeyboardArrowDown))
	i.DeleteIcon = MustIcon(widget.NewIcon(icons.ActionDelete))
	i.VisibilityIcon = MustIcon(widget.NewIcon(icons.ActionVisibility))
	i.VisibilityOffIcon = MustIcon(widget.NewIcon(icons.ActionVisibilityOff))
	return i
}

func (i *Icons) DefaultIcons() *Icons {
	decredIcons := assets.DecredIcons

	i.StandardMaterialIcons()
	i.AppIcon = NewImage(decredIcons["appicon"])
	i.OverviewIcon = NewImage(decredIcons["ic_overview"])
	i.OverviewIconInactive = NewImage(decredIcons["ic_overview_inactive"])
	i.WalletIconInactive = NewImage(decredIcons["ic_wallet_inactive"])
	i.ReceiveIcon = NewImage(decredIcons["ic_receive"])
	i.Transferred = NewImage(decredIcons["ic_transferred"])
	i.TransactionsIcon = NewImage(decredIcons["ic_transactions"])
	i.TransactionsIconInactive = NewImage(decredIcons["ic_tx_inactive"])
	i.SendIcon = NewImage(decredIcons["ic_send"])
	i.Logo = NewImage(decredIcons["logo_decred"])
	i.ConfirmIcon = NewImage(decredIcons["ic_confirmed"])
	i.PendingIcon = NewImage(decredIcons["pending"])
	i.RedirectIcon = NewImage(decredIcons["ic_redirect"])
	i.NewWalletIcon = NewImage(decredIcons["ic_add_wallet"])
	i.AccountIcon = NewImage(decredIcons["ic_account"])
	i.EditIcon = NewImage(decredIcons["ic_editIcon"])
	i.expandIcon = NewImage(decredIcons["ic_expand"])
	i.MixedTx = NewImage(decredIcons["ic_mixed_tx"])
	i.Mixer = NewImage(decredIcons["ic_mixer"])
	i.SettingsIcon = NewImage(decredIcons["ic_settings"])
	i.ArrowDownIcon = NewImage(decredIcons["ic_arrow_down"])
	i.SyncingIcon = NewImage(decredIcons["ic_syncing"])
	i.DocumentationIcon = NewImage(decredIcons["ic_document"])
	i.TimerIcon = NewImage(decredIcons["ic_timer"])
	i.WalletIcon = NewImage(decredIcons["ic_wallet"])
	i.TradeIconActive = NewImage(decredIcons["ic_trade_active"])
	i.TradeIconInactive = NewImage(decredIcons["ic_trade_inactive"])
	i.StakeIcon = NewImage(decredIcons["ic_stake"])
	i.StakeIconInactive = NewImage(decredIcons["ic_stake_inactive"])
	i.StakeyIcon = NewImage(decredIcons["ic_stakey"])
	i.TicketImmatureIcon = NewImage(decredIcons["ic_ticket_immature"])
	i.TicketUnminedIcon = NewImage(decredIcons["ic_ticket_unmined"])
	i.TicketLiveIcon = NewImage(decredIcons["ic_ticket_live"])
	i.TicketVotedIcon = NewImage(decredIcons["ic_ticket_voted"])
	i.TicketMissedIcon = NewImage(decredIcons["ic_ticket_missed"])
	i.TicketExpiredIcon = NewImage(decredIcons["ic_ticket_expired"])
	i.TicketRevokedIcon = NewImage(decredIcons["ic_ticket_revoked"])
	i.GovernanceActiveIcon = NewImage(decredIcons["ic_governance_active"])
	i.GovernanceInactiveIcon = NewImage(decredIcons["ic_governance_inactive"])
	i.Rebroadcast = NewImage(decredIcons["ic_rebroadcast"])
	i.AddExchange = NewImage(decredIcons["ic_add_exchange"])
	i.TradeExchangeIcon = NewImage(decredIcons["ic_trade_exchange"])
	i.TransactionFingerprint = NewImage(decredIcons["ic_tx_fingerprint"])
	i.ArrowForward = NewImage(decredIcons["ic_arrow_fwd"])
	i.AlertIcon = NewImage(decredIcons["ic_alert"])

	/* Start - Asset types Icons */
	i.DcrDex = NewImage(decredIcons["logo_dcrdex"])
	i.BTC = NewImage(decredIcons["logo_btc"])
	i.DCR = NewImage(decredIcons["logo_dcr"])
	i.DCRBlue = NewImage(decredIcons["logo_dcr_blue"])
	i.LTC = NewImage(decredIcons["logo_ltc"])
	i.DcrWatchOnly = NewImage(decredIcons["logo_dcr_watch_only"])
	i.BtcWatchOnly = NewImage(decredIcons["logo_btc_watch_only"])
	i.LtcWatchOnly = NewImage(decredIcons["logo_ltc_watch_only"])
	i.ImportedAccountIcon = NewImage(decredIcons["ic_imported_account"])
	i.DCRBackground = NewImage(decredIcons["bg_dcr"])
	i.LogoDCRSlide = NewImage(decredIcons["logo_dcr_slide"])
	i.LTCBackground = NewImage(decredIcons["bg_ltc"])
	i.LTCGroupIcon = NewImage(decredIcons["logo_ltc_blue"])
	i.BTCBackground = NewImage(decredIcons["bg_btc"])
	i.BTCGroupIcon = NewImage(decredIcons["logo_btc_yellow"])
	/* End - Asset types Icons */

	i.SuccessIcon = NewImage(decredIcons["ic_success_check"])
	i.FailedIcon = NewImage(decredIcons["ic_failed"])
	i.RedAlert = NewImage(decredIcons["ic_red_alert"])
	i.OrangeAlert = NewImage(decredIcons["ic_orange_alert"])
	i.NotSynced = NewImage(decredIcons["ic_not_synced"])
	i.UnmixedTxIcon = NewImage(decredIcons["ic_unmixed"])
	i.MixedTxIcon = NewImage(decredIcons["ic_mixed"])
	i.MixerIcon = NewImage(decredIcons["ic_mixer_2"])
	i.InfoAction = NewImage(decredIcons["ic_info"])
	i.DarkMode = NewImage(decredIcons["ic_moon"])
	i.LightMode = NewImage(decredIcons["ic_light_mode"])
	i.Notification = NewImage(decredIcons["ic_notification"])
	i.Dot = NewImage(decredIcons["ic_dot"])

	/* Start exchange icons */
	i.FlypMeIcon = NewImage(decredIcons["logo_flypme"])
	i.ChangellyIcon = NewImage(decredIcons["logo_changelly"])
	i.SimpleSwapIcon = NewImage(decredIcons["ic_simpleswap"])
	i.SwapzoneIcon = NewImage(decredIcons["ic_swapzone"])
	i.ShapeShiftIcon = NewImage(decredIcons["logo_shapeshift"])
	i.GodexIcon = NewImage(decredIcons["ic_godex"])
	i.CoinSwitchIcon = NewImage(decredIcons["ic_coinswitch"])
	i.ChangeNowIcon = NewImage(decredIcons["ic_changenow"])
	i.TrocadorIcon = NewImage(decredIcons["ic_trocador"])
	/* End exchange icons */

	i.MultiWalletIcon = NewImage(decredIcons["ic_multi_wallet"])
	i.IntegratedExchangeIcon = NewImage(decredIcons["ic_integrated_exchange"])
	i.CrossPlatformIcon = NewImage(decredIcons["ic_cross_platform"])
	i.FilterImgIcon = NewImage(decredIcons["ic_filter"])
	i.FilterOffImgIcon = NewImage(decredIcons["ic_filter_off"])
	i.ShareIcon = NewImage(decredIcons["ic_share"])
	i.CircleBTC = NewImage(decredIcons["logo_btc_white"])
	i.CircleDCR = NewImage(decredIcons["logo_dcr_white"])
	i.CircleLTC = NewImage(decredIcons["logo_ltc_white"])
	i.TelegramIcon = NewImage(decredIcons["ic_telegram"])
	i.MatrixIcon = NewImage(decredIcons["ic_matrix"])
	i.WebsiteIcon = NewImage(decredIcons["ic_www"])
	i.TwitterIcon = NewImage(decredIcons["logo_twitter"])

	return i
}

func (i *Icons) DarkModeIcons() *Icons {
	decredIcons := assets.DecredIcons

	i.OverviewIcon = NewImage(decredIcons["dm_overview"])
	i.OverviewIconInactive = NewImage(decredIcons["dm_overview_inactive"])
	i.WalletIconInactive = NewImage(decredIcons["dm_wallet_inactive"])
	i.TransactionsIcon = NewImage(decredIcons["dm_transactions"])
	i.TransactionsIconInactive = NewImage(decredIcons["dm_tx_inactive"])
	i.Logo = NewImage(decredIcons["logo_dm_decred"])
	i.RedirectIcon = NewImage(decredIcons["dm_redirect"])
	i.NewWalletIcon = NewImage(decredIcons["dm_add_wallet"])
	i.AccountIcon = NewImage(decredIcons["dm_account"])
	i.EditIcon = NewImage(decredIcons["dm_editIcon"])
	i.Mixer = NewImage(decredIcons["dm_mixer"])
	i.SettingsIcon = NewImage(decredIcons["dm_settings"])
	i.ArrowDownIcon = NewImage(decredIcons["dm_arrow_down"])
	i.TimerIcon = NewImage(decredIcons["dm_timer"])
	i.WalletIcon = NewImage(decredIcons["dm_wallet"])
	i.StakeIcon = NewImage(decredIcons["dm_stake"])
	i.TicketRevokedIcon = NewImage(decredIcons["dm_ticket_revoked"])
	i.GovernanceActiveIcon = NewImage(decredIcons["dm_governance_active"])
	i.GovernanceInactiveIcon = NewImage(decredIcons["dm_governance_inactive"])
	i.Rebroadcast = NewImage(decredIcons["dm_rebroadcast"])
	i.TransactionFingerprint = NewImage(decredIcons["dm_transaction_fingerprint"])
	i.ArrowForward = NewImage(decredIcons["dm_arrow_fwd"])
	i.Notification = NewImage(decredIcons["dm_notification"])
	i.TradeExchangeIcon = NewImage(decredIcons["dm_trade_exchange"])
	i.WebsiteIcon = NewImage(decredIcons["dm_www"])
	i.MatrixIcon = NewImage(decredIcons["dm_matrix"])
	i.ImportedAccountIcon = NewImage(decredIcons["dm_imported_account"])
	return i
}
