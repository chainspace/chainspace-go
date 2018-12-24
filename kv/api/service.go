package api

import (
	"encoding/base64"
	"net/http"

	"chainspace.io/prototype/kv"
	"chainspace.io/prototype/sbac"
)

// service is the Key-Value srv
type service struct {
	kvStore kv.Service
	sbac    sbac.Service
}

// Service interface
type Service interface {
	GetByLabel(label string) (LabelObject, int, error)
	GetByPrefix(prefix string) ([]LabelObject, int, error)
	GetVersionID(label string) (string, int, error)
	GetVersionIDByPrefix(prefixz string) ([]LabelVersionID, int, error)
}

// NewService ...
func NewService(kvStore kv.Service, sbac sbac.Service) Service {
	return &service{
		kvStore: kvStore,
		sbac:    sbac,
	}
}

// GetByLabel grabs a value from the store based on its label
func (srv *service) GetByLabel(label string) (LabelObject, int, error) {
	versionID, err := srv.kvStore.Get([]byte(label))
	if err != nil {
		return LabelObject{}, http.StatusNotFound, err
	}

	rawObject, err := srv.sbac.QueryObjectByVersionID(versionID)
	if err != nil {
		return LabelObject{}, http.StatusBadRequest, err
	}

	lObj := LabelObject{
		Label:     string(label),
		Object:    string(rawObject),
		VersionID: base64.StdEncoding.EncodeToString(versionID),
	}

	return lObj, http.StatusOK, nil
}

// GetByPrefix grabs a values based on the prefix string
func (srv *service) GetByPrefix(prefix string) ([]LabelObject, int, error) {
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

		lObj := LabelObject{
			Label:     string(v.Label),
			Object:    string(rawObject),
			VersionID: base64.StdEncoding.EncodeToString(v.VersionID),
		}
		out = append(out, lObj)
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
func (srv *service) GetVersionIDByPrefix(prefix string) ([]LabelVersionID, int, error) {
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
