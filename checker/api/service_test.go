package api_test

import (
	"context"
	"errors"
	"net/http"

	"chainspace.io/prototype/checker/api"
	checkerMocks "chainspace.io/prototype/checker/mocks"
	sbacapi "chainspace.io/prototype/sbac/api"
	sbacapiMocks "chainspace.io/prototype/sbac/api/mocks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

var _ = Describe("Service", func() {
	var checkerServiceMock *checkerMocks.Service
	var nodeID uint64
	var validatorMock *sbacapiMocks.TransactionValidator
	var srv api.Service
	var tc sbacapi.Trace
	var tx sbacapi.Transaction

	BeforeEach(func() {
		checkerServiceMock = &checkerMocks.Service{}
		nodeID = uint64(2)
		validatorMock = &sbacapiMocks.TransactionValidator{}
		srv = api.NewService(checkerServiceMock, nodeID, validatorMock)
		tc = sbacapi.Trace{}
		tx = sbacapi.Transaction{}
	})

	Describe("Check", func() {
		Context("error gets thrown", func() {
			When("a transaction has no traces", func() {
				It("should return an error and http status code", func() {
					response, status, err := srv.Check(context.TODO(), &tx)
					Expect(response).To(BeNil())
					Expect(status).To(Equal(http.StatusBadRequest))
					Expect(err.Error()).To(Equal("Transaction should have at least one trace"))
				})
			})

			When("the transcation to SBAC returns an error", func() {
				BeforeEach(func() {
					tc.InputObjectVersionIDs = []string{"foo", "bar"}
					tx.Traces = append(tx.Traces, tc)
					validatorMock.On("Validate", &tx).Return(errors.New("boom"))
				})

				It("should return an error and http status code", func() {
					response, status, err := srv.Check(context.TODO(), &tx)
					Expect(response).To(BeNil())
					Expect(status).To(Equal(http.StatusBadRequest))
					Expect(err.Error()).To(Equal("boom"))
				})
			})

			When("the check and sign returns an error", func() {
				BeforeEach(func() {
					tc.InputObjectVersionIDs = []string{"foo", "bar"}
					tx.Traces = append(tx.Traces, tc)
					validatorMock.On("Validate", &tx).Return(nil)

					checkerServiceMock.On("CheckAndSign", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*sbac.Transaction")).Return(false, nil, errors.New("boom2"))
				})

				It("should return an error and http status code", func() {
					response, status, err := srv.Check(context.TODO(), &tx)
					Expect(response).To(BeNil())
					Expect(status).To(Equal(http.StatusInternalServerError))
					Expect(err.Error()).To(Equal("boom2"))
				})
			})
		})

		When("everything just works", func() {
			BeforeEach(func() {
				validatorMock.On("Validate", &transactionFixture).Return(nil)

				signature := []uint8("signme")
				checkerServiceMock.On("CheckAndSign", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*sbac.Transaction")).Return(true, signature, nil)
			})

			It("should return the correct response and http status code", func() {
				response, status, err := srv.Check(context.TODO(), &transactionFixture)
				expectedResponse := api.CheckTransactionResponse{NodeID: 2, OK: true, Signature: "c2lnbm1l"}
				Expect(response).To(Equal(expectedResponse))
				Expect(status).To(Equal(http.StatusOK))
				Expect(err).To(BeNil())
			})
		})
	})
})
