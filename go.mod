module github.com/crypto-power/cryptopower

go 1.19

require (
	decred.org/dcrdex v0.6.3
	decred.org/dcrwallet/v3 v3.0.1
	gioui.org v0.1.0
	github.com/JohannesKaufmann/html-to-markdown v1.2.1
	github.com/PuerkitoBio/goquery v1.6.1
	github.com/ararog/timeago v0.0.0-20160328174124-e9969cf18b8d
	github.com/asdine/storm v0.0.0-20190216191021-fe89819f6282
	github.com/btcsuite/btcd v0.23.4
	github.com/btcsuite/btcd/btcec/v2 v2.3.2
	github.com/btcsuite/btcd/btcutil v1.1.3
	github.com/btcsuite/btcd/chaincfg/chainhash v1.0.2
	github.com/btcsuite/btclog v0.0.0-20170628155309-84c8d2346e9f
	github.com/btcsuite/btcwallet v0.16.10-0.20230706223227-037580c66b74
	github.com/btcsuite/btcwallet/wallet/txauthor v1.3.3
	github.com/btcsuite/btcwallet/wallet/txrules v1.2.0
	github.com/btcsuite/btcwallet/wallet/txsizes v1.2.3
	github.com/btcsuite/btcwallet/walletdb v1.4.0
	github.com/btcsuite/btcwallet/wtxmgr v1.5.0
	github.com/crypto-power/instantswap v0.0.0-20231205171529-1a958b193aa4
	github.com/dcrlabs/neutrino-ltc v0.0.0-20221031001456-55ef06cefead
	github.com/decred/dcrd/addrmgr/v2 v2.0.2
	github.com/decred/dcrd/blockchain/stake/v5 v5.0.0
	github.com/decred/dcrd/chaincfg/chainhash v1.0.4
	github.com/decred/dcrd/chaincfg/v3 v3.2.0
	github.com/decred/dcrd/connmgr/v3 v3.1.1
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.2.0
	github.com/decred/dcrd/dcrutil/v4 v4.0.1
	github.com/decred/dcrd/hdkeychain/v3 v3.1.1
	github.com/decred/dcrd/rpc/jsonrpc/types/v4 v4.0.0
	github.com/decred/dcrd/txscript/v4 v4.1.0
	github.com/decred/dcrd/wire v1.6.0
	github.com/decred/dcrdata/v8 v8.0.0-20230617164141-fa4d8e1b4e8e
	github.com/decred/politeia v1.4.0
	github.com/decred/slog v1.2.0
	github.com/dgraph-io/badger v1.6.2
	github.com/gen2brain/beeep v0.0.0-20220402123239-6a3042f4b71a
	github.com/gomarkdown/markdown v0.0.0-20220817224203-2206187d3406
	github.com/gorilla/websocket v1.5.0
	github.com/jessevdk/go-flags v1.5.0
	github.com/jrick/logrotate v1.0.0
	github.com/kevinburke/nacl v0.0.0-20190829012316-f3ed23dbd7f8
	github.com/lightninglabs/neutrino v0.15.0
	github.com/ltcsuite/ltcd v0.22.1-beta.0.20230329025258-1ea035d2e665
	github.com/ltcsuite/ltcd/btcec/v2 v2.1.0
	github.com/ltcsuite/ltcd/ltcutil v1.1.0
	github.com/ltcsuite/ltcwallet v0.13.1
	github.com/ltcsuite/ltcwallet/wallet/txauthor v1.1.0
	github.com/ltcsuite/ltcwallet/wallet/txrules v1.2.0
	github.com/ltcsuite/ltcwallet/wallet/txsizes v1.1.0
	github.com/ltcsuite/ltcwallet/walletdb v1.3.5
	github.com/ltcsuite/ltcwallet/wtxmgr v1.5.0
	github.com/nxadm/tail v1.4.8
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.10.5
	github.com/yeqown/go-qrcode v1.5.1
	go.etcd.io/bbolt v1.3.8
	golang.org/x/crypto v0.11.0
	golang.org/x/exp/shiny v0.0.0-20220827204233-334a2380cb91
	golang.org/x/image v0.5.0
	golang.org/x/sync v0.1.0
	golang.org/x/text v0.11.0
)

require github.com/yeqown/go-qrcode/v2 v2.2.2 // indirect

