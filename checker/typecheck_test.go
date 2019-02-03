package checker

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Typecheck", func() {
	Describe("b64", func() {
		It("should encode byte data", func() {
			actual := b64([]byte("Data yo!"))
			expected := "RGF0YSB5byE="
			Expect(actual).To(Equal(expected))
		})
	})

	Describe("typeCheck", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("TypeCheck", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})
})
