package load

import "time"

// TODO: This should ultimately replace Load, acting as the container for all
// properties and methods that every app page and window relies on. Should
// probably rename as well.
type AppInfo struct {
	version     string
	buildDate   time.Time
	startUpTime time.Time
}

// StartApp returns an instance of AppInfo with the startUpTime set to the current time.
func StartApp(version string, buildDate time.Time) *AppInfo {
	return &AppInfo{
		version:     version,
		buildDate:   buildDate,
		startUpTime: time.Now(),
	}
}

func (app *AppInfo) BuildDate() time.Time {
	return app.buildDate
}

func (app *AppInfo) Version() string {
	return app.version
}

func (app *AppInfo) StartupTime() time.Time {
	return app.startUpTime
}
