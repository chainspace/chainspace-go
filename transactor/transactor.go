package transactor

import (
	"context"
	"crypto/sha512"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"hash"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/node"

	"github.com/gogo/protobuf/proto"
	"github.com/tav/golly/log"
)

var (
	ErrUnsupportedOperation = errors.New("Unsupported operation (invalid opcode)")
	ErrInvalidRequestFormat = errors.New("Invalid request format (unmarshaling error)")
	ErrInvalidPayloadFormat = errors.New("Invalid payload formad (unmarshaling error)")
	ErrUnknownAlgorithm     = errors.New("Unknown digital signature algorithm")
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

type Transactor struct {
	h               hash.Hash
	state           *node.State
	privkey         signature.PrivateKey
	broadCastObject chan *Object
}

func New(
	state *node.State,
	signingKey *config.Key,
	broadCastObject chan *Object,
) (*Transactor, error) {
	algorithm, err := signature.AlgorithmFromString(signingKey.Type)
	if err != nil {
		return nil, err
	}
	privKeybytes, err := b32.DecodeString(signingKey.Private)
	if err != nil {
		return nil, err
	}
	privkey, err := signature.LoadPrivateKey(algorithm, privKeybytes)
	if err != nil {
		return nil, err
	}

	return &Transactor{
		h:               sha512.New512_256(),
		state:           state,
		privkey:         privkey,
		broadCastObject: broadCastObject,
	}, nil
}

// newError create a new RawMessage with the given error message
func newError(err error) *RawMessage {
	return &RawMessage{
		Op:      OpCode_ERROR,
		Payload: []byte(err.Error()),
	}
}

// HandleMessage unmarshal a raw message and dispatch it to the right action
// depending of the opcode from the message
func (s *Transactor) HandleMessage(ctx context.Context, data []byte) ([]byte, error) {
	raw := &RawMessage{}
	err := proto.Unmarshal(data, raw)
	if err != nil {
		log.Errorf("unmarshaling error: %v", err)
		errRes := newError(ErrInvalidRequestFormat)
		b, err := proto.Marshal(errRes)
		if err != nil {
			log.Errorf("marshaling error: %v", err)
			return nil, err
		}
		return b, nil

	}

	var tempResult *RawMessage
	switch raw.Op {
	case OpCode_TRANSACTION:
		tempResult, err = s.handleTransaction(ctx, raw.Payload)
	case OpCode_QUERY:
		tempResult, err = s.handleQuery(ctx, raw.Payload)
	default:
		tempResult = newError(ErrUnsupportedOperation)
	}

	if err != nil {
		log.Errorf("unable to process transactor message: %v", err)
		tempResult = newError(err)
	}

	result, err := proto.Marshal(tempResult)
	if err != nil {
		log.Errorf("marshaling error: %v", err)
		return nil, err
	}

	return result, nil
}

// handleTransaction unmarshal a transaction and will extract the objects from the transaction,
// create unique keys for the output objects then send them to be broadcasted to other nodes.
func (s *Transactor) handleTransaction(ctx context.Context, msg []byte) (*RawMessage, error) {
	tx := &Transaction{}
	err := proto.Unmarshal(msg, tx)
	if err != nil {
		log.Errorf("handleTransaction: Unable to unmarshal payload proto: %v", err)
		return nil, ErrInvalidPayloadFormat
	}

	objects := []*Object{}
	results := []*Object{}
	for _, trace := range tx.Traces {
		baseKey := []byte{}
		for _, inputObject := range trace.InputObjects {
			// add object to the key
			baseKey = append(baseKey, inputObject.Key...)
			// add object to the list of objects to broadcast later
			obj, err := s.newInputObject(inputObject.Data, inputObject.Key)
			if err != nil {
				return nil, err
			}
			objects = append(objects, obj)
		}
		for i, outputObject := range trace.OutputObjects {
			key := make([]byte, len(baseKey))
			copy(key, baseKey)

			// append the object data to the key
			key = append(key, outputObject.Data...)
			indexBuf := make([]byte, 4)
			// append the object index to the key
			binary.LittleEndian.PutUint32(indexBuf, uint32(i))
			key = append(key, indexBuf...)

			obj, err := s.newOutputObject(outputObject.Data, key)
			if err != nil {
				return nil, err
			}
			objects = append(objects, obj)
			results = append(results, obj)
		}
	}

	// broadcast objects to be created (outputObjects)
	// and consumed objects / to become inactive (inputsObjects)
	for _, o := range objects {
		s.broadCastObject <- o
	}

	res := &TransactionResult{
		Objects: results,
	}
	payload, err := proto.Marshal(res)
	if err != nil {
		return nil, err
	}
	payloadhash, err := s.hash(payload)
	if err != nil {
		return nil, err
	}
	return &RawMessage{
		Op:        OpCode_TRANSACTION_RESULT,
		Payload:   payload,
		Signature: s.privkey.Sign(payloadhash),
	}, err
}

// newInputObject create a new object with the same existing data, just adding a signature
func (s *Transactor) newInputObject(data, key []byte) (*Object, error) {
	objhash, err := s.hash(data)
	if err != nil {
		return nil, err
	}

	obj := &Object{
		Data:      data,
		Key:       key,
		Signature: s.privkey.Sign(objhash),
	}
	return obj, nil
}

// newOutputObject create new output object using the key created from the transaction objects
// and add the signature
func (s *Transactor) newOutputObject(data, key []byte) (*Object, error) {
	objhash, err := s.hash(data)
	if err != nil {
		return nil, err
	}
	keyhash, err := s.hash(key)
	if err != nil {
		return nil, err
	}
	obj := &Object{
		Data:      data,
		Key:       keyhash,
		Signature: s.privkey.Sign(objhash),
	}
	return obj, nil
}

func (s *Transactor) hash(b []byte) ([]byte, error) {
	s.h.Reset()
	if _, err := s.h.Write(b); err != nil {
		return nil, err
	}
	return s.h.Sum(nil), nil

}

func (s *Transactor) handleQuery(ctx context.Context, msg []byte) (*RawMessage, error) {
	err := errors.New("query: operation not implemented")
	return newError(err), err
}
