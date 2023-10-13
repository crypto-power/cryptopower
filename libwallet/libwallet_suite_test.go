package libwallet_test

import (
	"math/rand"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLibwallet(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.New(rand.NewSource(GinkgoRandomSeed()))
	RunSpecs(t, "Libwallet Suite")
}
