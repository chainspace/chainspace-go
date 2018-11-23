package contracts

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Checker", func() {
	Describe("encodeToStrings", func() {
		var bytes [][]byte

		BeforeEach(func() {
			strings := []string{"foo", "bar"}
			bytes = make([][]byte, len(strings))
			for i, v := range strings {
				bytes[i] = []byte(v)
			}
		})

		It("should do something", func() {
			actual := encodeToStrings(bytes)
			expected := []string{"Zm9v", "YmFy"}
			Expect(actual).To(Equal(expected))
		})
	})

	Describe("makeTrace", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("unmarshalIfaceSlice", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("Check", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("ContractID", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("Name", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("NewCheckers", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("NewDockerCheckers", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})
})
