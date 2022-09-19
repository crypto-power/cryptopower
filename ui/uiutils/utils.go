package uiutils

// the length of name should be 32 characters
func ValidateLengthName(name string) bool {
	return len(name) <= 32
}
