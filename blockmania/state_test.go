package blockmania

import (
	"chainspace.io/prototype/blockmania/messages"
	"chainspace.io/prototype/blockmania/states"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("State", func() {
	var (
		c   messages.Commit
		f   states.Final
		h   states.HNV
		nv  messages.NewView
		p   messages.Prepare
		pd  states.Prepared
		pp  messages.PrePrepare
		ppd states.PrePrepared
		v   states.View
		vc  messages.ViewChange
		vcd states.ViewChanged
	)

	BeforeEach(func() {
		c = messages.Commit{}
		f = states.Final{}
		h = states.HNV{}
		nv = messages.NewView{}
		p = messages.Prepare{}
		pd = states.Prepared{}
		pp = messages.PrePrepare{}
		ppd = states.PrePrepared{}
		v = states.View{}
		vc = messages.ViewChange{}
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

	Describe("kind", func() {
		Context("for commit", func() {
			It("should return the right msg kind", func() {
				actual := c.Kind()
				expected := messages.CommitMsg

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for newView", func() {
			It("should return the right msg kind", func() {
				actual := nv.Kind()
				expected := messages.NewViewMsg

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prepare", func() {
			It("should return the right msg kind", func() {
				actual := p.Kind()
				expected := messages.PrepareMsg

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prePrepare", func() {
			It("should return the right msg kind", func() {
				actual := pp.Kind()
				expected := messages.PrePrepareMsg

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for viewChange", func() {
			It("should return the right msg kind", func() {
				actual := vc.Kind()
				expected := messages.ViewChangedMsg

				Expect(actual).To(Equal(expected))
			})
		})
	})

	Describe("NodeRound", func() {
		Context("for Commit", func() {
			BeforeEach(func() {
				c.Node = 2
				c.Round = 1
			})

			It("should return the node and round values", func() {
				node, round := c.NodeRound()

				Expect(node).To(Equal(c.Node))
				Expect(round).To(Equal(c.Round))
			})
		})

		Context("for NewView", func() {
			BeforeEach(func() {
				nv.Node = 2
				nv.Round = 1
			})

			It("should return the node and round values", func() {
				node, round := nv.NodeRound()

				Expect(node).To(Equal(nv.Node))
				Expect(round).To(Equal(nv.Round))
			})
		})

		Context("for prepare", func() {
			BeforeEach(func() {
				p.Node = 2
				p.Round = 1
			})

			It("should return the node and round values", func() {
				node, round := p.NodeRound()

				Expect(node).To(Equal(p.Node))
				Expect(round).To(Equal(p.Round))
			})
		})

		Context("for prePrepare", func() {
			BeforeEach(func() {
				pp.Node = 2
				pp.Round = 1
			})

			It("should return the node and round values", func() {
				node, round := pp.NodeRound()

				Expect(node).To(Equal(pp.Node))
				Expect(round).To(Equal(pp.Round))
			})
		})

		Context("for viewChange", func() {
			BeforeEach(func() {
				vc.Node = 2
				vc.Round = 1
			})

			It("should return the node and round values", func() {
				node, round := vc.NodeRound()

				Expect(node).To(Equal(vc.Node))
				Expect(round).To(Equal(vc.Round))
			})
		})
	})

	Describe("pre", func() {
		Context("for commit", func() {
			BeforeEach(func() {
				c.Hash = "foofoofoofoo" // minimum 12 char hash :)
				c.Node = 2
				c.Round = 1
				c.View = 1
			})

			It("should return a prePrepare", func() {
				actual := c.Pre()
				expected := messages.PrePrepare{Hash: c.Hash, Node: c.Node, Round: c.Round, View: c.View}

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prepare", func() {
			BeforeEach(func() {
				p.Hash = "foofoofoofoo" // minimum 12 char hash :)
				p.Node = 2
				p.Round = 1
				p.View = 1
			})

			It("should return a prePrepare", func() {
				actual := p.Pre()
				expected := messages.PrePrepare{Hash: p.Hash, Node: p.Node, Round: p.Round, View: p.View}

				Expect(actual).To(Equal(expected))
			})
		})
	})

	Describe("sdKind", func() {
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
		Context("for commit", func() {
			BeforeEach(func() {
				c.Hash = "foofoofoofoo" // minimum 12 char hash :)
				c.Node = 2
				c.Round = 1
				c.Sender = 3
			})

			It("should return a formatted string", func() {
				actual := c.String()
				expected := "commit{node: 2, round: 1, view: 0, hash: '666F6F666F6F', sender: 3}"

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for messageKind", func() {
			type messageKindStringTest struct {
				msgKind  messages.MessageKind
				expected string
			}

			var messageKindStringTests = []messageKindStringTest{
				{messages.CommitMsg, "commit"},
				{messages.NewViewMsg, "new-view"},
				{messages.PrepareMsg, "prepare"},
				{messages.PrePrepareMsg, "pre-prepare"},
				{messages.UnknownMsg, "unknown"},
				{messages.ViewChangedMsg, "view-change"},
			}

			for _, mkt := range messageKindStringTests {
				mkt := mkt

				It("should return a formatted string for each msg type", func() {
					actual := mkt.msgKind.String()
					expected := mkt.expected

					Expect(actual).To(Equal(expected))
				})
			}

			Context("with an incorrect messageKind", func() {
				It("should panic by default", func() {
					var mkt = messageKindStringTest{msgKind: 99}
					Expect(func() { mkt.msgKind.String() }).To(Panic())
				})
			})
		})

		Context("for newView", func() {
			BeforeEach(func() {
				nv.Hash = "foofoofoofoo" // minimum 12 char hash :)
				nv.Node = 2
				nv.Round = 1
				nv.Sender = 3
			})

			It("should return a formatted string", func() {
				actual := nv.String()
				expected := "new-view{node: 2, round: 1, view: 0, hash: '666F6F666F6F', sender: 3}"

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prepare", func() {
			BeforeEach(func() {
				p.Hash = "foofoofoofoo" // minimum 12 char hash :)
				p.Node = 2
				p.Round = 1
				p.Sender = 3
			})

			It("should return a formatted string", func() {
				actual := p.String()
				expected := "prepare{node: 2, round: 1, view: 0, hash: '666F6F666F6F', sender: 3}"

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prePrepare", func() {
			BeforeEach(func() {
				pp.Hash = "foofoofoofoo" // minimum 12 char hash :)
				pp.Node = 2
				pp.Round = 1
			})

			It("should return a formatted string", func() {
				actual := pp.String()
				expected := "pre-prepare{node: 2, round: 1, view: 0, hash: '666F6F666F6F'}"

				Expect(actual).To(Equal(expected))
			})
		})

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

		Context("for viewChange", func() {
			BeforeEach(func() {
				vc.Hash = "foofoofoofoo" // minimum 12 char hash :)
				vc.Node = 2
				vc.Round = 1
				vc.Sender = 5
			})

			It("should return a formatted string", func() {
				actual := vc.String()
				expected := "view-change{node: 2, round: 1, view: 0, hash: '666F6F666F6F', sender: 5}"

				Expect(actual).To(Equal(expected))
			})
		})
	})
})
