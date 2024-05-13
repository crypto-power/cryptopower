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
	ActionSettings, ActionSwapHoriz, ActionSwapVertical, NavigationRefresh, ContentCopy *widget.Icon

	OverviewIcon, OverviewIconInactive, WalletIcon, WalletIconInactive, TradeIconActive, TradeIconInactive, MixerInactive, RedAlert, Alert,
	ReceiveIcon, Transferred, TransactionsIcon, TransactionsIconInactive, SendIcon, MoreIcon, MoreIconInactive,
	PendingIcon, Logo, RedirectIcon, ConfirmIcon, NewWalletIcon, WalletAlertIcon, ArrowForward, EllipseHoriz,
	ImportedAccountIcon, AccountIcon, EditIcon, expandIcon, CopyIcon, MixedTx, Mixer,
	Next, SettingsIcon, SecurityIcon, HelpIcon, AboutIcon, DebugIcon, VerifyMessageIcon, LocationPinIcon, SignMessageIcon, AlertGray,
	ArrowDownIcon, WatchOnlyWalletIcon, CurrencySwapIcon, SyncingIcon, TransactionFingerprint,
	Restore, DocumentationIcon, TimerIcon, StakeIcon, StakeIconInactive, StakeyIcon, EllipseVert,
	GovernanceActiveIcon, GovernanceInactiveIcon, LogoDarkMode, TimerDarkMode, Rebroadcast, Notification,
	SettingsActiveIcon, SettingsInactiveIcon, ActivatedActiveIcon, ActivatedInactiveIcon, LockinActiveIcon,
	LockinInactiveIcon, SuccessIcon, FailedIcon, ReceiveInactiveIcon, SendInactiveIcon, DarkmodeIcon,
	ChevronExpand, ChevronCollapse, ChevronLeft, MixedTxIcon, UnmixedTxIcon, MixerIcon, NotSynced, ConcealIcon,
	RevealIcon, InfoAction, LightMode, DarkMode, AddIcon, ChevronRight, AddExchange, FlypMeIcon, ChangellyIcon,
	SimpleSwapIcon, SwapzoneIcon, ShapeShiftIcon, GodexIcon, CoinSwitchIcon, ChangeNowIcon, TrocadorIcon, CaretUp, CaretDown,
	LTCBackground, LTCGroupIcon, DCRBackground, DCRGroupIcon, BTCBackground, BTCGroupIcon, CrossPlatformIcon,
	IntegratedExchangeIcon, MultiWalletIcon, Dot, TradeExchangeIcon, FilterImgIcon, FilterOffImgIcon, ShareIcon, DeleteIcon,
	CircleBTC, CircleLTC, CircleDCR, TelegramIcon, MatrixIcon, WebsiteIcon, TwitterIcon *Image

	NewStakeIcon,
	TicketImmatureIcon,
	TicketLiveIcon,
	TicketVotedIcon,
	TicketMissedIcon,
	TicketExpiredIcon,
	TicketRevokedIcon,
	TicketUnminedIcon *Image

	BTC, DCR, DCRBlue, LTC, BCH, DcrWatchOnly, BtcWatchOnly, LtcWatchOnly, BCHWatchOnly, DcrDex *Image
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

	return i
}

