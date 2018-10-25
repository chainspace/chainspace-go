package checker // import "chainspace.io/prototype/checker"

import (
	"context"
	"encoding/base32"
	"fmt"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
	"chainspace.io/prototype/service"
	transactor "chainspace.io/prototype/transactor"
	"github.com/gogo/protobuf/proto"
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

type Config struct {
	SigningKey *config.Key
	Checkers   []Checker
}

type Service struct {
	checkers checkersMap
	privkey  signature.PrivateKey
}

// checkersMap map contractID to map check procedure name to checker
type checkersMap map[string]map[string]Checker

func (s *Service) Name() string { return "checker" }

func (s *Service) Handle(peerID uint64, m *service.Message) (*service.Message, error) {
	ctx := context.TODO()
	switch Opcode(m.Opcode) {
	case Opcode_CHECK:
		log.Error("CALLING CHECKER")
		return s.check(ctx, m.Payload, m.ID)
	default:
		log.Error("checker: unknown message opcode",
			log.Int32("opcode", m.Opcode),
			fld.PeerID(peerID), log.Int("len", len(m.Payload)))
		return nil, fmt.Errorf("checker: unknown message opcode: %v", m.Opcode)
	}
}

func (s *Service) Check(ctx context.Context, tx *transactor.Transaction) (bool, error) {
	if err := typeCheck(idmap{}, tx.Traces); err != nil {
		return false, err
	}
	ok, err := run(ctx, s.checkers, tx)
	if err != nil {
		return false, fmt.Errorf("checker error [err=%v]", err)
	}
	return ok, nil
}

func (s *Service) check(
	ctx context.Context, payload []byte, msgID uint64) (*service.Message, error) {
	req := &CheckRequest{}
	err := proto.Unmarshal(payload, req)
	if err != nil {
		log.Error("checker: unmarshal error", fld.Err(err))
		return nil, fmt.Errorf("checker: unmarshal error [err=%v]", err)
	}

	// call checkers
	ok, err := s.Check(ctx, req.Tx)
	if err != nil {
		log.Error("checker error", fld.Err(err))
		return nil, fmt.Errorf("checker error: %v", err)
	}

	// sign tx and make response
	txbytes, err := proto.Marshal(req.Tx)
	if err != nil {
		log.Error("checker: marshal error", fld.Err(err))
		return nil, fmt.Errorf("marshal error [err=%v]", err)
	}
	res := &CheckResponse{ok, s.privkey.Sign(txbytes)}
	resbytes, err := proto.Marshal(res)
	if err != nil {
		log.Error("checker: marshal error", fld.Err(err))
		return nil, fmt.Errorf("checker: marshal error [err=%v]", err)
	}

	msg := &service.Message{
		ID:      msgID,
		Opcode:  int32(Opcode_CHECK),
		Payload: resbytes,
	}

	return msg, nil
}

func New(cfg *Config) (*Service, error) {
	checkers := map[string]map[string]Checker{}
	for _, c := range cfg.Checkers {
		if m, ok := checkers[c.ContractID()]; ok {
			m[c.Name()] = c
			continue
		}
		checkers[c.ContractID()] = map[string]Checker{c.Name(): c}
	}

	algorithm, err := signature.AlgorithmFromString(cfg.SigningKey.Type)
	if err != nil {
		return nil, err
	}
	privKeybytes, err := b32.DecodeString(cfg.SigningKey.Private)
	if err != nil {
		return nil, err
	}
	privkey, err := signature.LoadPrivateKey(algorithm, privKeybytes)
	if err != nil {
		return nil, err
	}

	return &Service{
		checkers: checkers,
		privkey:  privkey,
	}, nil
}
