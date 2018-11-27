package blockmania

import (
	"chainspace.io/prototype/blockmania/states"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("State", func() {
	var (
		f   states.Final
		h   states.HNV
		pd  states.Prepared
		ppd states.PrePrepared
		v   states.View
		vcd states.ViewChanged
	)

	BeforeEach(func() {
		f = states.Final{}
		h = states.HNV{}
		pd = states.Prepared{}
		ppd = states.PrePrepared{}
		v = states.View{}
		vcd = states.ViewChanged{}
	})

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

	Describe("getView", func() {
		Context("for state", func() {
			It("should clone the state", func() {
				Skip("Write test(s) for this...")
			})
		})
	})

	Describe("SdKind", func() {
		Context("for final", func() {
			It("should return the correct stateDataKind", func() {
				actual := f.SdKind()
				expected := states.FinalState

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for hnv", func() {
			It("should return the correct stateDataKind", func() {
				actual := h.SdKind()
				expected := states.HNVState

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prepared", func() {
			It("should return the correct stateDataKind", func() {
				actual := pd.SdKind()
				expected := states.PreparedState

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prePrepared", func() {
			It("should return the correct stateDataKind", func() {
				actual := ppd.SdKind()
				expected := states.PrePreparedState

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for view", func() {
			It("should return the correct stateDataKind", func() {
				actual := v.SdKind()
				expected := states.ViewState

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for viewChanged", func() {
			It("should return the correct stateDataKind", func() {
				actual := vcd.SdKind()
				expected := states.ViewChangedState

				Expect(actual).To(Equal(expected))
			})
		})
	})

	Describe("String", func() {

		Context("for stateDataKind", func() {
			type stateDataKindStringTest struct {
				sdKind   states.StateDataKind
				expected string
			}

			var stateDataKindStringTests = []stateDataKindStringTest{
				{states.FinalState, "final"},
				{states.HNVState, "hnv"},
				{states.PreparedState, "prepared"},
				{states.PrePreparedState, "preprepared"},
				{states.UnknownState, "unknown"},
				{states.ViewState, "viewState"},
				{states.ViewChangedState, "viewchanged"},
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
})
