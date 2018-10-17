package restsrv

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

func readlabel(rw http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to read request: %v", err))
		return nil, false
	}
	req := struct {
		Label string `json:"label"`
	}{}
	if err := json.Unmarshal(body, &req); err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to unmarshal: %v", err))
		return nil, false
	}
	if len(req.Label) <= 0 {
		fail(rw, http.StatusBadRequest, "missing label")
		return nil, false
	}
	// label, err := base64.StdEncoding.DecodeString(req.Label)
	// if err != nil {
	// 	fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to base64decode: %v", err))
	//	return nil, false
	//}
	return []byte(req.Label), true
}

func (s *Service) kvGetObjectID(rw http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(r.Header.Get("Content-Type"), "application/json") {
		fail(rw, http.StatusBadRequest, "unsupported content-type")
		return
	}
	if r.Method != http.MethodPost {
		fail(rw, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	label, ok := readlabel(rw, r)
	if !ok {
		return
	}

	objectID, err := s.store.Get(label)
	if err != nil {
		fail(rw, http.StatusInternalServerError, err.Error())
		return
	}

	res := struct {
		ObjectID string `json:"object_id"`
	}{
		ObjectID: base64.StdEncoding.EncodeToString(objectID),
	}
	success(rw, http.StatusOK, res)
}

func (s *Service) kvGet(rw http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(r.Header.Get("Content-Type"), "application/json") {
		fail(rw, http.StatusBadRequest, "unsupported content-type")
		return
	}
	if r.Method != http.MethodPost {
		fail(rw, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	label, ok := readlabel(rw, r)
	if !ok {
		return
	}

	objectID, err := s.store.Get(label)
	if err != nil {
		fail(rw, http.StatusInternalServerError, err.Error())
		return
	}

	objectraw, err := s.transactor.QueryObjectByKey(objectID)
	if err != nil {
		fail(rw, http.StatusInternalServerError, err.Error())
		return

	}

	var object interface{}
	err = json.Unmarshal(objectraw, &object)
	if err != nil {
		fail(rw, http.StatusInternalServerError, err.Error())
		return

	}

	res := struct {
		Object interface{} `json:"object"`
	}{
		Object: object,
	}

	success(rw, http.StatusOK, res)
}
