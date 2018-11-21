package blockmania

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Graph", func() {
	var e entry
	var graph = Graph{}

	BeforeEach(func() {
		e = entry{}
	})

	Describe("findOrCreateState", func() {
		Context("when there is already a previous state", func() {
			BeforeEach(func() {
				e = entry{
					prev: BlockID{Hash: "barbarbarbar"},
				}
			})

			It("returns a state", func() {
				actual := graph.findOrCreateState(&e)
				expected := graph.states[e.prev].clone(graph.round)

				Expect(actual).To(Equal(expected))
			})
		})

		Context("when no previous state exists", func() {
			It("returns a blank state populated with a timeout map", func() {
				actual := graph.findOrCreateState(&e)
				expected := &state{
					timeouts: map[uint64][]timeout{},
				}

				Expect(actual).To(Equal(expected))
			})
		})
	})
})
