package eth

import "code.cryptopower.dev/group/cryptopower/libwallet/utils"

func (asset *Asset) SaveUserConfigValue(key string, value interface{}) {
	log.Error(utils.ErrETHMethodNotImplemented("SaveUserConfigValue"))
}

func (asset *Asset) ReadUserConfigValue(key string, valueOut interface{}) error {
	return utils.ErrETHMethodNotImplemented("ReadUserConfigValue")
}

func (asset *Asset) SetBoolConfigValueForKey(key string, value bool) {
	log.Error(utils.ErrETHMethodNotImplemented("SetBoolConfigValueForKey"))
}

func (asset *Asset) SetDoubleConfigValueForKey(key string, value float64) {
	log.Error(utils.ErrETHMethodNotImplemented("SetDoubleConfigValueForKey"))
}

func (asset *Asset) SetIntConfigValueForKey(key string, value int) {
	log.Error(utils.ErrETHMethodNotImplemented("SetIntConfigValueForKey"))
}

func (asset *Asset) SetInt32ConfigValueForKey(key string, value int32) {
	log.Error(utils.ErrETHMethodNotImplemented("SetInt32ConfigValueForKey"))
}

func (asset *Asset) SetLongConfigValueForKey(key string, value int64) {
	log.Error(utils.ErrETHMethodNotImplemented("SetLongConfigValueForKey"))
}

func (asset *Asset) SetStringConfigValueForKey(key, value string) {
	log.Error(utils.ErrETHMethodNotImplemented("SetStringConfigValueForKey"))
}

func (asset *Asset) ReadBoolConfigValueForKey(key string, defaultValue bool) bool {
	log.Error(utils.ErrETHMethodNotImplemented("ReadBoolConfigValueForKey"))
	return false
}

func (asset *Asset) ReadDoubleConfigValueForKey(key string, defaultValue float64) float64 {
	log.Error(utils.ErrETHMethodNotImplemented("ReadDoubleConfigValueForKey"))
	return -1.0
}

func (asset *Asset) ReadIntConfigValueForKey(key string, defaultValue int) int {
	log.Error(utils.ErrETHMethodNotImplemented("ReadIntConfigValueForKey"))
	return -1
}

func (asset *Asset) ReadInt32ConfigValueForKey(key string, defaultValue int32) int32 {
	log.Error(utils.ErrETHMethodNotImplemented("ReadInt32ConfigValueForKey"))
	return -1
}

func (asset *Asset) ReadLongConfigValueForKey(key string, defaultValue int64) int64 {
	log.Error(utils.ErrETHMethodNotImplemented("ReadLongConfigValueForKey"))
	return -1
}

func (asset *Asset) ReadStringConfigValueForKey(key string, defaultValue string) string {
	log.Error(utils.ErrETHMethodNotImplemented("ReadStringConfigValueForKey"))
	return ""
}
