package research_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestResearch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Research Suite")
}
