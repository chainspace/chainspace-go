package blockmania_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBlockmania(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Blockmania Suite")
}