func (i *Icons) DefaultIcons() *Icons {
	decredIcons := assets.DecredIcons

	i.StandardMaterialIcons()
	i.OverviewIcon = NewImage(decredIcons["overview"])
	i.OverviewIconInactive = NewImage(decredIcons["overview_inactive"])
	i.WalletIconInactive = NewImage(decredIcons["wallet_inactive"])
	i.ReceiveIcon = NewImage(decredIcons["receive"])
	i.Transferred = NewImage(decredIcons["transferred"])
	i.TransactionsIcon = NewImage(decredIcons["transactions"])
	i.TransactionsIconInactive = NewImage(decredIcons["transactions_inactive"])
	i.SendIcon = NewImage(decredIcons["send"])
	i.MoreIcon = NewImage(decredIcons["more"])
	i.MoreIconInactive = NewImage(decredIcons["more_inactive"])
	i.Logo = NewImage(decredIcons["logo"])
	i.ConfirmIcon = NewImage(decredIcons["confirmed"])
	i.PendingIcon = NewImage(decredIcons["pending"])
	i.RedirectIcon = NewImage(decredIcons["redirect"])
	i.NewWalletIcon = NewImage(decredIcons["addNewWallet"])
	i.WalletAlertIcon = NewImage(decredIcons["walletAlert"])
	i.AccountIcon = NewImage(decredIcons["account"])
	i.ImportedAccountIcon = NewImage(decredIcons["imported_account"])
	i.EditIcon = NewImage(decredIcons["editIcon"])
	i.expandIcon = NewImage(decredIcons["expand_icon"])
	i.CopyIcon = NewImage(decredIcons["copy_icon"])
	i.MixedTx = NewImage(decredIcons["mixed_tx"])
	i.Mixer = NewImage(decredIcons["mixer"])
	i.Next = NewImage(decredIcons["ic_next"])
	i.SettingsIcon = NewImage(decredIcons["settings"])
	i.SecurityIcon = NewImage(decredIcons["security"])
	i.HelpIcon = NewImage(decredIcons["help_icon"])
	i.AboutIcon = NewImage(decredIcons["about_icon"])
	i.DebugIcon = NewImage(decredIcons["debug"])
	i.VerifyMessageIcon = NewImage(decredIcons["verify_message"])
	i.LocationPinIcon = NewImage(decredIcons["location_pin"])
	i.SignMessageIcon = NewImage(decredIcons["signMessage"])
	i.AlertGray = NewImage(decredIcons["alert_gray"])
	i.ArrowDownIcon = NewImage(decredIcons["arrow_down"])
	i.WatchOnlyWalletIcon = NewImage(decredIcons["watch_only_wallet"])
	i.CurrencySwapIcon = NewImage(decredIcons["swap"])
	i.SyncingIcon = NewImage(decredIcons["syncing"])
	i.DocumentationIcon = NewImage(decredIcons["documentation"])
	i.Restore = NewImage(decredIcons["restore"])
	i.TimerIcon = NewImage(decredIcons["timerIcon"])
	i.WalletIcon = NewImage(decredIcons["wallet"])
	i.TradeIconActive = NewImage(decredIcons["trade_active"])
	i.TradeIconInactive = NewImage(decredIcons["trade_inactive"])
	i.StakeIcon = NewImage(decredIcons["stake"])
	i.StakeIconInactive = NewImage(decredIcons["stake_inactive"])
	i.StakeyIcon = NewImage(decredIcons["stakey"])
	i.NewStakeIcon = NewImage(decredIcons["stake_purchased"])
	i.TicketImmatureIcon = NewImage(decredIcons["ticket_immature"])
	i.TicketUnminedIcon = NewImage(decredIcons["ticket_unmined"])
	i.TicketLiveIcon = NewImage(decredIcons["ticket_live"])
	i.TicketVotedIcon = NewImage(decredIcons["ic_ticket_voted"])
	i.TicketMissedIcon = NewImage(decredIcons["ticket_missed"])
	i.TicketExpiredIcon = NewImage(decredIcons["ticket_expired"])
	i.TicketRevokedIcon = NewImage(decredIcons["ticket_revoked"])
	i.GovernanceActiveIcon = NewImage(decredIcons["governance_active"])
	i.GovernanceInactiveIcon = NewImage(decredIcons["governance_inactive"])
	i.Rebroadcast = NewImage(decredIcons["rebroadcast"])
	i.ConcealIcon = NewImage(decredIcons["reveal"])
	i.RevealIcon = NewImage(decredIcons["hide"])
	i.AddExchange = NewImage(decredIcons["add_exchange"])
	i.TradeExchangeIcon = NewImage(decredIcons["trade_exchange_icon"])

	i.SettingsActiveIcon = NewImage(decredIcons["settings_active"])
	i.SettingsInactiveIcon = NewImage(decredIcons["settings_inactive"])
	i.ActivatedActiveIcon = NewImage(decredIcons["activated_active"])
	i.ActivatedInactiveIcon = NewImage(decredIcons["activated_inactive"])
	i.LockinActiveIcon = NewImage(decredIcons["lockin_active"])
	i.LockinInactiveIcon = NewImage(decredIcons["lockin_inactive"])
	i.TransactionFingerprint = NewImage(decredIcons["transaction_fingerprint"])
	i.ArrowForward = NewImage(decredIcons["arrow_fwd"])
	i.Alert = NewImage(decredIcons["alert"])

	/* Start - Asset types Icons */
	i.DcrDex = NewImage(decredIcons["logo_dcrdex"])
	i.BTC = NewImage(decredIcons["logo_btc"])
	i.DCR = NewImage(decredIcons["logo_dcr"])
	i.DCRBlue = NewImage(decredIcons["logo_dcr_blue"])
	i.LTC = NewImage(decredIcons["logo_ltc"])
	i.BCH = NewImage(decredIcons["logo_bch"])
	i.DcrWatchOnly = NewImage(decredIcons["logo_dcr_watch_only"])
	i.BtcWatchOnly = NewImage(decredIcons["logo_btc_watch_only"])
	i.LtcWatchOnly = NewImage(decredIcons["logo_ltc_watch_only"])
	i.BCHWatchOnly = NewImage(decredIcons["logo_bch_watch_only"])

	i.DCRBackground = NewImage(decredIcons["dcr_bg_image"])
	i.DCRGroupIcon = NewImage(decredIcons["dcrGroupImage"])
	i.LTCBackground = NewImage(decredIcons["ltc_bg_image"])
	i.LTCGroupIcon = NewImage(decredIcons["ltGroupImage"])
	i.BTCBackground = NewImage(decredIcons["btc_bg_image"])
	i.BTCGroupIcon = NewImage(decredIcons["btcGroupIcon"])
	/* End - Asset types Icons */

	i.SuccessIcon = NewImage(decredIcons["success_check"])
	i.FailedIcon = NewImage(decredIcons["crossmark_red"])
	i.ReceiveInactiveIcon = NewImage(decredIcons["receive_inactive"])
	i.SendInactiveIcon = NewImage(decredIcons["send_inactive"])
	i.DarkmodeIcon = NewImage(decredIcons["darkmodeIcon"])
	i.MixerInactive = NewImage(decredIcons["mixer_inactive"])
	i.RedAlert = NewImage(decredIcons["red_alert"])
	i.ChevronExpand = NewImage(decredIcons["chevron_expand"])
	i.ChevronCollapse = NewImage(decredIcons["coll_half"])
	i.ChevronRight = NewImage(decredIcons["chevron_coll"])
	i.ChevronLeft = NewImage(decredIcons["chevron_left"])
	i.CaretUp = NewImage(decredIcons["caret_up"])
	i.CaretDown = NewImage(decredIcons["caret_down"])
	i.NotSynced = NewImage(decredIcons["notSynced"])
	i.UnmixedTxIcon = NewImage(decredIcons["unmixed_icon"])
	i.MixedTxIcon = NewImage(decredIcons["mixed_icon"])
	i.MixerIcon = NewImage(decredIcons["mixer_icon"])
	i.InfoAction = NewImage(decredIcons["info_icon"])
	i.DarkMode = NewImage(decredIcons["ic_moon"])
	i.LightMode = NewImage(decredIcons["ic_sun"])
	i.AddIcon = NewImage(decredIcons["add_icon"])
	i.EllipseVert = NewImage(decredIcons["elipsis_vert"])
	i.EllipseHoriz = NewImage(decredIcons["elipsis"])
	i.Notification = NewImage(decredIcons["notification"])
	i.Dot = NewImage(decredIcons["dot"])

	/* Start exchange icons */
	i.FlypMeIcon = NewImage(decredIcons["flypme"])
	i.ChangellyIcon = NewImage(decredIcons["changelly"])
	i.SimpleSwapIcon = NewImage(decredIcons["simpleswap"])
	i.SwapzoneIcon = NewImage(decredIcons["swapzone"])
	i.ShapeShiftIcon = NewImage(decredIcons["shapeshift"])
	i.GodexIcon = NewImage(decredIcons["godex"])
	i.CoinSwitchIcon = NewImage(decredIcons["coinswitch"])
	i.ChangeNowIcon = NewImage(decredIcons["changenow"])
	i.TrocadorIcon = NewImage(decredIcons["trocador"])
	/* End exchange icons */

	i.MultiWalletIcon = NewImage(decredIcons["multiWalletIcon"])
	i.IntegratedExchangeIcon = NewImage(decredIcons["integratedExchangeIcon"])
	i.CrossPlatformIcon = NewImage(decredIcons["crossPlatformIcon"])
	i.FilterImgIcon = NewImage(decredIcons["ic_filter"])
	i.FilterOffImgIcon = NewImage(decredIcons["ic_filter_off"])
	i.ShareIcon = NewImage(decredIcons["ic_share"])
	i.DeleteIcon = NewImage(decredIcons["deleteIcon"])
	i.CircleBTC = NewImage(decredIcons["circle_btc_log"])
	i.CircleDCR = NewImage(decredIcons["circle_dcr_log"])
	i.CircleLTC = NewImage(decredIcons["circle_ltc_log"])
	i.TelegramIcon = NewImage(decredIcons["telegram"])
	i.MatrixIcon = NewImage(decredIcons["matrix"])
	i.WebsiteIcon = NewImage(decredIcons["www_icon"])
	i.TwitterIcon = NewImage(decredIcons["twitter"])

	return i
}

