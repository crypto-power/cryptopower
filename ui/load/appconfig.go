package load

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	libutils "github.com/crypto-power/cryptopower/libwallet/utils"
)

// AppConfig is a helper for reading and storing app-wide config values using a
// JSON file.
type AppConfig struct {
	mtx          sync.RWMutex
	jsonFilePath string
	values       *AppConfigValues
}

// AppConfigValues are app-wide configuration options with their values.
type AppConfigValues struct {
	NetType string `json:"netType"`
}

var defaultAppConfigValues = &AppConfigValues{
	NetType: string(libutils.Mainnet),
}

// AppConfigFromFile attempts to load and parse the JSON file at the specified
// path, and read the stored config values. If there is no file at the specified
// path, a new AppConfig instance with default values will be returned.
func AppConfigFromFile(jsonFilePath string) (*AppConfig, error) {
	cfg := &AppConfig{
		jsonFilePath: jsonFilePath,
		values:       defaultAppConfigValues, // updated below if the json file exists
	}

	cfgJSON, err := os.ReadFile(jsonFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("os.ReadFile error: %v", err)
	}

	if err = json.Unmarshal(cfgJSON, &cfg.values); err != nil {
		return nil, fmt.Errorf("unmarshal cfg json error: %v", err)
	}

	return cfg, nil
}

// Values returns a read-only copy of the current app-wide cofiguration values.
func (cfg *AppConfig) Values() AppConfigValues {
	cfg.mtx.RLock()
	defer cfg.mtx.RUnlock()
	values := cfg.values
	return *values // return a copy to prevent unprotected mutation
}

// Update provides access to the current app-wide cofiguration values for
// updating in a concurrent-safe manner. Concurrent reads and updates are
// prevented when an update operation is on. Returns an error if the updated
// config values cannot be persisted to the JSON file; and the update is
// discarded.
func (cfg *AppConfig) Update(updateFn func(*AppConfigValues)) error {
	// Write-lock the mtx to prevent concurrent updates and reads.
	cfg.mtx.Lock()
	defer cfg.mtx.Unlock()

	// Allow the caller to update a temporary copy of the values. Only accept
	// the update if the changes can be persisted to file below.
	tempValues := *cfg.values // copy the values before updating
	updateFn(&tempValues)

	// Persist the changes to the temp values copy to the json file.
	cfgJSON, err := json.Marshal(tempValues)
	if err != nil {
		return fmt.Errorf("json.Marshal error: %v", err)
	}
	if err = os.WriteFile(cfg.jsonFilePath, cfgJSON, libutils.UserFilePerm); err != nil {
		return fmt.Errorf("os.WriteFile error: %v", err)
	}

	cfg.values = &tempValues
	return nil
}
