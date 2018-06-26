package transactorserver

import (
	"context"
	"errors"

	"chainspace.io/prototype/transactor"

	"github.com/gogo/protobuf/proto"
	"github.com/tav/golly/log"
)

var (
	ErrUnsupportedOperation = errors.New("Unsupported operation (invalid opcode)")
	ErrInvalidRequestFormat = errors.New("Invalid request format (unmarshaling error)")
)

type Server struct{}

func newError(err error) *transactor.RawMessage {
	return &transactor.RawMessage{
		Op:      transactor.OpCode_ERROR,
		Payload: []byte(err.Error()),
	}
}

func (s *Server) HandleMessage(ctx context.Context, data []byte) ([]byte, error) {
	raw := &transactor.RawMessage{}
	err := proto.Unmarshal(data, raw)
	if err != nil {
		log.Errorf("unmarshaling error: %v", err)
		errMsg := newError(ErrInvalidRequestFormat)
		b, err := proto.Marshal(errMsg)
		if err != nil {
			log.Errorf("marshaling error: %v", err)
			return nil, err
		}
		return b, nil

	}

	var tempResult *transactor.RawMessage
	switch raw.Op {
	case transactor.OpCode_TRANSACTION:
		tempResult, err = s.handleTransaction(ctx)
	case transactor.OpCode_QUERY:
		tempResult, err = s.handleQuery(ctx)
	default:
		tempResult = newError(ErrUnsupportedOperation)
	}

	if err != nil {
		log.Errorf("unable to process transactor message: %v", err)
	}

	result, err := proto.Marshal(tempResult)
	if err != nil {
		log.Errorf("marshaling error: %v", err)
		return nil, err
	}

	return result, nil
}

func (s *Server) handleTransaction(ctx context.Context) (*transactor.RawMessage, error) {
	err := errors.New("transaction: operation not implemented")
	return newError(err), err
}

func (s *Server) handleQuery(ctx context.Context) (*transactor.RawMessage, error) {
	err := errors.New("query: operation not implemented")
	return newError(err), err
}
