package api_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPubSubAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PubSub API Suite")
}
