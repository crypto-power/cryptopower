//go:build darwin && cgo
// +build darwin,cgo

package macos

import (
	"testing"
)

func TestCancelNonexistent(t *testing.T) {
	var notif *Notification
	if err := notif.Cancel(); err == nil {
		t.Fatalf("should fail to cancel nonexistent notification id")
	}
}