require (
	decred.org/cspp/v2 v2.1.0 // indirect
	decred.org/dcrwallet v1.7.0 // indirect
	gioui.org/cpu v0.0.0-20210817075930-8d6a761490d2 // indirect
	gioui.org/shader v1.0.6 // indirect
	github.com/AndreasBriese/bbloom v0.0.0-20190825152654-46b345b51c96 // indirect
	github.com/DataDog/zstd v1.5.2 // indirect
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/VictoriaMetrics/fastcache v1.6.0 // indirect
	github.com/aead/siphash v1.0.1 // indirect
	github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412 // indirect
	github.com/andybalholm/cascadia v1.1.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/btcsuite/btcd/btcutil/psbt v1.1.8 // indirect
	github.com/btcsuite/go-socks v0.0.0-20170105172521-4720035b7bfd // indirect
	github.com/btcsuite/golangcrypto v0.0.0-20150304025918-53f62d9b43e8 // indirect
	github.com/btcsuite/websocket v0.0.0-20150119174127-31079b680792 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cockroachdb/errors v1.9.1 // indirect
	github.com/cockroachdb/logtags v0.0.0-20230118201751-21c54148d20b // indirect
	github.com/cockroachdb/pebble v0.0.0-20230209160836-829675f94811 // indirect
	github.com/cockroachdb/redact v1.1.3 // indirect
	github.com/companyzero/sntrup4591761 v0.0.0-20220309191932-9e0f3af2f07a // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dchest/blake2b v1.0.0 // indirect
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/dcrlabs/neutrino-bch v0.0.0-20221031001408-f296bfa9bd1c // indirect
	github.com/deckarep/golang-set/v2 v2.1.0 // indirect
	github.com/decred/base58 v1.0.5 // indirect
	github.com/decred/dcrd/blockchain/stake/v3 v3.0.0 // indirect
	github.com/decred/dcrd/blockchain/standalone/v2 v2.2.0 // indirect
	github.com/decred/dcrd/certgen v1.1.2 // indirect
	github.com/decred/dcrd/crypto/blake256 v1.0.1 // indirect
	github.com/decred/dcrd/crypto/ripemd160 v1.0.2 // indirect
	github.com/decred/dcrd/database/v2 v2.0.2 // indirect
	github.com/decred/dcrd/database/v3 v3.0.1 // indirect
	github.com/decred/dcrd/dcrec v1.0.1 // indirect
	github.com/decred/dcrd/dcrec/edwards/v2 v2.0.3 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v3 v3.0.0 // indirect
	github.com/decred/dcrd/dcrjson/v4 v4.0.1 // indirect
	github.com/decred/dcrd/dcrutil/v3 v3.0.0 // indirect
	github.com/decred/dcrd/gcs/v2 v2.1.0 // indirect
	github.com/decred/dcrd/gcs/v4 v4.0.0 // indirect
	github.com/decred/dcrd/lru v1.1.2 // indirect
	github.com/decred/dcrd/rpcclient/v8 v8.0.0 // indirect
	github.com/decred/dcrd/txscript/v3 v3.0.0 // indirect
	github.com/decred/dcrtime v0.0.0-20191018193024-8d8b4ef0458e // indirect
	github.com/decred/go-socks v1.1.0 // indirect
	github.com/decred/vspd/client/v2 v2.0.0 // indirect
	github.com/decred/vspd/types/v2 v2.0.0 // indirect
	github.com/dgraph-io/ristretto v0.0.2 // indirect
	github.com/dustin/go-humanize v1.0.1-0.20210705192016-249ff6c91207 // indirect
	github.com/edsrzf/mmap-go v1.0.0 // indirect
	github.com/ethereum/go-ethereum v1.11.5 // indirect
	github.com/fjl/memsize v0.0.0-20190710130421-bcb5799ab5e5 // indirect
	github.com/fogleman/gg v1.3.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/gballet/go-libpcsclite v0.0.0-20190607065134-2772fd86a8ff // indirect
	github.com/gcash/bchd v0.19.0 // indirect
	github.com/gcash/bchlog v0.0.0-20180913005452-b4f036f92fa6 // indirect
	github.com/gcash/bchutil v0.0.0-20210113190856-6ea28dff4000 // indirect
	github.com/gcash/bchwallet v0.10.0 // indirect
	github.com/gcash/bchwallet/walletdb v0.0.0-20210524114850-4837f9798568 // indirect
	github.com/gcash/neutrino v0.0.0-20210524114821-3b1878290cf9 // indirect
	github.com/getsentry/sentry-go v0.18.0 // indirect
	github.com/go-chi/chi/v5 v5.0.1 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/go-text/typesetting v0.0.0-20230602202114-9797aefac433 // indirect
	github.com/go-toast/toast v0.0.0-20190211030409-01e6764cf0a4 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gofrs/flock v0.8.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.3.0 // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.5-0.20220116011046-fa5810519dcb // indirect
	github.com/google/trillian v1.4.1 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gorilla/schema v1.1.0 // indirect
	github.com/h2non/go-is-svg v0.0.0-20160927212452-35e8c4b0612c // indirect
	github.com/hashicorp/go-bexpr v0.1.10 // indirect
	github.com/holiman/bloomfilter/v2 v2.0.3 // indirect
	github.com/holiman/uint256 v1.2.0 // indirect
	github.com/huin/goupnp v1.0.3 // indirect
	github.com/jackpal/go-nat-pmp v1.0.2 // indirect
	github.com/jrick/bitset v1.0.0 // indirect
	github.com/jrick/wsrpc/v2 v2.3.5 // indirect
	github.com/kkdai/bstream v1.0.0 // indirect
	github.com/klauspost/compress v1.15.15 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/lib/pq v1.10.4 // indirect
	github.com/lightninglabs/gozmq v0.0.0-20191113021534-d20a764486bf // indirect
	github.com/lightninglabs/neutrino/cache v1.1.0 // indirect
	github.com/lightningnetwork/lnd/clock v1.0.1 // indirect
	github.com/lightningnetwork/lnd/queue v1.0.1 // indirect
	github.com/lightningnetwork/lnd/ticker v1.0.0 // indirect
	github.com/lightningnetwork/lnd/tlv v1.0.2 // indirect
	github.com/ltcsuite/lnd/clock v0.0.0-20200822020009-1a001cbb895a // indirect
	github.com/ltcsuite/lnd/queue v1.0.3 // indirect
	github.com/ltcsuite/lnd/ticker v1.0.1 // indirect
	github.com/ltcsuite/ltcd/ltcutil/psbt v1.1.0-1 // indirect
	github.com/ltcsuite/neutrino v0.13.2 // indirect
	github.com/marcopeereboom/sbox v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.16 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/pointerstructure v1.2.0 // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/olekukonko/tablewriter v0.0.5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.39.0 // indirect
	github.com/prometheus/procfs v0.9.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/rogpeppe/go-internal v1.9.0 // indirect
	github.com/rs/cors v1.8.2 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/shirou/gopsutil v3.21.4-0.20210419000835-c7a38de76ee5+incompatible // indirect
	github.com/status-im/keycard-go v0.2.0 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20210819022825-2ae1ddf74ef7 // indirect
	github.com/tadvi/systray v0.0.0-20190226123456-11a2b8fa57af // indirect
	github.com/tklauser/go-sysconf v0.3.5 // indirect
	github.com/tklauser/numcpus v0.2.2 // indirect
	github.com/tyler-smith/go-bip39 v1.1.0 // indirect
	github.com/urfave/cli/v2 v2.17.2-0.20221006022127-8f469abc00aa // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	github.com/yeqown/go-qrcode/writer/standard v1.2.2
	github.com/yeqown/reedsolomon v1.0.0 // indirect
	github.com/zquestz/grab v0.0.0-20190224022517-abcee96e61b1 // indirect
	golang.org/x/exp v0.0.0-20230206171751-46f607a40771 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/sys v0.10.0 // indirect
	golang.org/x/term v0.10.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/natefinch/npipe.v2 v2.0.0-20160621034901-c1b8fa8bdcce // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	lukechampine.com/blake3 v1.2.1 // indirect
)

// Older versions of github.com/lib/pq are required by politeia (v1.9.0)
// and dcrdex (v1.10.3) but only v1.10.4 and above can be compiled for
// the android OS using gomobile. This replace can be removed once any
// of those projects update their github.com/lib/pq dependency.
replace github.com/lib/pq => github.com/lib/pq v1.10.4

// The newer version of github.com/btcsuite/btcwallet
// (v0.16.10-0.20230706223227-037580c66b74) is not backward compactible with the
// current version (v0.16.9) used by cryptopower. This temporary workaround is
// neccessary to work with dcrdex which has latest version of
// github.com/btcsuite/btcwallet.
replace github.com/btcsuite/btcwallet v0.16.10-0.20230706223227-037580c66b74 => github.com/btcsuite/btcwallet v0.16.9
