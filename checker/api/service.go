package api

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"

	"chainspace.io/prototype/checker"
)

// Service is the Key-Value srv
type service struct {
	checkr    checker.Service
	nodeID    uint64
	validator TransactionValidator
}

// Service interface
type Service interface {
	Check(ctx context.Context, tx *Transaction) (interface{}, int, error)
}

// NewService ...
func NewService(checkr checker.Service, nodeID uint64, validator TransactionValidator) Service {
	return &service{
		checkr:    checkr,
		nodeID:    nodeID,
		validator: validator,
	}
}

// Check grabs a value from the store based on its label
func (srv *service) Check(ctx context.Context, tx *Transaction) (interface{}, int, error) {
	if len(tx.Traces) <= 0 {
		return nil, http.StatusBadRequest, errors.New("Transaction should have at least one trace")
	}

	sbactx, err := tx.ToSBAC(srv.validator) // TODO add validator
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

	return resp, http.StatusOK, nil
}
