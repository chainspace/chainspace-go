package checker

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Checker", func() {
	var srv = Service{}

	Describe("check", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("Check", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("CheckAndSign", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("Handle", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("Name", func() {
		It("should return a string", func() {
			actual := srv.Name()
			expected := "checker"

			Expect(actual).To(Equal(expected))
		})
	})

	Describe("New", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})
})
