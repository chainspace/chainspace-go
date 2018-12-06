package api_test

import (
	"fmt"

	"chainspace.io/prototype/sbac/api"
	"github.com/getlantern/deepcopy"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TransactionValidator", func() {
	var tx api.Transaction
	var validator api.TransactionValidator

	Describe("Validate", func() {
		BeforeEach(func() {
			deepcopy.Copy(&tx, &transactionFixture)
			validator = api.Validator{}
		})

		Context("a ValidateTrace error gets thrown", func() {
			When("there is a problem with a dependency", func() {
				BeforeEach(func() {
					tx.Traces[0].Dependencies[0].InputObjectVersionIDs = []string{"foo"}
				})

				It("should return an error", func() {
					err := validator.Validate(&tx)
					Expect(err.Error()).To(Equal("Missing object mapping for key [foo]"))
				})
			})

			When("there are no InputObjectVersionIDs", func() {
				BeforeEach(func() {
					tx.Traces[0].InputObjectVersionIDs = []string{}
					fmt.Printf("InputObjectVersionIDs: %#v\n", tx.Traces[0].InputObjectVersionIDs)
				})

				It("should return an error", func() {
					err := validator.Validate(&tx)
					Expect(err.Error()).To(Equal("Missing input version ID"))
				})
			})

			When("there is a problem with InputObjectVersionIDs", func() {
				BeforeEach(func() {
					tx.Traces[0].InputObjectVersionIDs = []string{"foo"}
				})

				It("should return an error", func() {
					err := validator.Validate(&tx)
					Expect(err.Error()).To(Equal("Missing object mapping for key [foo]"))
				})
			})

			When("there is a problem with InputReferenceVersionIDs", func() {
				BeforeEach(func() {
					tx.Traces[0].InputReferenceVersionIDs = []string{"foo"}
				})

				It("should return an error", func() {
					err := validator.Validate(&tx)
					Expect(err.Error()).To(Equal("Missing reference mapping for key [foo]"))
				})
			})
		})

		When("everything just works", func() {
			It("should return nil", func() {
				err := validator.Validate(&transactionFixture)
				Expect(err).To(BeNil())
			})
		})
	})
})
