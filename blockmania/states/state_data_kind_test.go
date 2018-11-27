package states

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StateDataKind", func() {
	Describe("String", func() {

		type stateDataKindStringTest struct {
			sdKind   StateDataKind
			expected string
		}

		var stateDataKindStringTests = []stateDataKindStringTest{
			{FinalState, "final"},
			{HNVState, "hnv"},
			{PreparedState, "prepared"},
			{PrePreparedState, "preprepared"},
			{UnknownState, "unknown"},
			{ViewState, "viewState"},
			{ViewChangedState, "viewchanged"},
		}

		for _, sdt := range stateDataKindStringTests {
			sdt := sdt

			It("should return a formatted string for each msg type", func() {
				actual := sdt.sdKind.String()
				expected := sdt.expected

				Expect(actual).To(Equal(expected))
			})
		}
	})
})
