package api_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"chainspace.io/chainspace-go/checker/api"
	"chainspace.io/chainspace-go/checker/api/mocks"
	sbacapi "chainspace.io/chainspace-go/sbac/api"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

var _ = Describe("Controller", func() {
	var ctrl api.Controller
	var errorMsg string = "boom"
	var jsonContentType = "application/json"
	var srvMock *mocks.Service
	var srvr *httptest.Server
	var tx sbacapi.Transaction
	var url string

	BeforeEach(func() {
		gin.SetMode("test")

		srvMock = &mocks.Service{}
		ctrl = api.NewWithService(srvMock)

		router := gin.Default()
		ctrl.RegisterRoutes(router)
		srvr = httptest.NewServer(router)
	})

	AfterEach(func() {
		srvr.Close()
	})

	Describe("/api/checker/check", func() {
		BeforeEach(func() {
			tx = sbacapi.Transaction{}
			url = fmt.Sprintf("%v/api/checker/check", srvr.URL)
		})

		When("something goes wrong binding the tx", func() {
			It("should return an error", func() {
				res, resErr := http.Post(url, jsonContentType, bytes.NewBuffer([]byte("foo")))
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusBadRequest))

				actualJSON, _ := ioutil.ReadAll(res.Body)
				expectedJSON := []uint8(`{"error":"invalid character 'o' in literal false (expecting 'a')"}`)
				Expect(actualJSON).To(Equal(expectedJSON))
				res.Body.Close()
			})
		})

		When("something goes wrong with the check", func() {
			BeforeEach(func() {
				srvMock.On("Check", mock.AnythingOfType("*context.cancelCtx"), &tx).Return(nil, http.StatusBadRequest, errors.New(errorMsg))
			})

			It("should return an error", func() {
				res, resErr := http.Post(url, jsonContentType, bytes.NewBuffer([]byte(`{"foo":"bar"}`)))
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusBadRequest))

				actualJSON, _ := ioutil.ReadAll(res.Body)
				expectedJSON := []uint8(`{"error":"boom"}`)
				Expect(actualJSON).To(Equal(expectedJSON))
				res.Body.Close()
			})
		})

		When("everything just works", func() {
			BeforeEach(func() {
				someting := sbacapi.Transaction{}
				srvMock.On("Check", mock.AnythingOfType("*context.cancelCtx"), &tx).Return(someting, http.StatusBadRequest, nil)
			})

			It("should return json", func() {
				res, resErr := http.Post(url, jsonContentType, bytes.NewBuffer([]byte(`{"foo":"bar"}`)))
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusOK))

				actualJSON, _ := ioutil.ReadAll(res.Body)
				expectedJSON := []uint8(`{"mappings":null,"signatures":null,"traces":null}`)
				Expect(actualJSON).To(Equal(expectedJSON))
				res.Body.Close()
			})
		})
	})
})
