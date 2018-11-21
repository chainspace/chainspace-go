package blockmania

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("State", func() {
	var (
		c   commit
		f   final
		h   hnv
		nv  newView
		p   prepare
		pd  prepared
		pp  prePrepare
		ppd prePrepared
		v   view
		vc  viewChange
		vcd viewChanged
	)

	BeforeEach(func() {
		c = commit{}
		f = final{}
		h = hnv{}
		nv = newView{}
		p = prepare{}
		pd = prepared{}
		pp = prePrepare{}
		ppd = prePrepared{}
		v = view{}
		vc = viewChange{}
		vcd = viewChanged{}
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

	Describe("fmtHash", func() {
		Context("with a blank string", func() {
			It("should return a blank string", func() {
				actual := fmtHash("")
				expected := ""

				Expect(actual).To(Equal(expected))
			})
		})

		Context("with a string", func() {
			It("should return a formatted string", func() {
				actual := fmtHash("lotsoffoobar")
				expected := "666F6F626172"

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
				f.round = 1
			})

			It("should return the round value", func() {
				actual := f.getRound()
				expected := f.round

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for hnv", func() {
			BeforeEach(func() {
				h.round = 1
			})

			It("should return the round value", func() {
				actual := h.getRound()
				expected := h.round

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prepared", func() {
			BeforeEach(func() {
				pd.round = 1
			})

			It("should return the round value", func() {
				actual := pd.getRound()
				expected := pd.round

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prePrepared", func() {
			BeforeEach(func() {
				ppd.round = 1
			})

			It("should return the round value", func() {
				actual := ppd.getRound()
				expected := ppd.round

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for view", func() {
			BeforeEach(func() {
				v.round = 1
			})

			It("should return the round value", func() {
				actual := v.getRound()
				expected := v.round

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for viewChanged", func() {
			BeforeEach(func() {
				vcd.round = 1
			})

			It("should return the round value", func() {
				actual := vcd.getRound()
				expected := vcd.round

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

	Describe("kind", func() {
		Context("for commit", func() {
			It("should return the right msg kind", func() {
				actual := c.kind()
				expected := commitMsg

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for newView", func() {
			It("should return the right msg kind", func() {
				actual := nv.kind()
				expected := newViewMsg

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prepare", func() {
			It("should return the right msg kind", func() {
				actual := p.kind()
				expected := prepareMsg

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prePrepare", func() {
			It("should return the right msg kind", func() {
				actual := pp.kind()
				expected := prePrepareMsg

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for viewChange", func() {
			It("should return the right msg kind", func() {
				actual := vc.kind()
				expected := viewChangedMsg

				Expect(actual).To(Equal(expected))
			})
		})
	})

	Describe("nodeRound", func() {
		Context("for commit", func() {
			BeforeEach(func() {
				c.node = 2
				c.round = 1
			})

			It("should return the node and round values", func() {
				node, round := c.nodeRound()

				Expect(node).To(Equal(c.node))
				Expect(round).To(Equal(c.round))
			})
		})

		Context("for newView", func() {
			BeforeEach(func() {
				nv.node = 2
				nv.round = 1
			})

			It("should return the node and round values", func() {
				node, round := nv.nodeRound()

				Expect(node).To(Equal(nv.node))
				Expect(round).To(Equal(nv.round))
			})
		})

		Context("for prepare", func() {
			BeforeEach(func() {
				p.node = 2
				p.round = 1
			})

			It("should return the node and round values", func() {
				node, round := p.nodeRound()

				Expect(node).To(Equal(p.node))
				Expect(round).To(Equal(p.round))
			})
		})

		Context("for prePrepare", func() {
			BeforeEach(func() {
				pp.node = 2
				pp.round = 1
			})

			It("should return the node and round values", func() {
				node, round := pp.nodeRound()

				Expect(node).To(Equal(pp.node))
				Expect(round).To(Equal(pp.round))
			})
		})

		Context("for viewChange", func() {
			BeforeEach(func() {
				vc.node = 2
				vc.round = 1
			})

			It("should return the node and round values", func() {
				node, round := vc.nodeRound()

				Expect(node).To(Equal(vc.node))
				Expect(round).To(Equal(vc.round))
			})
		})
	})

	Describe("pre", func() {
		Context("for commit", func() {
			BeforeEach(func() {
				c.hash = "foofoofoofoo" // minimum 12 char hash :)
				c.node = 2
				c.round = 1
				c.view = 1
			})

			It("should return a prePrepare", func() {
				actual := c.pre()
				expected := prePrepare{hash: c.hash, node: c.node, round: c.round, view: c.view}

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prepare", func() {
			BeforeEach(func() {
				p.hash = "foofoofoofoo" // minimum 12 char hash :)
				p.node = 2
				p.round = 1
				p.view = 1
			})

			It("should return a prePrepare", func() {
				actual := p.pre()
				expected := prePrepare{hash: p.hash, node: p.node, round: p.round, view: p.view}

				Expect(actual).To(Equal(expected))
			})
		})
	})

	Describe("sdKind", func() {
		Context("for final", func() {
			It("should return the correct stateDataKind", func() {
				actual := f.sdKind()
				expected := finalState

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for hnv", func() {
			It("should return the correct stateDataKind", func() {
				actual := h.sdKind()
				expected := hnvState

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prepared", func() {
			It("should return the correct stateDataKind", func() {
				actual := pd.sdKind()
				expected := preparedState

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prePrepared", func() {
			It("should return the correct stateDataKind", func() {
				actual := ppd.sdKind()
				expected := prePreparedState

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for view", func() {
			It("should return the correct stateDataKind", func() {
				actual := v.sdKind()
				expected := viewState

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for viewChanged", func() {
			It("should return the correct stateDataKind", func() {
				actual := vcd.sdKind()
				expected := viewChangedState

				Expect(actual).To(Equal(expected))
			})
		})
	})

	Describe("String", func() {
		Context("for commit", func() {
			BeforeEach(func() {
				c.hash = "foofoofoofoo" // minimum 12 char hash :)
				c.node = 2
				c.round = 1
				c.sender = 3
			})

			It("should return a formatted string", func() {
				actual := c.String()
				expected := "commit{node: 2, round: 1, view: 0, hash: '666F6F666F6F', sender: 3}"

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for messageKind", func() {
			type messageKindStringTest struct {
				msgKind  messageKind
				expected string
			}

			var messageKindStringTests = []messageKindStringTest{
				{commitMsg, "commit"},
				{newViewMsg, "new-view"},
				{prepareMsg, "prepare"},
				{prePrepareMsg, "pre-prepare"},
				{unknownMsg, "unknown"},
				{viewChangedMsg, "view-change"},
			}

			for _, mkt := range messageKindStringTests {
				mkt := mkt

				It("should return a formatted string for each msg type", func() {
					actual := mkt.msgKind.String()
					expected := mkt.expected

					Expect(actual).To(Equal(expected))
				})
			}
		})

		Context("for newView", func() {
			BeforeEach(func() {
				nv.hash = "foofoofoofoo" // minimum 12 char hash :)
				nv.node = 2
				nv.round = 1
				nv.sender = 3
			})

			It("should return a formatted string", func() {
				actual := nv.String()
				expected := "new-view{node: 2, round: 1, view: 0, hash: '666F6F666F6F', sender: 3}"

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prepare", func() {
			BeforeEach(func() {
				p.hash = "foofoofoofoo" // minimum 12 char hash :)
				p.node = 2
				p.round = 1
				p.sender = 3
			})

			It("should return a formatted string", func() {
				actual := p.String()
				expected := "prepare{node: 2, round: 1, view: 0, hash: '666F6F666F6F', sender: 3}"

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prePrepare", func() {
			BeforeEach(func() {
				pp.hash = "foofoofoofoo" // minimum 12 char hash :)
				pp.node = 2
				pp.round = 1
			})

			It("should return a formatted string", func() {
				actual := pp.String()
				expected := "pre-prepare{node: 2, round: 1, view: 0, hash: '666F6F666F6F'}"

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for stateDataKind", func() {
			type stateDataKindStringTest struct {
				sdKind   stateDataKind
				expected string
			}

			var stateDataKindStringTests = []stateDataKindStringTest{
				{finalState, "final"},
				{hnvState, "hnv"},
				{preparedState, "prepared"},
				{prePreparedState, "preprepared"},
				{unknownState, "unknown"},
				{viewState, "viewState"},
				{viewChangedState, "viewchanged"},
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

		Context("for viewChange", func() {
			BeforeEach(func() {
				vc.hash = "foofoofoofoo" // minimum 12 char hash :)
				vc.node = 2
				vc.round = 1
				vc.sender = 5
			})

			It("should return a formatted string", func() {
				actual := vc.String()
				expected := "view-change{node: 2, round: 1, view: 0, hash: '666F6F666F6F', sender: 5}"

				Expect(actual).To(Equal(expected))
			})
		})
	})
})
