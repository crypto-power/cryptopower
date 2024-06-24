# Cryptopower Wallet for Mobile and Desktop

A cross-platform desktop wallet built with [gio](https://gioui.org/).

**Links**

* Website: https://cryptopower.dev
* App Store: https://apps.apple.com/ca/app/cryptopower-wallet/id6472668308
* Google Play: https://play.google.com/store/apps/details?id=com.dreacotdigital.cryptopower.mainnet
* APK: https://github.com/crypto-power/cryptopower/releases
* Linux: https://github.com/crypto-power/cryptopower/releases
* Windows: https://github.com/crypto-power/cryptopower/releases
* MacOS: https://github.com/crypto-power/cryptopower/releases

**Features**

- Multi-Coin support - Native SPV wallets for Decred, Bitcoin and Litecoin.
- Coin Control - It allows users to select exact addresses and outputs to be used in a transaction.
- CoinShuffle++ - CoinShuffle++ (CSPP) is a mixing protocol used to create Decred CoinJoin transactions.
- Staking -  This allows users to purchase tickets via DCR’s [PoS (Proof-Of-Stake) consensus](https://docs.decred.org/proof-of-stake/overview/) implementation.
- Governance - This allows users to vote on Decred Proposals, Consensus changes and Treasury Spendings.
- Set gap limit - Users can choose a custom gap limit for use with address discovery.
- Instant Exchange - Instant exchanging between assets using (Flyp.me, Godex, Changenow, Trocador).
- Fee Rates API - This allows for custom fee selection when sending BTC transactions.
- Transaction note - This allows for a note/description when sending transactions.
- DCRDEX has been implemented with current trading pairs; LTC/DCR & DCR/BTC. It is currently under beta testing.
- Users can now send transactions to multiple recipients at a go.
- Users can now export transactions for better record keeping.
- An improved User Interface for easier accessibility and use.
- Users can switch between mainnet and testnet right from the settings page

**Desktop App**
| <img src="https://github.com/crypto-power/cryptopower/assets/25265396/0e738538-6a1f-4a96-8f34-dd478c4c878a" width="500">|<img src="https://github.com/crypto-power/cryptopower/assets/25265396/72946cbc-39da-4ff8-90c0-d3975640c25a" width="500"> |
|-|-|

| <img src="https://github.com/crypto-power/cryptopower/assets/25265396/f799da72-3f15-44a3-acca-2f454d0aa4ae" width="500">|<img src="https://github.com/crypto-power/cryptopower/assets/25265396/e0e0468a-a5bf-4ea0-a2d6-458bff1d6fcd" width="500"> |
|-|-|

| <img src="https://github.com/crypto-power/cryptopower/assets/25265396/a1746066-ab3c-46b5-a768-f19f427f44b4" width="500">|<img src="https://github.com/crypto-power/cryptopower/assets/25265396/16decf32-7855-4b98-aac8-c55748b8e14d" width="500"> |
|-|-|

**Mobile App**
| <img src="https://github.com/crypto-power/cryptopower/assets/25265396/0749c556-f7be-45d3-af87-bfdaf132a694" height="450">|<img src="https://github.com/crypto-power/cryptopower/assets/25265396/b155f3c1-034b-4de2-8be4-1c0d0dd79765" height="450"> |<img src="https://github.com/crypto-power/cryptopower/assets/25265396/d329012a-ae07-4e2c-a043-e5f931aec141" height="450"> |
|-|-|-|

| <img src="https://github.com/crypto-power/cryptopower/assets/25265396/56446510-92a5-41ed-a56e-b1a83742e444" height="450">|<img src="https://github.com/crypto-power/cryptopower/assets/25265396/451c2457-2807-44dd-addb-c5d20699bebf" height="450"> |<img src="https://github.com/crypto-power/cryptopower/assets/25265396/0cfec467-4628-4e43-b209-e6ccbc8b1f3f" height="450"> |
|-|-|-|

## Building

* Set up instant exchange private key: copy file `./libwallet/instantswap/instant_example.json`
into `./libwallet/instantswap/instant.json`. Then replace with your own key. For now, 
the supported instant exchange are: [trocador](https://trocador.app/), 
[changenow](https://changenow.io/), [godex](https://godex.io/) and 
[flypme](https://flyp.me/) (flypme does not require a private key)

For taking the api keys. Please go to the instant exchange websites

Note: You need to have [Go 1.20](https://golang.org/dl/) or above to build.

Then `go build`.

### Linux

To build **cryptopower** on Linux these [gio dependencies](https://gioui.org/doc/install/linux) are required.

Arch Linux:
`pacman -S vulkan-headers libxkbcommon-x11`

## FreeBSD

To build **cryptopower** on FreeBSD you will need to `pkg install vulkan-headers` as root. This is a gio dependency.

## Running cryptopower

### General usage

By default, **cryptopower** runs on Mainnet network type. However, cryptopower can run on testnet by issuing commands on the terminal in the format:

```bash
cryptopower [options]
```

- Run `./cryptopower --network=testnet` to run cryptopower on the testnet network.
- Run `./cryptopower --network=dextest` to run cryptopower with the decred dex simnet harness.
- Run `cryptopower -h` or `cryptopower help` to get general information of commands and options that can be issued on the cli.
- Use `cryptopower <command> -h` or `cryptopower help <command>` to get detailed information about a command.

## Profiling

Cryptopower uses [pprof](https://github.com/google/pprof) for profiling. It creates a web server which you can use to save your profiles. To setup a profiling web server, run cryptopower with the --profile flag and pass a server port to it as an argument.

So, after running the build command above, run the command

`./cryptopower --profile=6060`

You should now have a local web server running on 127.0.0.1:6060.

To save a profile, you can simply use

`curl -O localhost:6060/debug/pprof/profile`

## Contributing

See [CONTRIBUTING.md](https://github.com/crypto-power/cryptopower/blob/master/.github/CONTRIBUTING.md)

## Other

Earlier experimental work with other user interface toolkits can be found at [godcr-old](https://github.com/raedahgroup/godcr-old).

## Bootstrappable Cryptopower Builds

The [reproduciblebuilds](https://github.com/crypto-power/cryptopower/tree/master/reproduciblebuilds) directory contains the files necessary to perform bootstrappable Cryptopower builds.

Bootstrappability furthers our binary security guarantees by allowing us to audit and reproduce our toolchain instead of blindly trusting binary downloads.

We achieve bootstrappability by using Docker.
