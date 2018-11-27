package messages

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Messages", func() {

	var (
		c  Commit
		nv NewView
		p  Prepare
		pp PrePrepare
		vc ViewChange
	)

	BeforeEach(func() {
		c = Commit{}
		nv = NewView{}
		p = Prepare{}
		pp = PrePrepare{}
		vc = ViewChange{}
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

	// TODO: I think we may be able to dry up these tests, given
	// that they are mostly re-testing the same interfaces on different types,
	// and have pretty much the same expected results across Message-conforming
	// types

	Describe("kind", func() {
		Context("for commit", func() {
			It("should return the right msg kind", func() {
				actual := c.Kind()
				expected := CommitMsg

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for newView", func() {
			It("should return the right msg kind", func() {
				actual := nv.Kind()
				expected := NewViewMsg

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prepare", func() {
			It("should return the right msg kind", func() {
				actual := p.Kind()
				expected := PrepareMsg

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for prePrepare", func() {
			It("should return the right msg kind", func() {
				actual := pp.Kind()
				expected := PrePrepareMsg

				Expect(actual).To(Equal(expected))
			})
		})

		Context("for viewChange", func() {
			It("should return the right msg kind", func() {
				actual := vc.Kind()
				expected := ViewChangedMsg

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

	Describe("Pre", func() {
		Context("for commit", func() {
			BeforeEach(func() {
				c.Hash = "foofoofoofoo" // minimum 12 char hash :)
				c.Node = 2
				c.Round = 1
				c.View = 1
			})

			It("should return a prePrepare", func() {
				actual := c.Pre()
				expected := PrePrepare{Hash: c.Hash, Node: c.Node, Round: c.Round, View: c.View}

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
				expected := PrePrepare{Hash: p.Hash, Node: p.Node, Round: p.Round, View: p.View}

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
				msgKind  MessageKind
				expected string
			}

			var messageKindStringTests = []messageKindStringTest{
				{CommitMsg, "commit"},
				{NewViewMsg, "new-view"},
				{PrepareMsg, "prepare"},
				{PrePrepareMsg, "pre-prepare"},
				{UnknownMsg, "unknown"},
				{ViewChangedMsg, "view-change"},
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
