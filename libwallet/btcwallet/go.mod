module gitlab.com/raedah/libwallet/btcwallet

require (
	github.com/btcsuite/btcd v0.22.0-beta.0.20211026140004-31791ba4dc6e
	github.com/btcsuite/btclog v0.0.0-20170628155309-84c8d2346e9f
	github.com/btcsuite/btcutil v1.0.3-0.20210527170813-e2ba6805a890 // note: hoists btcd's own require of btcutil
	github.com/btcsuite/btcwallet v0.12.0
	github.com/btcsuite/btcwallet/wallet/txauthor v1.1.0 // indirect
	github.com/btcsuite/btcwallet/wallet/txsizes v1.1.0 // indirect
	github.com/btcsuite/btcwallet/walletdb v1.4.0
	github.com/btcsuite/btcwallet/wtxmgr v1.3.0
	github.com/decred/dcrd/lru v1.1.1 // indirect
	github.com/decred/slog v1.2.0
	github.com/jrick/logrotate v1.0.0
	github.com/kkdai/bstream v1.0.0 // indirect
	github.com/lightninglabs/neutrino v0.13.1-0.20211214231330-53b628ce1756
	github.com/stretchr/testify v1.7.0 // indirect
	go.etcd.io/bbolt v1.3.5 // indirect
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97 // indirect
	golang.org/x/sys v0.0.0-20210816183151-1e6c022a8912 // indirect
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d // indirect
)

go 1.16
