package lbx_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLbx(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LBX Suite")
}
