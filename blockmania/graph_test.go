package blockmania

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Graph", func() {

	var graph = Graph{}
	var e entry
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
				var result = graph.findOrCreateState(&e)
				var expected = graph.states[e.prev].clone(graph.round)
				Expect(result).To(Equal(expected))
			})
		})

		Context("when no previous state exists", func() {
			It("returns a blank state populated with a timeout map", func() {
				var result = graph.findOrCreateState(&e)
				var expected = &state{
					timeouts: map[uint64][]timeout{},
				}
				Expect(result).To(Equal(expected))
			})
		})
	})
})
