package api_test

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"chainspace.io/prototype/pubsub/api"
	"chainspace.io/prototype/pubsub/api/mocks"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

var _ = Describe("Controller", func() {
	var ctrl api.Controller
	var srvr *httptest.Server
	var srvMock = &mocks.Service{}
	var upgraderMock = &mocks.WSUpgrader{}
	var wmc = &mocks.WriteMessageCloser{}
	var ctx context.Context
	var cancel func()
	var url string

	BeforeEach(func() {
		gin.SetMode("test")

		ctx, cancel = context.WithCancel(context.Background())
		srvMock = &mocks.Service{}
		wmc = &mocks.WriteMessageCloser{}
		upgraderMock = &mocks.WSUpgrader{}
		ctrl = api.NewWithServiceAndWSUpgrader(
			ctx, srvMock, upgraderMock)

		router := gin.Default()
		ctrl.RegisterRoutes(router)
		srvr = httptest.NewServer(router)
	})

	AfterEach(func() {
		srvr.Close()
	})

	Describe("/api/pubsub/ws", func() {
		BeforeEach(func() {
			url = fmt.Sprintf("%v/api/pubsub/ws", srvr.URL)
		})

		AfterEach(func() {
			cancel()
		})

		When("an error happen tyring to upgrade the connection", func() {
			BeforeEach(func() {
				upgraderMock.On("Upgrade",
					mock.AnythingOfType("*gin.responseWriter"),
					mock.AnythingOfType("*http.Request")).
					Return(nil, errors.New("unable to upgrade connection"))
			})

			It("should return an error", func() {
				res, resErr := http.Get(url)
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusInternalServerError))

				actualJSON, _ := ioutil.ReadAll(res.Body)
				expectedJSON := []uint8(`{"error":"unable to upgrade connection"}`)
				Expect(actualJSON).To(Equal(expectedJSON))
				res.Body.Close()
			})
		})

		When("an error happen in the service", func() {
			BeforeEach(func() {
				upgraderMock.On("Upgrade",
					mock.AnythingOfType("*gin.responseWriter"),
					mock.AnythingOfType("*http.Request")).
					Return(wmc, nil)
				srvMock.On("Websocket",
					mock.AnythingOfType("string"),
					wmc,
				).Return(
					http.StatusInternalServerError,
					errors.New("websocket closed"))
			})

			It("should return an error", func() {
				res, resErr := http.Get(url)
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusInternalServerError))

				actualJSON, _ := ioutil.ReadAll(res.Body)
				expectedJSON := []uint8(`{"error":"websocket closed"}`)
				Expect(actualJSON).To(Equal(expectedJSON))
				res.Body.Close()
			})
		})

	})
})