func (i *Icons) DarkModeIcons() *Icons {
	decredIcons := assets.DecredIcons

	i.OverviewIcon = NewImage(decredIcons["dm_overview"])
	i.OverviewIconInactive = NewImage(decredIcons["dm_overview_inactive"])
	i.WalletIconInactive = NewImage(decredIcons["dm_wallet_inactive"])
	i.TransactionsIcon = NewImage(decredIcons["dm_transactions"])
	i.TransactionsIconInactive = NewImage(decredIcons["dm_transactions_inactive"])
	i.MoreIcon = NewImage(decredIcons["dm_more"])
	i.MoreIconInactive = NewImage(decredIcons["dm_more_inactive"])
	i.Logo = NewImage(decredIcons["logo_darkmode"])
	i.RedirectIcon = NewImage(decredIcons["dm_redirect"])
	i.NewWalletIcon = NewImage(decredIcons["dm_addNewWallet"])
	i.WalletAlertIcon = NewImage(decredIcons["dm_walletAlert"])
	i.AccountIcon = NewImage(decredIcons["dm_account"])
	i.ImportedAccountIcon = NewImage(decredIcons["dm_imported_account"])
	i.EditIcon = NewImage(decredIcons["dm_editIcon"])
	i.CopyIcon = NewImage(decredIcons["dm_copy_icon"])
	i.Mixer = NewImage(decredIcons["dm_mixer"])
	i.Next = NewImage(decredIcons["dm_ic_next"])
	i.SettingsIcon = NewImage(decredIcons["dm_settings"])
	i.SecurityIcon = NewImage(decredIcons["dm_security"])
	i.HelpIcon = NewImage(decredIcons["dm_help_icon"])
	i.AboutIcon = NewImage(decredIcons["dm_info_icon"])
	i.DebugIcon = NewImage(decredIcons["dm_debug"])
	i.VerifyMessageIcon = NewImage(decredIcons["dm_verify_message"])
	i.LocationPinIcon = NewImage(decredIcons["dm_location_pin"])
	i.ArrowDownIcon = NewImage(decredIcons["dm_arrow_down"])
	i.WatchOnlyWalletIcon = NewImage(decredIcons["dm_watch_only_wallet"])
	i.CurrencySwapIcon = NewImage(decredIcons["dm_swap"])
	i.Restore = NewImage(decredIcons["dm_restore"])
	i.TimerIcon = NewImage(decredIcons["dm_timerIcon"])
	i.WalletIcon = NewImage(decredIcons["dm_wallet"])
	i.StakeIcon = NewImage(decredIcons["dm_stake"])
	i.TicketRevokedIcon = NewImage(decredIcons["dm_ticket_revoked"])
	i.GovernanceActiveIcon = NewImage(decredIcons["dm_governance_active"])
	i.GovernanceInactiveIcon = NewImage(decredIcons["dm_governance_inactive"])
	i.Rebroadcast = NewImage(decredIcons["dm_rebroadcast"])
	i.ActivatedActiveIcon = NewImage(decredIcons["dm_activated_active"])
	i.LockinActiveIcon = NewImage(decredIcons["dm_lockin_active"])
	i.TransactionFingerprint = NewImage(decredIcons["dm_transaction_fingerprint"])
	i.ArrowForward = NewImage(decredIcons["dm_arrow_fwd"])
	i.ChevronLeft = NewImage(decredIcons["chevron_left"])
	i.Notification = NewImage(decredIcons["dm_notification"])
	i.TradeExchangeIcon = NewImage(decredIcons["dm_trade_exchange_icon"])
	i.WebsiteIcon = NewImage(decredIcons["dm_www_icon"])
	i.MatrixIcon = NewImage(decredIcons["dm_matrix"])
	return i
}
