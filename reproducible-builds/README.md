# Bootstrappable Cryptopower Builds

This directory contains the files necessary to perform bootstrappable cryptopower builds.

Bootstrappability furthers our binary security guarantees by allowing us to audit and reproduce our toolchain instead of blindly trusting binary downloads.

We achieve bootstrappability by using Docker.

## Usage

Note: You need to have [Docker](https://www.docker.com/) to build.

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