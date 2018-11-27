package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"chainspace.io/prototype/kv"
	"chainspace.io/prototype/sbac"
)

// ServiceImp is the Key-Value srv
type ServiceImp struct {
	kvStore kv.Service
	sbac    sbac.Service
}

// Service interface
type Service interface {
	GetByLabel(label string) (interface{}, int, error)
	GetByPrefix(prefix string) ([]LabelObject, int, error)
	GetVersionID(label string) (string, int, error)
	GetVersionIDByPrefix(prefixz string) ([]LabelVersionID, int, error)
}

// NewService ...
func NewService(kvStore kv.Service, sbac sbac.Service) *ServiceImp {
	return &ServiceImp{
		kvStore: kvStore,
		sbac:    sbac,
	}
}

// GetByLabel grabs a value from the store based on its label
func (srv *ServiceImp) GetByLabel(label string) (interface{}, int, error) {
	versionID, err := srv.kvStore.Get([]byte(label))
	if err != nil {
		return nil, http.StatusNotFound, err
	}

	rawObject, err := srv.sbac.QueryObjectByVersionID(versionID)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	var object interface{}
	if err = json.Unmarshal(rawObject, &object); err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return object, http.StatusOK, nil
}

// GetByPrefix grabs a values based on the prefix string
func (srv *ServiceImp) GetByPrefix(prefix string) ([]LabelObject, int, error) {
	objs, err := srv.kvStore.GetByPrefix([]byte(prefix))
	if err != nil {
		return nil, http.StatusNotFound, err
	}

	out := make([]LabelObject, 0, len(objs))
	for _, v := range objs {
		rawObject, err := srv.sbac.QueryObjectByVersionID(v.VersionID)
		if err != nil {
			return nil, http.StatusBadRequest, err
		}

		var object interface{}
		if err = json.Unmarshal(rawObject, &object); err != nil {
			return nil, http.StatusInternalServerError, err
		}

		lObj := LabelObject{
			Label:  string(v.Label),
			Object: object,
		}
		out = append(out, lObj)
	}

	return out, http.StatusOK, nil
}

// GetVersionID ...
func (srv *ServiceImp) GetVersionID(label string) (string, int, error) {
	objectID, err := srv.kvStore.Get([]byte(label))
	if err != nil {
		return "", http.StatusNotFound, err
	}

	return base64.StdEncoding.EncodeToString(objectID), http.StatusOK, nil
}

// GetVersionIDByPrefix ...
func (srv *ServiceImp) GetVersionIDByPrefix(prefix string) ([]LabelVersionID, int, error) {
	objs, err := srv.kvStore.GetByPrefix([]byte(prefix))
	if err != nil {
		return nil, http.StatusNotFound, err
	}

	out := make([]LabelVersionID, 0, len(objs))
	for _, obj := range objs {
		lvID := LabelVersionID{
			Label:     string(obj.Label),
			VersionID: base64.StdEncoding.EncodeToString(obj.VersionID),
		}
		out = append(out, lvID)
	}

	return out, http.StatusOK, nil
}
