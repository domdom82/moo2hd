package ship_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestShip(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ship Suite")
}
