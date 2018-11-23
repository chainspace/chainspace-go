package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"chainspace.io/prototype/internal/log"
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

// GetByPrefix grabs a values based on the prefix string
func (srv *service) GetByPrefix(
	prefix string) ([]LabelObject, int, error) {
	pairs, err := srv.kvStore.GetByPrefix([]byte(prefix))
	if err != nil {
		return nil, http.StatusNotFound, err
	}

	out := make([]LabelObject, 0, len(pairs))
	for _, v := range pairs {
		rawobject, err :=
			srv.sbac.QueryObjectByVersionID(v.VersionID)
		if err != nil {
			return nil, http.StatusBadRequest, err
		}

		var object interface{}
		err = json.Unmarshal(rawobject, &object)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		lobj := LabelObject{
			Label:  string(v.Label),
			Object: object,
		}
		out = append(out, lobj)
	}

	return out, http.StatusOK, nil
}

// GetVersionID ...
func (srv *service) GetVersionID(label string) (string, int, error) {
	objectID, err := srv.kvStore.Get([]byte(label))
	if err != nil {
		return "", http.StatusNotFound, err
	}

	return base64.StdEncoding.EncodeToString(objectID), http.StatusOK, nil
}

// GetVersionIDByPrefix ...
func (srv *service) GetVersionIDByPrefix(
	prefix string) ([]LabelVersionID, int, error) {
	log.Error("PREFIX: ", log.String("prefix", prefix))
	pairs, err := srv.kvStore.GetByPrefix([]byte(prefix))
	if err != nil {
		return nil, http.StatusNotFound, err
	}
	out := make([]LabelVersionID, 0, len(pairs))
	log.Error("PREFIX RESULT: ", log.Int("pairs", len(pairs)))
	for _, v := range pairs {
		lvid := LabelVersionID{
			Label:     string(v.Label),
			VersionID: base64.StdEncoding.EncodeToString(v.VersionID),
		}
		out = append(out, lvid)
	}

	return out, http.StatusOK, nil
}
