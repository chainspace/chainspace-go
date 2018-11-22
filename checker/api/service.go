package api

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"chainspace.io/prototype/checker"
	"chainspace.io/prototype/internal/log"
)

// Service is the Key-Value srv
type service struct {
	checkr *checker.Service
	nodeID uint64
}

// newService ...
func newService(checkr *checker.Service, nodeID uint64) *service {
	return &service{
		checkr: checkr,
		nodeID: nodeID,
	}
}

// Check grabs a value from the store based on its label
func (srv *service) Check(ctx context.Context, tx *Transaction) (interface{}, int, error) {
	now := time.Now()

	for _, v := range tx.Traces {
		if len(v.InputObjectVersionIDs) <= 0 {
			return nil, http.StatusBadRequest, errors.New("Missing input version ID")
		}
	}

	sbactx, err := tx.ToSBAC()
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	ok, signature, err := srv.checkr.CheckAndSign(ctx, sbactx)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	resp := CheckTransactionResponse{
		NodeID:    srv.nodeID,
		OK:        ok,
		Signature: base64.StdEncoding.EncodeToString(signature),
	}

	log.Error("new transaction checked", log.String("time.taken", time.Since(now).String()))
	return resp, http.StatusOK, nil
}
