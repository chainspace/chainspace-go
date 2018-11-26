package sbac

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Store", func() {
  var key []byte

  BeforeEach(func() {
    key = nil
  })

	Describe("NewBadgerStore", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

  Describe("committedTxnKey", func() {
    BeforeEach(func() {
      key = []byte("foo bar")
    })

    It("should return a commited txn key", func() {
      actual := committedTxnKey(key)
      expected := []uint8{2, 102, 111, 111, 32, 98, 97, 114}
      Expect(actual).To(Equal(expected))
    })
  })

	Describe("finishedTxnKey", func() {
    BeforeEach(func() {
      key = []byte("foo bar")
    })

    It("should return a finished txn key", func() {
      actual := finishedTxnKey(key)
      expected := []uint8{4, 102, 111, 111, 32, 98, 97, 114}
      Expect(actual).To(Equal(expected))
    })
	})

	Describe("objectKey", func() {
    BeforeEach(func() {
      key = []byte("foo bar")
    })

    It("should return an object key", func() {
      actual := objectKey(key)
      expected := []uint8{0, 102, 111, 111, 32, 98, 97, 114}
      Expect(actual).To(Equal(expected))
    })
	})

	Describe("objectStatusKey", func() {
    BeforeEach(func() {
      key = []byte("foo bar")
    })

    It("should return an object status key", func() {
      actual := objectStatusKey(key)
      expected := []uint8{1, 102, 111, 111, 32, 98, 97, 114}
      Expect(actual).To(Equal(expected))
    })
	})

	Describe("seenTxnKey", func() {
    BeforeEach(func() {
      key = []byte("foo bar")
    })

    It("should return a seen txn key", func() {
      actual := seenTxnKey(key)
      expected := []uint8{3, 102, 111, 111, 32, 98, 97, 114}
      Expect(actual).To(Equal(expected))
    })
	})

	Describe("setObjectsInactive", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("createObjects", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("commitTransaction", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("Close", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("CommitTransaction", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("LockObjects", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("UnlockObjects", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("AddTransaction", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("GetTransaction", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("GetObjects", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("DeactivateObjects", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("CreateObjects", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("CreateObject", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("DeleteObjects", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("FinishTransaction", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})

	Describe("TxnFinished", func() {
		It("should do something", func() {
			Skip("Write test(s) for this...")
		})
	})
})
