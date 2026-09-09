package galaxy_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGalaxy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Galaxy Suite")
}
