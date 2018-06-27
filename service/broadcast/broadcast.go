// Package broadcast implements the network broadcast within a shard.
package broadcast

import (
	"context"
	"fmt"

	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
	"chainspace.io/prototype/state"
)

type Config struct {
	NodeID uint64
}

type Service struct {
	cfg   *Config
	state *state.Machine
	top   *network.Topology
}

func (s *Service) Handle(ctx context.Context, msg *service.Message) (*service.Message, error) {
	switch Opcode(msg.Opcode) {
	case Opcode_BLOCK:
		return nil, nil
	case Opcode_BLOCKS_REQUEST:
		return nil, nil
	default:
		return nil, fmt.Errorf("broadcast: unknown message opcode: %d", msg.Opcode)
	}
}

func (s *Service) Name() string {
	return "broadcaster"
}

func New(cfg *Config, top *network.Topology, state *state.Machine) *Service {
	return &Service{
		cfg:   cfg,
		state: state,
		top:   top,
	}
}
