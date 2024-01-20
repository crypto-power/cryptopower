package appos

// AppOS holds properties and methods for determining the OS the app is running
// on.
type AppOS struct {
	isAndroid bool
	isIOS     bool
}

// Default AppOS value. Updated in OS-specific files for each OS.
var appCurrentOS = &AppOS{}

// Current provides read-only access to the appCurrentOS variable.
func Current() *AppOS {
	return appCurrentOS
}

func (os *AppOS) IsAndroid() bool {
	return os.isAndroid
}

func (os *AppOS) IsIOS() bool {
	return os.isIOS
}

func (os *AppOS) IsMobile() bool {
	return os.isAndroid || os.isIOS
}
