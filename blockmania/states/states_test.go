package states_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "chainspace.io/prototype/blockmania/states"
)

var _ = Describe("States", func() {

	var (
		f   Final
		h   HNV
		pd  Prepared
		ppd PrePrepared
		v   View
		vcd ViewChanged
	)

	BeforeEach(func() {
		f = Final{}
		h = HNV{}
		pd = Prepared{}
		ppd = PrePrepared{}
		v = View{}
		vcd = ViewChanged{}
	})

	// TODO: I think we may be able to dry up these tests, given
	// that they are mostly re-testing the same interfaces on different types,
	// and have pretty much the same expected results across State-conforming
	// types

	Describe("getRound", func() {
		Context("for final", func() {
			BeforeEach(func() {
				f.Round = 1
			})

			It("should return the round value", func() {
				actual := f.GetRound()
				expected := f.Round

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for hnv", func() {
			BeforeEach(func() {
				h.Round = 1
			})

			It("should return the round value", func() {
				actual := h.GetRound()
				expected := h.Round

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prepared", func() {
			BeforeEach(func() {
				pd.Round = 1
			})

			It("should return the round value", func() {
				actual := pd.GetRound()
				expected := pd.Round

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prePrepared", func() {
			BeforeEach(func() {
				ppd.Round = 1
			})

			It("should return the round value", func() {
				actual := ppd.GetRound()
				expected := ppd.Round

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for view", func() {
			BeforeEach(func() {
				v.Round = 1
			})

			It("should return the round value", func() {
				actual := v.GetRound()
				expected := v.Round

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for viewChanged", func() {
			BeforeEach(func() {
				vcd.Round = 1
			})

			It("should return the round value", func() {
				actual := vcd.GetRound()
				expected := vcd.Round

				Expect(actual).To(Equal(expected))
			})
		})
	})

	Describe("SdKind", func() {
		Context("for final", func() {
			It("should return the correct stateDataKind", func() {
				actual := f.SdKind()
				expected := FinalState

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for hnv", func() {
			It("should return the correct stateDataKind", func() {
				actual := h.SdKind()
				expected := HNVState

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prepared", func() {
			It("should return the correct stateDataKind", func() {
				actual := pd.SdKind()
				expected := PreparedState

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prePrepared", func() {
			It("should return the correct stateDataKind", func() {
				actual := ppd.SdKind()
				expected := PrePreparedState

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for view", func() {
			It("should return the correct stateDataKind", func() {
				actual := v.SdKind()
				expected := ViewState

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for viewChanged", func() {
			It("should return the correct stateDataKind", func() {
				actual := vcd.SdKind()
				expected := ViewChangedState

				Expect(actual).To(Equal(expected))
			})
		})
	})

})
