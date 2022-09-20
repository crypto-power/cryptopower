package uiutils

import (
	"net"
	"net/url"
	"strings"
)

// the length of name should be 32 characters
func ValidateLengthName(name string) bool {
	return len(name) <= 32
}

func ValidateHost(host string) bool {
	adress := strings.Trim(host, " ")

	if net.ParseIP(adress) != nil {
		return true
	}

	_, err := url.ParseRequestURI(adress)
	return err == nil

}
