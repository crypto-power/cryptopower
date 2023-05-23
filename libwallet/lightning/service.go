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
	"github.com/lightningnetwork/lnd/signal"
)

// Service represents the lightning service
type Service struct {
	sync.Mutex
	started         int32
	stopped         int32
	lightningClient lnrpc.LightningClient
	config          *ServiceConfig
	startTime       time.Time
	running         bool
	quitChan        chan struct{}
	wg              sync.WaitGroup
	interceptor     signal.Interceptor
}

// ServiceConfig represents the configuration of the lightning service
type ServiceConfig struct {
	WorkingDir string
	Network    string `long:"network"`
}

// NewService returns an initialized instance of lightning service.
func NewService(cfg *ServiceConfig) (*Service, error) {
	return &Service{
		config: cfg,
	}, nil
}

// Start is used to start the lightning network service.
func (s *Service) Start() error {
	if atomic.SwapInt32(&s.started, 1) == 1 {
		return errors.New("Service already started")
	}
	s.startTime = time.Now()

	if err := s.startService(); err != nil {
		return fmt.Errorf("failed to start lightning service: %v", err)
	}

	return nil
}

func (s *Service) startService() error {
	s.Lock()
	defer s.Unlock()
	if s.running {
		return errors.New("lightning service already running")
	}

	s.quitChan = make(chan struct{})
	//readyChan := make(chan interface{})

	s.wg.Add(1)
	//go d.notifyWhenReady(readyChan)
	s.running = true

	go func() {
		defer func() {
			defer s.wg.Done()
			go s.stopService()
		}()

		interceptor, err := signal.Intercept()
		if err != nil {
			fmt.Printf("failed to create signal interceptor %v", err)
			return
		}
		s.interceptor = interceptor

		lndConfig, err := s.createConfig(s.config.WorkingDir, interceptor)
		if err != nil {
			fmt.Printf("failed to create config %v", err)
		}

		implConfig := lndConfig.ImplementationConfig(interceptor)

		err = lnd.Main(lndConfig, lnd.ListenerCfg{}, implConfig, interceptor)
		if err != nil {
			fmt.Printf("lnd main function returned with error: %v", err)
		}
	}()
	return nil
}

func (s *Service) stopService() {
	s.Lock()
	defer s.Unlock()
	if !s.running {
		return
	}

	fmt.Println("Lightning service shutting down")
	close(s.quitChan)
	s.wg.Wait()
	s.running = false
}

// Stop is used to stop the lightning network service.
func (s *Service) Stop() error {
	if atomic.SwapInt32(&s.stopped, 1) == 0 {
		s.stopService()
	}
	s.wg.Wait()
	fmt.Println("lightning service shutdown successfully")
	return nil
}

func (s *Service) createConfig(workingDir string, interceptor signal.Interceptor) (*lnd.Config, error) {
	lndConfig := lnd.DefaultConfig()
	lndConfig.Bitcoin.Active = true
	lndConfig.Bitcoin.Node = "neutrino" // Use neutrino node
	if s.config.Network == "mainnet" {
		lndConfig.Bitcoin.MainNet = true
	} else if s.config.Network == "testnet" {
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
			fmt.Printf("Failed to parse config %v", err)
			return nil, err
		}
	}

	// This section should be moved to log file once that is added.s
	buildLogWriter := build.NewRotatingLogWriter()
	filename := workingDir + "/logs/bitcoin/" + s.config.Network + "/lnd.log"
	err := buildLogWriter.InitLogRotator(filename, 10, 3)
	if err != nil {
		fmt.Printf("Error initializing log %v", err)
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

	conf, err := lnd.ValidateConfig(cfg, interceptor, fileParser, flagParser)
	if err != nil {
		fmt.Printf("ValidateConfig returned with error: %v", err)
		return nil, err
	}
	return conf, nil
}
