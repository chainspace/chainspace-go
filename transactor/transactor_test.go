package transactor

import (
	"testing"

	"github.com/gogo/protobuf/proto"
	"golang.org/x/net/context"
)

func marshalMessage(t *testing.T, m *RawMessage) []byte {
	b, err := proto.Marshal(m)
	if err != nil {
		t.Fatalf("unable to marshal test msg: %v", err)
	}
	return b
}

func TestHandleTransaction(t *testing.T) {
	testCases := []struct {
		Data     []byte
		Expected string
	}{
		{
			Data:     []byte("not a valid message"),
			Expected: ErrInvalidRequestFormat.Error(),
		},
		{
			Data: marshalMessage(t, &RawMessage{
				Op:      OpCode_TRANSACTION,
				Payload: []byte("testing"),
			}),
			Expected: "transaction: operation not implemented",
		},
	}

	s := Transactor{}

	for _, tc := range testCases {
		resB, err := s.HandleMessage(context.Background(), tc.Data)
		if err != nil {
			t.Fatalf("unexpected error from s.HandleMessage: %v", err)
		}

		resMsg := &RawMessage{}
		err = proto.Unmarshal(resB, resMsg)
		if err != nil {
			t.Fatalf("unable to unmarshal result from s.HandleMessage: %v", err)
		}

		if string(resMsg.Payload) != tc.Expected {
			t.Fatalf("invalid return value from s.HandleMessage, expected: %v got %v", tc.Expected, string(resMsg.Payload))
		}
	}
}
