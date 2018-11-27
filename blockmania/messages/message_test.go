package messages

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Message", func() {
	Describe("fmtHash", func() {
		Context("with a blank string", func() {
			It("should return a blank string", func() {
				actual := fmtHash("")
				expected := ""

				Expect(actual).To(Equal(expected))
			})
		})

		Context("with a string", func() {
			It("should return a formatted string", func() {
				actual := fmtHash("lotsoffoobar")
				expected := "666F6F626172"

				Expect(actual).To(Equal(expected))
			})
		})
	})

})
