// Package fld provides field constructors with preset key names.
package fld

import (
	"fmt"
	"time"

	"chainspace.io/prototype/internal/log"
)

// Address log field.
func Address(value string) log.Field {
	return log.String("address", value)
}

// BlockHash log field.
func BlockHash(value []byte) log.Field {
	return log.Digest("block.hash", value)
}

// BlockID log field.
func BlockID(value fmt.Stringer) log.Field {
	return log.String("block.id", value.String())
}

// ConnectionType log field.
func ConnectionType(value int32) log.Field {
	return log.Int32("type", value)
}

// Err log field.
func Err(value error) log.Field {
	return log.Err(value)
}

// FromBlock log field.
func FromBlock(value fmt.Stringer) log.Field {
	return log.String("block.from", value.String())
}

// FromState log from.
func FromState(value fmt.Stringer) log.Field {
	return log.String("status.from", value.String())
}

// InterpretedRound log field.
func InterpretedRound(value uint64) log.Field {
	return log.Uint64("interpreted.round", value)
}

// LatestRound log field.
func LatestRound(value uint64) log.Field {
	return log.Uint64("latest.round", value)
}

// NetworkName log field.
func NetworkName(value string) log.Field {
	return log.String("network.name", value)
}

// NodeID log field.
func NodeID(value uint64) log.Field {
	return log.Uint64("node.id", value)
}

// OK log field.
func OK(value bool) log.Field {
	return log.Bool("ok", value)
}

// ObjectID log field.
func ObjectID(value []byte) log.Field {
	return log.Digest("object.id", value)
}

// Path log field.
func Path(value string) log.Field {
	return log.String("path", value)
}

// PayloadLimit log field.
func PayloadLimit(value int) log.Field {
	return log.Int("payload.limit", value)
}

// PeerID log field.
func PeerID(value uint64) log.Field {
	return log.Uint64("peer.id", value)
}

// PeerShard log field.
func PeerShard(value uint64) log.Field {
	return log.Uint64("peer.shard", value)
}

// Perspective log field.
func Perspective(value uint64) log.Field {
	return log.Uint64("perspective", value)
}

// Port log field.
func Port(value int) log.Field {
	return log.Int("port", value)
}

// Round log field.
func Round(value uint64) log.Field {
	return log.Uint64("round", value)
}

// Rounds log field.
func Rounds(value []uint64) log.Field {
	return log.Uint64s("rounds", value)
}

// SelfNodeID log field.
func SelfNodeID(value uint64) log.Field {
	return log.Uint64("@self/node.id", value)
}

// SelfShardID log field.
func SelfShardID(value uint64) log.Field {
	return log.Uint64("@self/shard.id", value)
}

// Service log field.
func Service(value string) log.Field {
	return log.String("service", value)
}

// ShardCount log field.
func ShardCount(value uint64) log.Field {
	return log.Uint64("shard.count", value)
}

// ShardID log field.
func ShardID(value uint64) log.Field {
	return log.Uint64("shard.id", value)
}

// ShardSize log field.
func ShardSize(value uint64) log.Field {
	return log.Uint64("shard.size", value)
}

// Size log field.
func Size(value int) log.Field {
	return log.Int("size", value)
}

// Status log field.
func Status(value fmt.Stringer) log.Field {
	return log.String("status", value.String())
}

// TimeTaken log field.
func TimeTaken(value time.Duration) log.Field {
	return log.Duration("time.taken", value)
}

// ToBlock log field.
func ToBlock(value fmt.Stringer) log.Field {
	return log.String("block.to", value.String())
}

// ToState log field.
func ToState(value fmt.Stringer) log.Field {
	return log.String("status.to", value.String())
}

// SBACCmd log field.
func SBACCmd(value string) log.Field {
	return log.String("cmd", value)
}

// TxID log field.
func TxID(value uint32) log.Field {
	return log.Uint32("tx.id", value)
}

// URL log field.
func URL(value fmt.Stringer) log.Field {
	return log.String("url", value.String())
}
