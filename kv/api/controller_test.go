package api_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"chainspace.io/prototype/kv/api"
	serviceMocks "chainspace.io/prototype/kv/api/mocks"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Controller", func() {
	var ctrl api.Controller
	var errorMsg string = "boom"
	var srvMock *serviceMocks.Service
	var srvr *httptest.Server
	var url string

	BeforeEach(func() {
		gin.SetMode("test")

		srvMock = &serviceMocks.Service{}
		ctrl = api.NewWithService(srvMock)

		router := gin.Default()
		ctrl.RegisterRoutes(router)
		srvr = httptest.NewServer(router)
	})

	AfterEach(func() {
		srvr.Close()
	})

	Describe("/api/kv/label/:label", func() {
		BeforeEach(func() {
			url = fmt.Sprintf("%v/api/kv/label/foo", srvr.URL)
		})

		When("something with the label exists", func() {
			BeforeEach(func() {
				srvMock.On("GetByLabel", "foo").Return("bar", http.StatusOK, nil)
			})

			It("should return some data", func() {
				res, resErr := http.Get(url)
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusOK))

				actualJSON, _ := ioutil.ReadAll(res.Body)
				expectedJSON := []uint8(`{"object":"bar"}`)
				Expect(actualJSON).To(Equal(expectedJSON))
				res.Body.Close()
			})
		})

		When("nothing with the label exists", func() {
			BeforeEach(func() {
				srvMock.On("GetByLabel", "foo").Return("", http.StatusBadRequest, errors.New(errorMsg))
			})

			It("should return an error", func() {
				res, resErr := http.Get(url)
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusBadRequest))

				actualJSON, _ := ioutil.ReadAll(res.Body)
				expectedJSON := []uint8(`{"error":"boom"}`)
				Expect(actualJSON).To(Equal(expectedJSON))
				res.Body.Close()
			})
		})
	})

	Describe("/api/kv/label/:label/version-id", func() {
		BeforeEach(func() {
			url = fmt.Sprintf("%v/api/kv/label/foo/version-id", srvr.URL)
		})

		When("something with the label exists", func() {
			BeforeEach(func() {
				srvMock.On("GetVersionID", "foo").Return("bar", http.StatusOK, nil)
			})

			It("should return some data", func() {
				res, resErr := http.Get(url)
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusOK))

				actualJSON, _ := ioutil.ReadAll(res.Body)
				expectedJSON := []uint8(`{"version_id":"bar"}`)
				Expect(actualJSON).To(Equal(expectedJSON))
				res.Body.Close()
			})
		})

		When("nothing with the label exists", func() {
			BeforeEach(func() {
				srvMock.On("GetVersionID", "foo").Return("", http.StatusBadRequest, errors.New(errorMsg))
			})

			It("should return an error", func() {
				res, resErr := http.Get(url)
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusBadRequest))

				actualJSON, _ := ioutil.ReadAll(res.Body)
				expectedJSON := []uint8(`{"error":"boom"}`)
				Expect(actualJSON).To(Equal(expectedJSON))
				res.Body.Close()
			})
		})
	})

	Describe("/api/kv/prefix/:prefix", func() {
		BeforeEach(func() {
			url = fmt.Sprintf("%v/api/kv/prefix/foo", srvr.URL)
		})

		When("something with the prefix exists", func() {
			BeforeEach(func() {
				obj := api.LabelObject{
					Label:  "foo",
					Object: "bar",
				}
				data := []api.LabelObject{}
				data = append(data, obj)
				srvMock.On("GetByPrefix", "foo").Return(data, http.StatusOK, nil)
			})

			It("should return some data", func() {
				res, resErr := http.Get(url)
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusOK))

				actualJSON, _ := ioutil.ReadAll(res.Body)
				expectedJSON := []uint8(`{"pairs":[{"label":"foo","object":"bar"}]}`)
				Expect(actualJSON).To(Equal(expectedJSON))
				res.Body.Close()
			})
		})

		When("nothing with the prefix exists", func() {
			BeforeEach(func() {
				srvMock.On("GetByPrefix", "foo").Return(nil, http.StatusBadRequest, errors.New(errorMsg))
			})

			It("should return an error", func() {
				res, resErr := http.Get(url)
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusBadRequest))

				actualJSON, _ := ioutil.ReadAll(res.Body)
				expectedJSON := []uint8(`{"error":"boom"}`)
				Expect(actualJSON).To(Equal(expectedJSON))
				res.Body.Close()
			})
		})
	})

	Describe("/api/kv/prefix/:prefix/version-id", func() {
		BeforeEach(func() {
			url = fmt.Sprintf("%v/api/kv/prefix/foo/version-id", srvr.URL)
		})

		When("something with the prefix exists", func() {
			BeforeEach(func() {
				obj := api.LabelVersionID{
					Label:     "foo",
					VersionID: "bar",
				}
				data := []api.LabelVersionID{}
				data = append(data, obj)
				srvMock.On("GetVersionIDByPrefix", "foo").Return(data, http.StatusOK, nil)
			})

			It("should return some data", func() {
				res, resErr := http.Get(url)
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusOK))

				actualJSON, _ := ioutil.ReadAll(res.Body)
				expectedJSON := []uint8(`{"pairs":[{"label":"foo","version_id":"bar"}]}`)
				Expect(actualJSON).To(Equal(expectedJSON))
				res.Body.Close()
			})
		})

		When("nothing with the prefix exists", func() {
			BeforeEach(func() {
				srvMock.On("GetVersionIDByPrefix", "foo").Return(nil, http.StatusBadRequest, errors.New(errorMsg))
			})

			It("should return an error", func() {
				res, resErr := http.Get(url)
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusBadRequest))

				actualJSON, _ := ioutil.ReadAll(res.Body)
				expectedJSON := []uint8(`{"error":"boom"}`)
				Expect(actualJSON).To(Equal(expectedJSON))
				res.Body.Close()
			})
		})
	})
})
