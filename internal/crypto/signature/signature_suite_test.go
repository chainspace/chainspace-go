package signature_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSignature(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Signature Suite")
}
