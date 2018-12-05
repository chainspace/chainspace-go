package api_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCheckerAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Checker API Suite")
}
