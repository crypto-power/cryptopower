sbox
====

[![Build Status](https://travis-ci.org/marcopeereboom/sbox.png?branch=master)](https://travis-ci.org/marcopeereboom/sbox)
[![ISC License](http://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](http://godoc.org/github.com/marcopeereboom/sbox)
[![Go Report Card](https://goreportcard.com/badge/github.com/marcopeereboom/sbox)](https://goreportcard.com/report/github.com/marcopeereboom/sbox)

## Sbox Overview

Sbox takes random data and encrypts it into a portable binary blob. The binary
blob has a header that encodes the random 24 byte nonce and it provides a
single 32 bit user settable field that can function as a tag to identify or
version data.

## License

sbox is licensed under the [copyfree](http://copyfree.org) ISC License.
