package blockmania_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "chainspace.io/prototype/blockmania"
)

var _ = Describe("Byzco", func() {

	var (
		block BlockID
	)

	BeforeEach(func() {
		block = BlockID{}
	})

	Describe("Valid", func() {
		Context("with an empty hash", func() {
			It("should be invalid", func() {
				Expect(block.Valid()).To(Equal(false))
			})
		})

		Context("with an populated hash", func() {
			BeforeEach(func() {
				block.Hash = "foo"
			})
			It("should be valid", func() {
				Expect(block.Valid()).To(Equal(true))
			})
		})
	})
})
