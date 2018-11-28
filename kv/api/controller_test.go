package api_test

import (
	"encoding/json"
	"errors"
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
		When("something with the label exists", func() {
			data := "bar"
			apiResponse := api.ObjectResponse{}

			BeforeEach(func() {
				srvMock.On("GetByLabel", "foo").Return(data, http.StatusOK, nil)
			})

			It("should return some data", func() {
				res, resErr := http.Get(srvr.URL + "/api/kv/label/foo")
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusOK))

				body, bodyErr := ioutil.ReadAll(res.Body)
				Expect(body).ToNot(Equal(""))
				Expect(bodyErr).To(BeNil())
				res.Body.Close()

				jsonErr := json.Unmarshal(body, &apiResponse)
				Expect(jsonErr).To(BeNil())
				Expect(apiResponse.Object).To(Equal(data))
			})
		})

		When("nothing with the label exists", func() {
			data := ""
			apiResponse := api.Error{}

			BeforeEach(func() {
				srvMock.On("GetByLabel", "foo").Return(data, http.StatusBadRequest, errors.New(errorMsg))
			})

			It("should return an error", func() {
				res, resErr := http.Get(srvr.URL + "/api/kv/label/foo")
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusBadRequest))

				body, bodyErr := ioutil.ReadAll(res.Body)
				Expect(body).ToNot(Equal(""))
				Expect(bodyErr).To(BeNil())
				res.Body.Close()

				jsonErr := json.Unmarshal(body, &apiResponse)
				Expect(jsonErr).To(BeNil())
				Expect(apiResponse.Error).To(Equal(errorMsg))
			})
		})
	})

	Describe("/api/kv/label/:label/version-id", func() {
		When("something with the label exists", func() {
			data := "bar"
			apiResponse := api.VersionIDResponse{}

			BeforeEach(func() {
				srvMock.On("GetVersionID", "foo").Return(data, http.StatusOK, nil)
			})

			It("should return some data", func() {
				res, resErr := http.Get(srvr.URL + "/api/kv/label/foo/version-id")
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusOK))

				body, bodyErr := ioutil.ReadAll(res.Body)
				Expect(body).ToNot(Equal(""))
				Expect(bodyErr).To(BeNil())
				res.Body.Close()

				jsonErr := json.Unmarshal(body, &apiResponse)
				Expect(jsonErr).To(BeNil())
				Expect(apiResponse.VersionID).To(Equal(data))
			})
		})

		When("nothing with the label exists", func() {
			data := ""
			apiResponse := api.Error{}

			BeforeEach(func() {
				srvMock.On("GetVersionID", "foo").Return(data, http.StatusBadRequest, errors.New(errorMsg))
			})

			It("should return an error", func() {
				res, resErr := http.Get(srvr.URL + "/api/kv/label/foo/version-id")
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusBadRequest))

				body, bodyErr := ioutil.ReadAll(res.Body)
				Expect(body).ToNot(Equal(""))
				Expect(bodyErr).To(BeNil())
				res.Body.Close()

				jsonErr := json.Unmarshal(body, &apiResponse)
				Expect(jsonErr).To(BeNil())
				Expect(apiResponse.Error).To(Equal(errorMsg))
			})
		})
	})

	Describe("/api/kv/prefix/:prefix", func() {
		When("something with the prefix exists", func() {
			data := []api.LabelObject{}
			apiResponse := api.ListObjectsResponse{}

			BeforeEach(func() {
				obj := api.LabelObject{
					Label:  "foo",
					Object: "bar",
				}
				data = append(data, obj)
				srvMock.On("GetByPrefix", "foo").Return(data, http.StatusOK, nil)
			})

			It("should return some data", func() {
				res, resErr := http.Get(srvr.URL + "/api/kv/prefix/foo")
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusOK))

				body, bodyErr := ioutil.ReadAll(res.Body)
				Expect(body).ToNot(Equal(""))
				Expect(bodyErr).To(BeNil())
				res.Body.Close()

				jsonErr := json.Unmarshal(body, &apiResponse)
				Expect(jsonErr).To(BeNil())
				Expect(apiResponse.Pairs).To(Equal(data))
			})
		})

		When("nothing with the prefix exists", func() {
			apiResponse := api.Error{}

			BeforeEach(func() {
				srvMock.On("GetByPrefix", "foo").Return(nil, http.StatusBadRequest, errors.New(errorMsg))
			})

			It("should return an error", func() {
				res, resErr := http.Get(srvr.URL + "/api/kv/prefix/foo")
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusBadRequest))

				body, bodyErr := ioutil.ReadAll(res.Body)
				Expect(body).ToNot(Equal(""))
				Expect(bodyErr).To(BeNil())
				res.Body.Close()

				jsonErr := json.Unmarshal(body, &apiResponse)
				Expect(jsonErr).To(BeNil())
				Expect(apiResponse.Error).To(Equal(errorMsg))
			})
		})
	})

	Describe("/api/kv/prefix/:prefix/version-id", func() {
		When("something with the prefix exists", func() {
			data := []api.LabelVersionID{}
			apiResponse := api.ListVersionIDsResponse{}

			BeforeEach(func() {
				obj := api.LabelVersionID{
					Label:     "foo",
					VersionID: "bar",
				}
				data = append(data, obj)
				srvMock.On("GetVersionIDByPrefix", "foo").Return(data, http.StatusOK, nil)
			})

			It("should return some data", func() {
				res, resErr := http.Get(srvr.URL + "/api/kv/prefix/foo/version-id")
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusOK))

				body, bodyErr := ioutil.ReadAll(res.Body)
				Expect(body).ToNot(Equal(""))
				Expect(bodyErr).To(BeNil())
				res.Body.Close()

				jsonErr := json.Unmarshal(body, &apiResponse)
				Expect(jsonErr).To(BeNil())
				Expect(apiResponse.Pairs).To(Equal(data))
			})
		})

		When("nothing with the prefix exists", func() {
			apiResponse := api.Error{}

			BeforeEach(func() {
				srvMock.On("GetVersionIDByPrefix", "foo").Return(nil, http.StatusBadRequest, errors.New(errorMsg))
			})

			It("should return an error", func() {
				res, resErr := http.Get(srvr.URL + "/api/kv/prefix/foo/version-id")
				Expect(resErr).To(BeNil())
				Expect(res.StatusCode).To(Equal(http.StatusBadRequest))

				body, bodyErr := ioutil.ReadAll(res.Body)
				Expect(body).ToNot(Equal(""))
				Expect(bodyErr).To(BeNil())
				res.Body.Close()

				jsonErr := json.Unmarshal(body, &apiResponse)
				Expect(jsonErr).To(BeNil())
				Expect(apiResponse.Error).To(Equal(errorMsg))
			})
		})
	})
})
