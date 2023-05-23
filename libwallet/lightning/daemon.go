package lightning

import (
	"errors"
	"fmt"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/lightningnetwork/lnd"
	"github.com/lightningnetwork/lnd/build"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/invoicesrpc"
	"github.com/lightningnetwork/lnd/signal"
)

// Daemon represents the lightning daemon
type Daemon struct {
	sync.Mutex
	started     int32
	stopped     int32
	config      *ServiceConfig
	startTime   time.Time
	running     bool
	quitChan    chan struct{}
	wg          sync.WaitGroup
	interceptor signal.Interceptor
}

// NewDaemon returns an initialized instance of lightning Daemon.
func NewDaemon(cfg *ServiceConfig) *Daemon {
	// Simulate build tags, to get around client enforcements
	buildTag := &build.RawTags
	*buildTag = "signrpc,walletrpc,routerrpc,chainrpc,invoicesrpc"
	return &Daemon{
		config: cfg,
	}
}

// Start is used to start the lightning network daemon.
func (d *Daemon) Start() error {
	if atomic.SwapInt32(&d.started, 1) == 1 {
		return errors.New("Daemon already started")
	}
	d.startTime = time.Now()

	if err := d.startDaemon(); err != nil {
		return fmt.Errorf("failed to start lightning Daemon: %v", err)
	}

	return nil
}

func (d *Daemon) startDaemon() error {
	d.Lock()
	defer d.Unlock()
	if d.running {
		return errors.New("lightning Daemon already running")
	}

	d.quitChan = make(chan struct{})

	d.wg.Add(1)
	d.running = true

	go func() {
		defer func() {
			defer d.wg.Done()
			go d.stopDaemon()
		}()

		interceptor, err := signal.Intercept()
		if err != nil {
			log.Infof("failed to create signal interceptor %v", err)
			return
		}
		d.interceptor = interceptor

		lndConfig, err := d.createConfig(d.config.WorkingDir, interceptor)
		if err != nil {
			log.Infof("failed to create config %v", err)
		}

		implConfig := lndConfig.ImplementationConfig(interceptor)

		err = lnd.Main(lndConfig, lnd.ListenerCfg{}, implConfig, interceptor)
		if err != nil {
			log.Infof("lnd main function returned with error: %v", err)
		}
	}()
	return nil
}

func (d *Daemon) stopDaemon() {
	d.Lock()
	defer d.Unlock()
	if !d.running {
		return
	}

	fmt.Println("Lightning Daemon shutting down")
	close(d.quitChan)
	d.wg.Wait()
	d.running = false
}

// Stop is used to stop the lightning network Daemon.
func (d *Daemon) Stop() error {
	if atomic.SwapInt32(&d.stopped, 1) == 0 {
		d.stopDaemon()
	}
	d.wg.Wait()
	log.Infof("lightning daemon shutdown successfully")
	return nil
}

// createConfig creates the daemon config configuration by parsing config file/cmd options or using defaults.
func (d *Daemon) createConfig(workingDir string, interceptor signal.Interceptor) (*lnd.Config, error) {
	lndConfig := lnd.DefaultConfig()
	lndConfig.Bitcoin.Active = true
	lndConfig.Bitcoin.Node = "neutrino" // Use neutrino node
	if d.config.Network == "mainnet" {
		lndConfig.Bitcoin.MainNet = true
	} else if d.config.Network == "testnet" {
		lndConfig.Bitcoin.TestNet3 = true
	} else {
		lndConfig.Bitcoin.SimNet = true
	}
	lndConfig.LndDir = workingDir
	lndConfig.ConfigFile = path.Join(workingDir, "lnd.conf")

	cfg := lndConfig
	// If a config file exists parse it.
	if lnrpc.FileExists(lndConfig.ConfigFile) {
		if err := flags.IniParse(lndConfig.ConfigFile, &cfg); err != nil {
			log.Infof("Failed to parse config %v", err)
			return nil, err
		}
	}

	// This section should be moved to log file once that is added.
	buildLogWriter := build.NewRotatingLogWriter()
	filename := workingDir + "/logs/bitcoin/" + d.config.Network + "/lnd.log"
	err := buildLogWriter.InitLogRotator(filename, 10, 3)
	if err != nil {
		log.Infof("Error initializing log %v", err)
		return nil, err
	}

	cfg.LogWriter = buildLogWriter
	cfg.MinBackoff = time.Second * 20
	cfg.TLSDisableAutofill = true

	fileParser := &flags.Parser{}
	if lnrpc.FileExists(lndConfig.ConfigFile) {
		fileParser := flags.NewParser(&cfg, flags.IgnoreUnknown)
		err = flags.NewIniParser(fileParser).ParseFile(lndConfig.ConfigFile)
		if err != nil {
			return nil, err
		}
	}

	// Finally, parse the remaining command line options again to ensure
	// they take precedence.
	flagParser := flags.NewParser(&cfg, flags.IgnoreUnknown)
	if _, err := flagParser.Parse(); err != nil {
		return nil, err
	}

	cfg.SubRPCServers.InvoicesRPC = &invoicesrpc.Config{}

	conf, err := lnd.ValidateConfig(cfg, interceptor, fileParser, flagParser)
	if err != nil {
		log.Infof("ValidateConfig returned with error: %v", err)
		return nil, err
	}
	return conf, nil
}
