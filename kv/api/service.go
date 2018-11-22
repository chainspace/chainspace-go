package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"chainspace.io/prototype/kv"
	"chainspace.io/prototype/sbac"
)

// Service is the Key-Value srv
type service struct {
	kvStore kv.Service
	sbac    *sbac.Service
}

// newService ...
func newService(kvStore kv.Service, sbac *sbac.Service) *service {
	return &service{
		kvStore: kvStore,
		sbac:    sbac,
	}
}

// Get grabs a value from the store based on its label
func (srv *service) Get(label string) (interface{}, int, error) {
	objectID, err := srv.kvStore.Get([]byte(label))
	if err != nil {
		return nil, http.StatusNotFound, err
	}

	rawobject, err := srv.sbac.QueryObjectByVersionID(objectID)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	var object interface{}
	err = json.Unmarshal(rawobject, &object)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return object, http.StatusOK, nil
}

// GetVersionID ...
func (srv *service) GetVersionID(label string) (string, int, error) {
	objectID, err := srv.kvStore.Get([]byte(label))
	if err != nil {
		return "", http.StatusNotFound, err
	}

	return base64.StdEncoding.EncodeToString(objectID), http.StatusOK, nil
}
