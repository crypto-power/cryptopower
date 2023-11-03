package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"time"

	"gioui.org/app"

	"github.com/crypto-power/cryptopower/libwallet"
	"github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/logger"
	"github.com/crypto-power/cryptopower/ui"
	_ "github.com/crypto-power/cryptopower/ui/assets"
)

const (
	devBuild  = "dev"
	prodBuild = "prod"
)

var (
	// Version is the application version. It is set using the -ldflags
	Version = "1.7.0"
	// BuildDate is the date the application was built. It is set using the -ldflags
	BuildDate string
	// BuildEnv is the build environment. It is set using the -ldflags
	BuildEnv = devBuild
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return
	}

	// before the asset manager is initialized use command line debuglevel option if passed or
	// default to log level info for startup logs.
	if cfg.DebugLevel == "" {
		logger.SetLogLevels(utils.LogLevelInfo)
	} else {
		logger.SetLogLevels(cfg.DebugLevel)
	}

	if cfg.Profile > 0 {
		go func() {
			log.Info(fmt.Sprintf("Starting profiling server on port %d", cfg.Profile))
			log.Error(http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", cfg.Profile), nil))
		}()
	}

	var buildDate time.Time
	if BuildEnv == prodBuild {
		buildDate, err = time.Parse(time.RFC3339, BuildDate)
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			return
		}
	} else {
		buildDate = time.Now()
	}

	var net string
	switch cfg.Network {
	case "testnet":
		net = "testnet3"
	default:
		net = cfg.Network
	}

	logDir := filepath.Join(cfg.LogDir, net)
	assetsManager, err := libwallet.NewAssetsManager(cfg.HomeDir, logDir, net)
	if err != nil {
		log.Errorf("init assetsManager error: %v", err)
		return
	}

	// if debuglevel is passed at commandLine persist the option.
	if cfg.DebugLevel != "" && assetsManager.IsAssetManagerDB() {
		assetsManager.SetLogLevels(cfg.DebugLevel)
	}

	if assetsManager.IsAssetManagerDB() {
		// now that assets manager is up, set stored debuglevel
		logger.SetLogLevels(assetsManager.GetLogLevels())
	}

	win, err := ui.CreateWindow(assetsManager, Version, buildDate)
	if err != nil {
		log.Errorf("Could not initialize window: %s\ns", err)
		return
	}

	go func() {
		// Wait until we receive the shutdown request.
		<-win.Quit
		// Terminate all the backend processes safely.
		assetsManager.Shutdown()
		// Backend process terminated safely trigger app shutdown now.
		win.IsShutdown <- struct{}{}
	}()

	go func() {
		// blocks until the backend processes terminate.
		win.HandleEvents()
		// Exit the app.
		os.Exit(0)
	}()

	// Start the GUI frontend.
	app.Main()
}
