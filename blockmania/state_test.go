package blockmania

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("State", func() {

	Describe("clone", func() {
		Context("for state", func() {
			It("should clone the state", func() {
				Skip("Write test(s) for this...")
			})
		})
	})

	Describe("diff", func() {
		var a uint64
		var b uint64

		Context("when a > b", func() {
			BeforeEach(func() {
				a = 4
				b = 2
			})

			It("Should return a - b", func() {
				actual := diff(a, b)
				expected := a - b

				Expect(actual).To(Equal(expected))
			})
		})

		Context("when a = b", func() {
			BeforeEach(func() {
				a = 2
				b = 2
			})

			It("Should return a - b", func() {
				actual := diff(a, b)
				expected := a - b

				Expect(actual).To(Equal(expected))
			})
		})

		Context("when a < b", func() {
			BeforeEach(func() {
				a = 2
				b = 4
			})

			It("Should return b - a", func() {
				actual := diff(a, b)
				expected := b - a

				Expect(actual).To(Equal(expected))
			})
		})
	})

	Describe("getBitset", func() {
		Context("for state", func() {
			It("should clone the state", func() {
				Skip("Write test(s) for this...")
			})
		})
	})

	Describe("getOutput", func() {
		Context("for state", func() {
			It("should clone the state", func() {
				Skip("Write test(s) for this...")
			})
		})
	})

	Describe("getView", func() {
		Context("for state", func() {
			It("should clone the state", func() {
				Skip("Write test(s) for this...")
			})
		})
	})

})
