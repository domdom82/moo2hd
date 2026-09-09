package colony_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestColony(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Colony Suite")
}
