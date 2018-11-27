package blockmania

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Blockmania", func() {
	Describe("BlockID", func() {
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

				It("should return a formatted string with the node, round, and block hash", func() {
					actual := blockID.String()
					expected := fmt.Sprintf("%v | %v | %v", blockID.Node, blockID.Round, "666F6F666F6F")

					Expect(actual).To(Equal(expected))
				})
			})

			Context("An invalid BlockID", func() {
				It("should return a formatted string with only the node and round", func() {
					actual := blockID.String()
					expected := fmt.Sprintf("%v | %v", blockID.Node, blockID.Round)

					Expect(actual).To(Equal(expected))
				})
			})
		})

		Describe("Valid", func() {
			Context("with an empty hash", func() {
				It("should be invalid", func() {
					actual := blockID.Valid()
					expected := false

					Expect(actual).To(Equal(expected))
				})
			})

			Context("with a populated hash", func() {
				BeforeEach(func() {
					blockID.Hash = "foo"
				})

				It("should be valid", func() {
					actual := blockID.Valid()
					expected := true

					Expect(actual).To(Equal(expected))
				})
			})
		})
	})

})
