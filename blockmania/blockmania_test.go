package blockmania_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "chainspace.io/prototype/blockmania"
)

var _ = Describe("Blockmania", func() {

	var (
		blockID BlockID
	)

	BeforeEach(func() {
		blockID = BlockID{}
	})

	Describe("String", func() {
		Context("A valid BlockID", func() {
			BeforeEach(func() {
				blockID.Hash = "foofoofoofoo" // minimum 12 char hash :)
			})

			It("should return a formatted string", func() {

				var result = fmt.Sprintf("%v | %v | %v", blockID.Node, blockID.Round, "666F6F666F6F")
				Expect(blockID.String()).To(Equal(result))
			})
		})

		Context("An invalid BlockID", func() {
			It("should return a formatted string", func() {
				var result = fmt.Sprintf("%v | %v", blockID.Node, blockID.Round)
				Expect(blockID.String()).To(Equal(result))
			})
		})
	})

	Describe("Valid", func() {
		Context("with an empty hash", func() {
			It("should be invalid", func() {
				Expect(blockID.Valid()).To(Equal(false))
			})
		})

		Context("with an populated hash", func() {
			BeforeEach(func() {
				blockID.Hash = "foo"
			})
			It("should be valid", func() {
				Expect(blockID.Valid()).To(Equal(true))
			})
		})
	})
})
