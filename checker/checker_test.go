package checker_test

import (
	"chainspace.io/prototype/checker"
	"chainspace.io/prototype/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var signingKey = &config.Key{
	Type:    "ed25519",
	Public:  "7EEAMDHFJQJKQE2N6GWWG7DETU5ZPEDDUQCFFHR74PECIF4FHVCQ",
	Private: "DCUCO7RJVZK33IXR3EMQELOQNQNJJTCOSY4MYR6IEM4BBWVL5SL7SCAGBTSUYEVICNG7DLLDPRSJ2O4XSBR2IBCSTY76HSBEC6CT2RI",
}

var _ = Describe("Checker", func() {
	var srv checker.Service

	BeforeEach(func() {
		cfg := &checker.Config{
			Checkers:   []checker.Checker{},
			SigningKey: signingKey,
		}
		srv, _ = checker.New(cfg)
	})

	Describe("Name", func() {
		It("should return a string", func() {
			actual := srv.Name()
			expected := "checker"

			Expect(actual).To(Equal(expected))
		})
	})
})
