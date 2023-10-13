Cryptopower is a self-custodial multi-coin wallet application designed for both desktop and mobile platforms. It offers support for creating wallets not only for Decred, but also for other cryptocurrencies such as Bitcoin and Litecoin. The app is built using [Gio](https://gioui.org/), a Golang library for the implementation of cross-platform user interfaces.

## Available Features

- Multi-Coin support - Native SPV wallets for Decred, Bitcoin and Litecoin.
- Coin Control - It allows users to select exact addresses and outputs to be used in a transaction.
- CoinShuffle++ - CoinShuffle++ (CSPP) is a mixing protocol used to create Decred CoinJoin transactions.
- Staking - This allows users to purchase tickets via DCR’s [PoS (Proof-Of-Stake) consensus](https://docs.decred.org/proof-of-stake/overview/) implementation.
- Governance - This allows users to vote on Decred Proposals, Consensus changes and Treasury Spendings.
- Set gap limit - Users can choose a custom gap limit for use with address discovery.
- Instant Exchange - Instant exchanging between assets using (Flyp.me, Godex, Changenow).
- Fee Rates API - This allows for custom fee selection when sending BTC transactions.
- Transaction note - This allows for a note/description when sending transactions.

## Verifying the Release

In order to verify the release, you'll need to have `gpg` or `gpg2` installed on your system. Once you've obtained a copy (and hopefully verified that as well), you can proceed with importing the key that has signed this release (provided below) if you haven't done so already:

```
-----BEGIN PGP PUBLIC KEY BLOCK-----

mQGNBGTLayEBDADUoSIR+C+GNqp1RygG6fSijuvqlIQ8np/ij1x1Fzvprk4FxBIp
tLfDxdeCmjJ8OxojfVLRuW6hgxd2kQmGJBv2MfIMRXWpvGbQBttuprbrT2/BE2F/
f4CkyZCldFUS915isR7mkZ09u72rr4feHyr5Ff8e9GwKoYGq3SCD9KrdiPw2H8AB
TE8usZ6Eqh+O0cy9Lf3mOi54hwN1goVC2v6ixcle24MU30gC/NgYk2LfjmbQhBJ2
v+cRUtvDLxAIEOf2FLYv78WqVYIyCny3cohlMsi75egKoy+Hn/B3NCcHJi74b/Fu
QXffVPuM/g+cbV8j6q4ZbPwR19BdLNR9fKxPybsYndM+yFccIFUNIAvLjo0cndlu
4qTtU6opaYVgHSl/iek5jwTJlVT7c5Lrm9X2FJhdB6saefc5Rk1ctvfAsmDM+M49
Ho2Jd1EXnwk+6UdZEE2sVqox5lUaIKSCeOMMwUJt5IbIW6ocwMAu08OCei49MXEj
C9CptJAz6Xf+Xt8AEQEAAbQlQ3J5cHRvcG93ZXIgPHJlbGVhc2VAY3J5cHRvcG93
ZXIuZGV2PokB1AQTAQoAPhYhBFwmv+xsJGalKNVVHNBax09ol25SBQJky2shAhsD
BQkDwmcABQsJCAcCBhUKCQgLAgQWAgMBAh4BAheAAAoJENBax09ol25SOLYMAMsG
+yiCcikPuvdojzuCpGVXjFNYP2PpO9LswvvkALVfIf4fNgJWcNQKrvoYMmX5Ia6v
HXX3SHcDfWW/sw7TTRLkk+Tz00ZjsVdToAFGmVDHd8nVngyusgBuBw/oQWUKYnrU
+iwY7llbleLlUK+2yJgLtkjkCH4p91tocJfc3SsWC+IXxOy1n0lyCPeJsm6l1+kl
S4M0oborkzxJR6Fufj4q/cgBJhbpvU8fuu10Kugf+MXYO7XP2UyCAmJzfgoD+Huk
JJDlQJhBtioUaJ2jBBXEITzhNQ9DA/x+iUN7rQFNhKsMeczEErDmKucGrPSqQKiG
Hz7c20lVnrT5EGdEL1bBPzSY7SJKkzn0rgsAxPHFxZlSkY/AFhn+/py/0+Tk1Rjq
7ullWCiAtBpOtIySGv8wnJHNSXVRE3kU5vPOXeF0YVqhjgIyVK+1jiXErSRQaSn1
pkpeNavFQQ0CrJlEeKFWicvBkbSfuhDArDeyq8PMdGCaD2pvMEla5VvTi1A3PLkB
jQRky2shAQwAt01gHBk+g0UP5WUcJgL+FaC0k4Lpk7mvd3ZKuVLObyRyEgNnsTLS
sgEUNBbaQ+y5XTV7O6VsLWQRNUQiSvy3bOPh+jYellTaJA1XVdM/07dGrVuWmAY+
wAi8i/NIJurDXhasU8e6Hj/RnQzYVXweYgbuH31c/9RTaU3ndjEBBUE4jqImsYbs
zG5HxLo154CUzH88YsCf5ZN6Jzl3GLo+213Y0dGT7EHzDpxDtLPzJuo8lnOEqR4/
5RistNF1fVGl4zma5pUk8ZerRT/mOPm+xAbWXeR6nJEXNI69g3ixPLOdM44jqHbo
XIpB4FQWJJAtrOLJqM65CPDhWIGs6fvG2U7tv66XVvrUKTBOCfOceGyJEGyKWGX6
kDECEWFTOdnalsGhX3PUriejhbCNQbkoK6a9PcRIM/VaK++I9eaytz1lShB03Je2
uXBtzf3uTlc2e/2WZvMmBictoXEZ22WkJh8dDRaKcuxfjkft3DovbcNgP1Yl332b
oumpQa/FcsdpABEBAAGJAbwEGAEKACYWIQRcJr/sbCRmpSjVVRzQWsdPaJduUgUC
ZMtrIQIbDAUJA8JnAAAKCRDQWsdPaJduUi+dDAC3mm2EvHXUYxGPvYb6GqfqseDQ
4rba+jjD6TUaAJVitpTY3CoKKBrdEWRZPR5+WSS/Hx75uAVS84g+3AZza5O4f100
spZkQtmOxQ4lHVtSZgaj/DR1dgvunHF5bqbiIq1VNOvRGcXafxhKvCksM/aKST6f
lxYeC8RQAaK8QUJCLqGBR6aiCkFa4/+t95rUEEFrRuukYXm77cfhFlWfiDk5QB5a
gBjYVCa6INCaEw57dqB3CxBQloIZ4m7PY0ofYiKVD+PWidwztbfO4ee4MeaXVo6z
NcUVH+fzcCZGgDO1sTuDOzOAdx5uvPmpyzMXzDAe5VYgLoHrsjDhJWsB9YZZjKhx
/PN+YS3KGsrD8HMgNhAdn3/QkLD2OKl3/sW5kmuFmlywdLkocl8FmZlGyRd/XxVD
u374UjAHT4NkQzbNV9DPKFeWk8yQq/xVj7wSezndFb/+YuGyz5Skji7mKTuoCjWS
pawty5DgDQwzHHdhqGlT7Dnb/R9QMEyPvK12HaQ=
=dt9F
-----END PGP PUBLIC KEY BLOCK-----
```

Simply copy that entire text, save it as `key.txt`, and open a terminal.

In the terminal, use the cd command to navigate to the directory path where you’ve saved `key.txt`. Then use the `gpg --import` command.

(Note that Windows uses `\` for directory paths, but Linux/macOS use `/`)

```
$ cd /path/to/the/key
$ gpg --import < key.txt
```

Example output:

```
gpg: key D05AC74F68976E52: "Cryptopower <release@cryptopower.dev>" imported
gpg: Total number processed: 1
gpg:              unchanged: 1
```

You can then delete `key.txt`.

Once you have the required PGP keys, you can verify the release with:

`gpg --verify cryptopower-v1.0.0-manifest.txt.asc`

You should see the following if the verification was successful:

```
gpg: assuming signed data in 'cryptopower-v1.0.0-manifest.txt'
gpg: Signature made Thu 03 Aug 2023 17:08:06 WAT
gpg:                using RSA key 5C26BFEC6C2466A528D5551CD05AC74F68976E52
gpg:                issuer "release@cryptopower.dev"
gpg: Good signature from "Cryptopower <release@cryptopower.dev>" [ultimate]
```

That will verify the signature of the manifest file, which ensures integrity and authenticity of the archive you've downloaded locally containing the binaries. Next, depending on your operating system, you should then re-compute the `sha256` hash of the archive with `sha256sum <filename>`, compare it with the corresponding one in the manifest file, and ensure they match exactly.

## Code Contributors (alphabetical order):

- Amos Ezeme (@crux25)
- Kennedy Izuegbu (@dreacot)
- Migwi Ndung’u (@dmigwi)
