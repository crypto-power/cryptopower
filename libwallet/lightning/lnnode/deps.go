package lnnode

import (
	"github.com/lightninglabs/neutrino"
	"github.com/lightningnetwork/lnd/channeldb"
)

/*
Dependencies is an implementation of interface daemon.Dependencies
used for injecting breez deps int LND running as library
*/
type Dependencies struct {
	workingDir   string
	chainService *neutrino.ChainService
	readyChan    chan interface{}
	chanDB       *channeldb.DB
}

/*
ReadyChan returns the channel passed to LND for getting ready signal
*/
func (d *Dependencies) ReadyChan() chan interface{} {
	return d.readyChan
}

/*
ChainService returns a neutrino.ChainService to be used from both sync job and
LND running daemon
*/
func (d *Dependencies) ChainService() *neutrino.ChainService {
	return d.chainService
}

/*
ChanDB returns the channel db for the daemon
*/
func (d *Dependencies) ChanDB() *channeldb.DB {
	return d.chanDB
}
