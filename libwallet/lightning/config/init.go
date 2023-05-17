package config

import (
	"path"
	"sync"

	flags "github.com/jessevdk/go-flags"
)

const (
	configFile = "breez.conf"
)

var (
	cfg         *Config
	configError error
	once        sync.Once
)

/*
JobConfig hodls the job configuration
*/
type JobConfig struct {
	ConnectedPeers     []string `long:"peer"`
	AssertFilterHeader string   `long:"assertfilterheader"`
	DisableRest        bool     `long:"disablerest"`
}

/*
Config holds the breez configuration
*/
type Config struct {
	WorkingDir         string
	BreezServer        string `long:"breezserver"`
	BreezServerNoTLS   bool   `long:"breezservernotls"`
	LspToken           string `long:"lsptoken"`
	SwapperPubkey      string `long:"swapperpubkey"`
	Network            string `long:"network"`
	GrpcKeepAlive      bool   `long:"grpckeepalive"`
	BootstrapURL       string `long:"bootstrap"`
	ClosedChannelsURL  string `long:"closedchannelsurl"`
	BugReportURL       string `long:"bugreporturl"`
	BugReportURLSecret string `long:"bugreporturlsecret"`
	TxSpentURL         string `long:"txspenturl"`

	//Job Options
	JobCfg JobConfig `group:"Job Options"`
}

// GetConfig returns the config object
func GetConfig(workingDir string) (*Config, error) {
	once.Do(func() {
		configError = initConfig(workingDir)
	})
	return cfg, configError
}

func initConfig(workingDir string) error {
	c := &Config{WorkingDir: workingDir}
	if err := flags.IniParse(path.Join(workingDir, configFile), c); err != nil {
		return err
	}
	cfg = c
	return nil
}
