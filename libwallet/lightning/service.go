package lightning

import (
	"context"
	"fmt"
	"time"
)

type Service struct {
	daemon *Daemon
	client *Client
	config *ServiceConfig
}

// ServiceConfig represents the configuration of the lightning service.
type ServiceConfig struct {
	WorkingDir string
	Network    string
}

func NewService(config *ServiceConfig) (*Service, error) {
	dm := NewDaemon(config)
	return &Service{
		daemon: dm,
		config: config,
	}, nil
}

func (s *Service) Start() error {
	go func() {
		go s.daemon.Start()
		ticker := time.NewTicker(time.Second * 10)

		// Wait for the daemon to start
		// TODO: write a listener outside that doesn't block and doesn't use lightning client.
		time.Sleep(120 * time.Second)

		// TODO: This will fail because the daemon don't have a lightning wallet at this point, so admin macaroon
		// is not created. Wallet should be created before calling this function.
		cl, err := NewClient(buildClienConfig(s.config))
		if err != nil {
			log.Infof("Error creating client %+v \n", err)
		}
		s.client = cl
		// poll and fetch node information to ascertain connectivity.
		for {
			select {
			case <-ticker.C:
				if s.client != nil {
					state, err := s.client.Client.GetInfo(context.Background())
					if err != nil {
						log.Infof("Error getting state %s: \n", err)
						continue
					}
					fmt.Printf("State: %+v\n", state)
				}
			default:
				continue
			}
		}
	}()

	return nil
}
