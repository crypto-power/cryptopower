# Bootstrappable Cryptopower Builds

This directory contains the files necessary to perform bootstrappable cryptopower builds.

Bootstrappability furthers our binary security guarantees by allowing us to audit and reproduce our toolchain instead of blindly trusting binary downloads.

We achieve bootstrappability by using Docker.

## Usage

Note: You need to have [Docker](https://www.docker.com/) to build.

in the cryptopower root directory and execute:

`./reproducible_builds.sh {target}`

select your target from [Build Targets](https://github.com/crypto-power/cryptopower/tree/master/reproduciblebuilds#build-targets)

e.g `./reproducible_builds.sh linux-amd64-binary`

the reproduced binary can be found in this directory

### Verification

You can then verify if the binaries are reproducible using a tool like [diffoscope](https://diffoscope.org/) or [reprotest](https://salsa.debian.org/reproducible-builds/reprotest)

#### Using Diffoscope

e.g `diffoscope binary1 binary2`

The 2 binaries should be the official cryptopower release binary which can be found on the [release page](https://github.com/crypto-power/cryptopower/releases) and the binary you reproduced which can be found in this directory

The output should be clean if the binaries are reproducible (DYOR).

You should get a clutter of outputs if the binaries are not reproducible

## Build Targets

### Linux

Linux amd64:
`sudo make linux-amd64-binary`

Linux arm64:
`sudo make linux-arm64-binary`

### Darwin

Darwin amd64:
`sudo make darwin-amd64-binary`

Darwin arm64:
`sudo make darwin-arm64-binary`

### Windows

Windows amd64:
`sudo make windows-amd64-binary`

Windows arm64:
`sudo make windows-386-binary`

### FreeBSD

FreeBSD amd64:
`sudo make freebsd-amd64-binary`

FreeBSD arm:
`sudo make freebsd-arm-binary`