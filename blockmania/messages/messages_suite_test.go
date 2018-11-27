package messages_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMessages(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Messages Suite")
}
