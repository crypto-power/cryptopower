package main

import (
	"fmt"
	golog "log"
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
	"github.com/crypto-power/cryptopower/ui/load"
)

const (
	devBuild  = "dev"
	prodBuild = "prod"
)

var (
	// Version is the application version. It is set using the -ldflags
	Version = "1.1.3"
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

	// f, _ := os.Create("cpu.pprof")
	// pprof.StartCPUProfile(f)

	// mem, _ := os.Create("mem.pprof")
	// runtime.GC()
	// if err := pprof.WriteHeapProfile(mem); err != nil {
	// 	fmt.Println("could not write memory profile: ", err)
	// }

	if cfg.Profile > 0 {
		go func() {
			golog.Printf("Starting profiling server on port %d\n", cfg.Profile)
			golog.Println(http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", cfg.Profile), nil))
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

	initializeAssetsManager := func(netType utils.NetworkType) (*libwallet.AssetsManager, error) {
		//  Initialize loggers and set log level before the asset manager is
		//  initialized.
		logDir := filepath.Join(cfg.LogDir, string(netType))
		initLogRotator(logDir, cfg.MaxLogZips)
		if cfg.DebugLevel == "" {
			logger.SetLogLevels(utils.DefaultLogLevel)
		} else {
			logger.SetLogLevels(cfg.DebugLevel)
		}

		assetsManager, err := libwallet.NewAssetsManager(cfg.HomeDir, logDir, netType, cfg.DEXTestAddr)
		if err != nil {
			return nil, err
		}

		// if debuglevel is passed at commandLine persist the option.
		if cfg.DebugLevel != "" {
			assetsManager.SetLogLevels(cfg.DebugLevel)
		} else {
			logger.SetLogLevels(assetsManager.GetLogLevels())
		}

		return assetsManager, nil
	}

	// Load the app-wide config which stores information such as the netType
	// last used by the user.
	appCfgFilePath := filepath.Join(cfg.HomeDir, "config.json")
	appCfg, err := load.AppConfigFromFile(appCfgFilePath)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return
	}

	// Init the AssetsManager using the user-selected netType or mainnet if the
	// user has not selected a netType from the app's settings page.
	appInfo, err := load.StartApp(Version, buildDate, cfg.Network, appCfg, initializeAssetsManager)
	if err != nil {
		log.Errorf("init assetsManager error: %v", err)
		return
	}

	win, err := ui.CreateWindow(appInfo)
	if err != nil {
		log.Errorf("Could not initialize window: %s\ns", err)
		return
	}

	go func() {
		// Wait until we receive the shutdown request.
		<-win.Quit
		// Terminate all the backend processes safely.
		// pprof.StopCPUProfile()
		// mem.Close()
		appInfo.AssetsManager.Shutdown()
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
