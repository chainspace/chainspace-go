package api_test

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"chainspace.io/prototype/pubsub/api"
	mocks "chainspace.io/prototype/pubsub/api/mocks"
	pbmocks "chainspace.io/prototype/pubsub/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

var _ = Describe("Service", func() {
	var pbService *pbmocks.Server
	var wmc *mocks.WriteMessageCloser
	var srv api.Service
	var ctx context.Context
	var cancel func()

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		pbService = &pbmocks.Server{}
		pbService.On("RegisterNotifier",
			mock.AnythingOfType("pubsub.Notifier")).Return()
		wmc = &mocks.WriteMessageCloser{}
		srv = api.NewService(ctx, pbService)
	})

	Describe("Websocket", func() {
		Context("something throw an error", func() {
			When("the context is cancelled", func() {
				BeforeEach(func() {
					wmc.On("Close").Return(nil)
				})

				It("should process the error and correct http status", func() {
					var status int
					var err error
					// waitgroup in order to notify termination of the method
					wg := &sync.WaitGroup{}
					// run in a goroutine as this is a blocking operation
					go func() {
						defer wg.Done()
						status, err = srv.Websocket("addr:port", wmc)
					}()
					wg.Add(1)
					// cancel context
					cancel()
					// wait for function to return
					wg.Wait()
					// check errors
					Expect(err.Error()).To(Equal("context canceled"))
					// it's ok when status is cancelled
					Expect(status).To(Equal(http.StatusOK))
				})
			})

			When("an error happend when closing the WriteMessageCloser", func() {
				BeforeEach(func() {
					wmc.On("Close").Return(errors.New("unable to close the conn"))
				})

				It("should process the error and correct http status", func() {
					var status int
					var err error
					// waitgroup in order to notify termination of the method
					wg := &sync.WaitGroup{}
					// run in a goroutine as this is a blocking operation
					go func() {
						defer wg.Done()
						status, err = srv.Websocket("addr:port", wmc)
					}()
					wg.Add(1)
					// cancel context
					cancel()
					// wait for function to return
					wg.Wait()
					// check errors
					Expect(err.Error()).To(Equal("unable to close the conn"))
					// it's ok when status is cancelled
					Expect(status).To(Equal(http.StatusInternalServerError))
				})
			})

			/*
				When("an error happend when writing to the WriteMessageCloser", func() {
					BeforeEach(func() {
						wmc.On("WriteMessage",
							websocket.TextMessage,
							mock.AnythingOfType("[]uint8"),
						).Return(errors.New("unable to write to the conn"))
					})

					It("should process the error and correct http status", func() {
						var status int
						var err error
						// waitgroup in order to notify termination of the method
						wg := &sync.WaitGroup{}
						// run in a goroutine as this is a blocking operation
						go func() {
							defer wg.Done()
							status, err = srv.Websocket("addr:port", wmc)
						}()
						wg.Add(1)
						// send a message to write
						srv.Callback(internal.Payload{})
						// wait for function to return
						wg.Wait()
						// check errors
						Expect(err.Error()).To(Equal("unable to write to the conn"))
						// it's ok when status is cancelled
						Expect(status).To(Equal(http.StatusInternalServerError))
					})
				})
			*/
		})
	})
})
