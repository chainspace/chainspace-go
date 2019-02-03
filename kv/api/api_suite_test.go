package api_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestKVAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "KV API Suite")
}
