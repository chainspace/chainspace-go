package conns_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestConns(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Conns Suite")
}
