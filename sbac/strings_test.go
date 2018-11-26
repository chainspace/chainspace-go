package sbac

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Strings", func() {
  var stringsMap [][]string
  var strings []string
  var strs *Strings
  var strsMap []*Strings
  var strsSlice StringsSlice

	BeforeEach(func() {
		// Making sure we've reset all the values!
		strings = nil
		stringsMap = nil
		strs = nil
		strsMap = nil
		strsSlice = nil
	})

	Describe("AsSlice", func() {
		When("called on Strings", func() {
			BeforeEach(func() {
				strings = append(strings, "foo", "bar", "baz")
				strs = &Strings{strings}
			})

			It("should return a slice", func() {
				actual := strs.AsSlice()
				expected := strings
				Expect(actual).To(Equal(expected))
			})
		})

		When("called on StringsSlice", func() {
			BeforeEach(func() {
				strings = append(strings, "foo", "bar", "baz")
				stringsMap = append(stringsMap, strings)
				strs = &Strings{strings}
				strsSlice = append(strsSlice, strs)
			})

			It("should return  a slice", func() {
				actual := strsSlice.AsSlice()
				expected := stringsMap
				Expect(actual).To(Equal(expected))
			})
		})
	})

	Describe("FromSlice", func() {
		BeforeEach(func() {
			strings = append(strings, "foo", "bar", "baz")
			stringsMap = append(stringsMap, strings)
			strs = &Strings{strings}
			strsMap = append(strsMap, strs)
			strsSlice = append(strsSlice, strs)
		})

		It("should return strings", func() {
			actual := strsSlice.FromSlice(stringsMap)
			expected := strsMap
			Expect(actual).To(Equal(expected))
		})
	})
})
