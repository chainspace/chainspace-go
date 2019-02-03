package api_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"chainspace.io/chainspace-go/kv"
	"chainspace.io/chainspace-go/kv/api"
	kvMocks "chainspace.io/chainspace-go/kv/mocks"
	sbacMocks "chainspace.io/chainspace-go/sbac/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Service", func() {
	var errorMsg string
	var kvService *kvMocks.Service
	var label string
	var sbacService *sbacMocks.Service
	var srv api.Service

	BeforeEach(func() {
		errorMsg = "boom"
		label = "foo"
		kvService = &kvMocks.Service{}
		sbacService = &sbacMocks.Service{}
		srv = api.NewService(kvService, sbacService)
	})

	Describe("GetByLabel", func() {
		Context("something throws an error", func() {
			When("kvStore.Get returns an error", func() {
				BeforeEach(func() {
					kvService.On("Get", []byte(label)).Return(nil, errors.New(errorMsg))
				})

				It("should process the error and correct http status", func() {
					obj, status, err := srv.GetByLabel(label)
					Expect(obj).To(BeNil())
					Expect(status).To(Equal(http.StatusNotFound))
					Expect(err.Error()).To(Equal(errorMsg))
				})
			})

			When("sbac.QueryObjectByVersionID returns an error", func() {
				objectID := []byte("bar")

				BeforeEach(func() {
					kvService.On("Get", []byte(label)).Return(objectID, nil)
					sbacService.On("QueryObjectByVersionID", objectID).Return(nil, errors.New(errorMsg))
				})

				It("should process the error and correct http status", func() {
					obj, status, err := srv.GetByLabel(label)
					Expect(obj).To(BeNil())
					Expect(status).To(Equal(http.StatusBadRequest))
					Expect(err.Error()).To(Equal(errorMsg))
				})
			})

			When("json.Unmarshal returns an error", func() {
				objectID := []byte("bar")
				jsonErrMsg := "invalid character 'b' looking for beginning of value"
				rawObj := []uint8("bar")

				BeforeEach(func() {
					kvService.On("Get", []byte(label)).Return(objectID, nil)
					sbacService.On("QueryObjectByVersionID", objectID).Return(rawObj, nil)
				})

				It("should process the error and correct http status", func() {
					obj, status, err := srv.GetByLabel(label)
					Expect(obj).To(BeNil())
					Expect(status).To(Equal(http.StatusInternalServerError))
					Expect(err.Error()).To(Equal(jsonErrMsg))
				})
			})
		})

		When("everything just works", func() {
			objectID := []byte("bar")
			rawObj := []uint8(`{"foo":"bar"}`)
			var jsonObj interface{}

			BeforeEach(func() {
				kvService.On("Get", []byte(label)).Return(objectID, nil)
				sbacService.On("QueryObjectByVersionID", objectID).Return(rawObj, nil)

				if err := json.Unmarshal(rawObj, &jsonObj); err != nil {
					panic(err)
				}
			})

			It("should process the object and correct http status", func() {
				obj, status, err := srv.GetByLabel(label)
				Expect(obj).To(Equal(jsonObj))
				Expect(status).To(Equal(http.StatusOK))
				Expect(err).To(BeNil())
			})
		})
	})

	Describe("GetByPrefix", func() {
		Context("something throws an error", func() {
			When("kvStore.GetByPrefix returns an error", func() {
				var expectedObj []api.LabelObject

				BeforeEach(func() {
					kvService.On("GetByPrefix", []byte(label)).Return(nil, errors.New(errorMsg))
					expectedObj = nil
				})

				It("should process the error and correct http status", func() {
					obj, status, err := srv.GetByPrefix(label)
					Expect(obj).To(Equal(expectedObj))
					Expect(status).To(Equal(http.StatusNotFound))
					Expect(err.Error()).To(Equal(errorMsg))
				})
			})

			When("sbac.QueryObjectByVersionID returns an error", func() {
				var objEntries []kv.ObjectEntry

				BeforeEach(func() {
					objEntry := kv.ObjectEntry{
						Label:     []byte(label),
						VersionID: []byte(label),
					}
					objEntries = append(objEntries, objEntry)

					kvService.On("GetByPrefix", []byte(label)).Return(objEntries, nil)
					sbacService.On("QueryObjectByVersionID", objEntry.VersionID).Return(nil, errors.New(errorMsg))
				})

				It("should process the error and correct http status", func() {
					obj, status, err := srv.GetByPrefix(label)
					Expect(obj).To(BeNil())
					Expect(status).To(Equal(http.StatusBadRequest))
					Expect(err.Error()).To(Equal(errorMsg))
				})
			})

			When("json.Unmarshal returns an error", func() {
				var objEntries []kv.ObjectEntry
				jsonErrMsg := "invalid character 'b' looking for beginning of value"
				rawObj := []uint8("bar")

				BeforeEach(func() {
					objEntry := kv.ObjectEntry{
						Label:     []byte(label),
						VersionID: []byte(label),
					}
					objEntries = append(objEntries, objEntry)

					kvService.On("GetByPrefix", []byte(label)).Return(objEntries, nil)
					sbacService.On("QueryObjectByVersionID", objEntry.VersionID).Return(rawObj, nil)
				})

				It("should process the error and correct http status", func() {
					obj, status, err := srv.GetByPrefix(label)
					Expect(obj).To(BeNil())
					Expect(status).To(Equal(http.StatusInternalServerError))
					Expect(err.Error()).To(Equal(jsonErrMsg))
				})
			})
		})

		When("everything just works", func() {
			var expectedObjs []api.LabelObject
			var jsonObj interface{}
			var objEntries []kv.ObjectEntry
			rawObj := []uint8(`{"foo":"bar"}`)

			BeforeEach(func() {
				objEntry := kv.ObjectEntry{
					Label:     []byte(label),
					VersionID: []byte(label),
				}
				objEntries = append(objEntries, objEntry)

				if err := json.Unmarshal(rawObj, &jsonObj); err != nil {
					panic(err)
				}

				lObj := api.LabelObject{
					Label:  label,
					Object: jsonObj,
				}
				expectedObjs = append(expectedObjs, lObj)

				kvService.On("GetByPrefix", []byte(label)).Return(objEntries, nil)
				sbacService.On("QueryObjectByVersionID", objEntry.VersionID).Return(rawObj, nil)
			})

			It("should process the objects and correct http status", func() {
				obj, status, err := srv.GetByPrefix(label)
				Expect(obj).To(Equal(expectedObjs))
				Expect(status).To(Equal(http.StatusOK))
				Expect(err).To(BeNil())
			})
		})
	})

	Describe("GetVersionID", func() {
		Context("something throws an error", func() {
			When("kvStore.Get returns an error", func() {
				BeforeEach(func() {
					kvService.On("Get", []byte(label)).Return(nil, errors.New(errorMsg))
				})

				It("should process the error and correct http status", func() {
					obj, status, err := srv.GetVersionID(label)
					Expect(obj).To(Equal(""))
					Expect(status).To(Equal(http.StatusNotFound))
					Expect(err.Error()).To(Equal(errorMsg))
				})
			})
		})

		When("everything just works", func() {
			objectID := []byte("bar")
			encodedVersionID := base64.StdEncoding.EncodeToString(objectID)

			BeforeEach(func() {
				kvService.On("Get", []byte(label)).Return(objectID, nil)
			})

			It("should process the object and correct http status", func() {
				obj, status, err := srv.GetVersionID(label)
				Expect(obj).To(Equal(encodedVersionID))
				Expect(status).To(Equal(http.StatusOK))
				Expect(err).To(BeNil())
			})
		})
	})

	Describe("GetVersionIDByPrefix", func() {
		Context("something throws an error", func() {
			When("kvStore.GetByPrefix returns an error", func() {
				var expectedObj []api.LabelVersionID

				BeforeEach(func() {
					kvService.On("GetByPrefix", []byte(label)).Return(nil, errors.New(errorMsg))
					expectedObj = nil
				})

				It("should process the error and correct http status", func() {
					obj, status, err := srv.GetVersionIDByPrefix(label)
					Expect(obj).To(Equal(expectedObj))
					Expect(status).To(Equal(http.StatusNotFound))
					Expect(err.Error()).To(Equal(errorMsg))
				})
			})
		})

		When("everything just works", func() {
			var objEntries []kv.ObjectEntry
			var expectedObjs []api.LabelVersionID

			BeforeEach(func() {
				objEntry := kv.ObjectEntry{
					Label:     []byte(label),
					VersionID: []byte(label),
				}
				objEntries = append(objEntries, objEntry)

				lvID := api.LabelVersionID{
					Label:     string(objEntry.Label),
					VersionID: base64.StdEncoding.EncodeToString(objEntry.VersionID),
				}
				expectedObjs = append(expectedObjs, lvID)

				kvService.On("GetByPrefix", []byte(label)).Return(objEntries, nil)
			})

			It("should process the objects and correct http status", func() {
				obj, status, err := srv.GetVersionIDByPrefix(label)
				Expect(obj).To(Equal(expectedObjs))
				Expect(status).To(Equal(http.StatusOK))
				Expect(err).To(BeNil())
			})
		})
	})
})
