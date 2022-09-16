CoinShuffle++
=============

[![Build Status](https://github.com/decred/cspp/workflows/Build%20and%20Test/badge.svg)](https://github.com/decred/cspp/actions)
[![Doc](https://img.shields.io/badge/doc-reference-blue.svg)](https://pkg.go.dev/decred.org/cspp)

## Overview

This module provides client and server implementations to execute the
[CoinShuffle++](https://decred.org/research/ruffing2016.pdf) mixing protocol.
While intended to be used to create Decred CoinJoin transactions, the client and
server packages are generic enough to anonymously mix and join elements of any
group.

This implementation differs from the protocol described by the CoinShuffle++
paper in the following ways:

* DiceMix is replaced by [DiceMix Light](https://github.com/ElementsProject/dicemix/blob/master/doc/protocol.md).
  This simplifies the computational cost by mixing smaller random numbers and
  sorting results to reserve anonymous slot assignments in a traditional XOR
  DC-net.  It also enables scaling the protocol to mix very large messages
  without increasing the anonymization cost.

* Peer session keys are ephemeral.  This is a tradeoff made to reduce
  participant correlation observed by the server between multiple mixes with the
  understanding that it makes peer reputation and authentication systems more
  difficult to design.  These systems can still be built on top of ephemeral
  session keys, e.g. by signing a challenge with both the epheremal and a static
  identity key to demonstrate knowledge of both secret keys.

* A special server acts as the central communication point rather than all peers
  broadcasting all messages to every participant through a bulletin board or
  chat room.  The server is the only party that advances the protocol into the
  next state when timeouts occur.
  
* TLS (or another secure, authenticated channel) is required.  This simplifies
  the protocol by removing the need to use the session signing key to
  authenticate each individual message.  It also improves efficiency by allowing
  authentication to be performed through a MAC.

* All non-anonymous portions of the final result (in a CoinJoin, all inputs and
  the change outputs) are shared with the server at the start of the protocol.
  The server is responsible for merging these messages, and the mixed messages
  from the DC-net, into a single message that clients confirm.  With knowledge
  of how many inputs each peer is providing and their change output, a server
  can require each peer to pay their fair share of the transaction fee and check
  that inputs do not double spend.

## Privacy guarantees and caveats

CoinShuffle++ provides creation of transactions with anonymized mixing of
outputs with equally-sized amounts in a CoinJoin.  It does not anonymize
participation, transaction inputs, or change outputs.  It is possible for a mix
to be deanonymized in the future if participants reveal, maliciously or
inadvertently, which mixed messages were theirs.

It is not possible to apply CoinShuffle++ to mix outputs with differing output
amounts without aborting the protocol entirely upon run failure (rather than
assigning blame, removing misbehaving peers, and starting another run).  Doing
so would deanonymize the final outputs by associating each peer with an output
of the revealed amount.  Therefore, only equally-sized outputs may be mixed
anonymously.

## Build requirements

All client packages and server software from this repo requires Go 1.13 or
later.

In addition, the server software, found in the cmd/csppserver directory, depends
on Flint2 to efficiently factor polynomials.  This requires additional C
libraries and headers to be installed for the server to compile and link
against.  On Ubuntu 18.04:

  $ sudo apt-get install libflint-dev libmpfr-dev

## License

cspp is released under a permissive ISC license.
