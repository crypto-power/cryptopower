package dcr

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"decred.org/dcrwallet/v5/wallet"
	"github.com/crypto-power/cryptopower/libwallet/internal/loader"
	"github.com/decred/dcrd/chaincfg/v3"
)

func TestWalletLifecycle(t *testing.T) {
	for _, driver := range []string{"bdb", "badgerdb"} {
		t.Run(driver, func(t *testing.T) {
			ctx := context.Background()
			l := NewLoader(&LoaderConf{
				ChainParams:     chaincfg.SimNetParams(),
				DBDirPath:       t.TempDir(),
				StakeOptions:    &StakeOptions{},
				GapLimit:        20,
				AccountGapLimit: wallet.DefaultAccountGapLimit,
				DBMutex:         new(sync.Mutex),
			})
			l.SetDatabaseDriver(driver)
			t.Cleanup(func() {
				if _, loaded := l.GetLoadedWallet(); loaded {
					if err := l.UnloadWallet(); err != nil {
						t.Error(err)
					}
				}
			})

			pubPass := []byte("public")
			loaded, err := l.CreateNewWallet(ctx, &loader.CreateWalletParams{
				WalletID:       "spending",
				PubPassphrase:  pubPass,
				PrivPassphrase: []byte("private"),
				Seed:           bytes.Repeat([]byte{1}, 32),
			})
			if err != nil {
				t.Fatal(err)
			}
			xpub, err := loaded.DCR.AccountXpub(ctx, 0)
			if err != nil {
				t.Fatal(err)
			}
			wantXpub := xpub.String()

			checkWallet := func(w *wallet.Wallet, watchingOnly bool) {
				t.Helper()
				if w.WatchingOnly() != watchingOnly {
					t.Fatalf("WatchingOnly = %v, want %v", w.WatchingOnly(), watchingOnly)
				}
				if !w.MixingEnabled() {
					t.Fatal("mixing must remain enabled after the v5 migration")
				}
				xpub, err := w.AccountXpub(ctx, 0)
				if err != nil {
					t.Fatal(err)
				}
				if xpub.String() != wantXpub {
					t.Fatal("account public key changed")
				}
			}
			checkWallet(loaded.DCR, false)

			for _, watchingOnly := range []bool{false, true} {
				walletID := "spending"
				if watchingOnly {
					if err := l.UnloadWallet(); err != nil {
						t.Fatal(err)
					}
					walletID = "watching"
					loaded, err = l.CreateWatchingOnlyWallet(ctx, &loader.WatchOnlyWalletParams{
						WalletID:       walletID,
						ExtendedPubKey: wantXpub,
						PubPassphrase:  pubPass,
					})
					if err != nil {
						t.Fatal(err)
					}
					checkWallet(loaded.DCR, true)
				}
				if err := l.UnloadWallet(); err != nil {
					t.Fatal(err)
				}
				loaded, err = l.OpenExistingWallet(ctx, walletID, pubPass)
				if err != nil {
					t.Fatal(err)
				}
				checkWallet(loaded.DCR, watchingOnly)
			}
		})
	}
}
