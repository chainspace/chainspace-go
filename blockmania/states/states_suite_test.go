package states_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestStates(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "States Suite")
}
