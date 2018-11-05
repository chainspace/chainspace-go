package restsrv

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

func (s *Service) checkTransaction(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(rw, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !strings.EqualFold(r.Header.Get("Content-Type"), "application/json") {
		fail(rw, http.StatusBadRequest, "unsupported content-type")
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to read request: %v", err))
		return
	}
	req := Transaction{}
	if err := json.Unmarshal(body, &req); err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to unmarshal: %v", err))
		return
	}
	// require at least input object.
	for _, v := range req.Traces {
		if len(v.InputObjectVersionIDs) <= 0 {
			fail(rw, http.StatusBadRequest, "no input objects for a trace")
			return
		}
	}
	tx, err := req.ToSBAC()
	if err != nil {
		fail(rw, http.StatusBadRequest, err.Error())
		return
	}

	ok, signature, err := s.checker.CheckAndSign(r.Context(), tx)
	if err != nil {
		errorr(rw, http.StatusInternalServerError, err.Error())
		return
	}
	resp := struct {
		OK         bool   `json:"ok"`
		Signatures string `json:"signature"`
	}{
		OK:         ok,
		Signatures: base64.StdEncoding.EncodeToString(signature),
	}
	success(rw, http.StatusOK, resp)
}
