package libwallet_test

import (
	"math/rand"
	"testing"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

func TestLibwallet(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	rand.Seed(ginkgo.GinkgoRandomSeed())
	ginkgo.RunSpecs(t, "Libwallet Suite")
}
