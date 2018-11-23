package sbac_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSBAC(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SBAC Suite")
}
