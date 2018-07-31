// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: types.proto

package broadcast

import proto "github.com/gogo/protobuf/proto"
import fmt "fmt"
import math "math"
import _ "github.com/gogo/protobuf/gogoproto"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

type OP int32

const (
	OP_UNKNOWN       OP = 0
	OP_ACK_BROADCAST OP = 1
	OP_BROADCAST     OP = 2
	OP_GET_BLOCKS    OP = 3
	OP_GET_ROUNDS    OP = 4
	OP_LIST_BLOCKS   OP = 5
)

var OP_name = map[int32]string{
	0: "UNKNOWN",
	1: "ACK_BROADCAST",
	2: "BROADCAST",
	3: "GET_BLOCKS",
	4: "GET_ROUNDS",
	5: "LIST_BLOCKS",
}
var OP_value = map[string]int32{
	"UNKNOWN":       0,
	"ACK_BROADCAST": 1,
	"BROADCAST":     2,
	"GET_BLOCKS":    3,
	"GET_ROUNDS":    4,
	"LIST_BLOCKS":   5,
}

func (x OP) String() string {
	return proto.EnumName(OP_name, int32(x))
}
func (OP) EnumDescriptor() ([]byte, []int) { return fileDescriptorTypes, []int{0} }

type AckBroadcast struct {
	Last uint64 `protobuf:"varint,1,opt,name=last,proto3" json:"last,omitempty"`
}

func (m *AckBroadcast) Reset()                    { *m = AckBroadcast{} }
func (m *AckBroadcast) String() string            { return proto.CompactTextString(m) }
func (*AckBroadcast) ProtoMessage()               {}
func (*AckBroadcast) Descriptor() ([]byte, []int) { return fileDescriptorTypes, []int{0} }

func (m *AckBroadcast) GetLast() uint64 {
	if m != nil {
		return m.Last
	}
	return 0
}

type Block struct {
	Node         uint64             `protobuf:"varint,1,opt,name=node,proto3" json:"node,omitempty"`
	Previous     *SignedData        `protobuf:"bytes,2,opt,name=previous" json:"previous,omitempty"`
	References   []*SignedData      `protobuf:"bytes,3,rep,name=references" json:"references,omitempty"`
	Round        uint64             `protobuf:"varint,4,opt,name=round,proto3" json:"round,omitempty"`
	Transactions []*TransactionData `protobuf:"bytes,5,rep,name=transactions" json:"transactions,omitempty"`
}

func (m *Block) Reset()                    { *m = Block{} }
func (m *Block) String() string            { return proto.CompactTextString(m) }
func (*Block) ProtoMessage()               {}
func (*Block) Descriptor() ([]byte, []int) { return fileDescriptorTypes, []int{1} }

func (m *Block) GetNode() uint64 {
	if m != nil {
		return m.Node
	}
	return 0
}

func (m *Block) GetPrevious() *SignedData {
	if m != nil {
		return m.Previous
	}
	return nil
}

func (m *Block) GetReferences() []*SignedData {
	if m != nil {
		return m.References
	}
	return nil
}

func (m *Block) GetRound() uint64 {
	if m != nil {
		return m.Round
	}
	return 0
}

func (m *Block) GetTransactions() []*TransactionData {
	if m != nil {
		return m.Transactions
	}
	return nil
}

type BlockReference struct {
	Hash  []byte `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
	Node  uint64 `protobuf:"varint,2,opt,name=node,proto3" json:"node,omitempty"`
	Round uint64 `protobuf:"varint,3,opt,name=round,proto3" json:"round,omitempty"`
}

func (m *BlockReference) Reset()                    { *m = BlockReference{} }
func (m *BlockReference) String() string            { return proto.CompactTextString(m) }
func (*BlockReference) ProtoMessage()               {}
func (*BlockReference) Descriptor() ([]byte, []int) { return fileDescriptorTypes, []int{2} }

func (m *BlockReference) GetHash() []byte {
	if m != nil {
		return m.Hash
	}
	return nil
}

func (m *BlockReference) GetNode() uint64 {
	if m != nil {
		return m.Node
	}
	return 0
}

func (m *BlockReference) GetRound() uint64 {
	if m != nil {
		return m.Round
	}
	return 0
}

type GetBlocks struct {
	Blocks []*BlockReference `protobuf:"bytes,1,rep,name=blocks" json:"blocks,omitempty"`
}

func (m *GetBlocks) Reset()                    { *m = GetBlocks{} }
func (m *GetBlocks) String() string            { return proto.CompactTextString(m) }
func (*GetBlocks) ProtoMessage()               {}
func (*GetBlocks) Descriptor() ([]byte, []int) { return fileDescriptorTypes, []int{3} }

func (m *GetBlocks) GetBlocks() []*BlockReference {
	if m != nil {
		return m.Blocks
	}
	return nil
}

type GetRounds struct {
	Rounds []uint64 `protobuf:"varint,1,rep,packed,name=rounds" json:"rounds,omitempty"`
}

func (m *GetRounds) Reset()                    { *m = GetRounds{} }
func (m *GetRounds) String() string            { return proto.CompactTextString(m) }
func (*GetRounds) ProtoMessage()               {}
func (*GetRounds) Descriptor() ([]byte, []int) { return fileDescriptorTypes, []int{4} }

func (m *GetRounds) GetRounds() []uint64 {
	if m != nil {
		return m.Rounds
	}
	return nil
}

type ListBlocks struct {
	Blocks []*SignedData `protobuf:"bytes,1,rep,name=blocks" json:"blocks,omitempty"`
}

func (m *ListBlocks) Reset()                    { *m = ListBlocks{} }
func (m *ListBlocks) String() string            { return proto.CompactTextString(m) }
func (*ListBlocks) ProtoMessage()               {}
func (*ListBlocks) Descriptor() ([]byte, []int) { return fileDescriptorTypes, []int{5} }

func (m *ListBlocks) GetBlocks() []*SignedData {
	if m != nil {
		return m.Blocks
	}
	return nil
}

// The data of a SignedData may either be an encoded Block or BlockReference.
type SignedData struct {
	Data      []byte `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
	Signature []byte `protobuf:"bytes,2,opt,name=signature,proto3" json:"signature,omitempty"`
}

func (m *SignedData) Reset()                    { *m = SignedData{} }
func (m *SignedData) String() string            { return proto.CompactTextString(m) }
func (*SignedData) ProtoMessage()               {}
func (*SignedData) Descriptor() ([]byte, []int) { return fileDescriptorTypes, []int{6} }

func (m *SignedData) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

func (m *SignedData) GetSignature() []byte {
	if m != nil {
		return m.Signature
	}
	return nil
}

type TransactionData struct {
	Data []byte `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
	Fee  uint64 `protobuf:"varint,2,opt,name=fee,proto3" json:"fee,omitempty"`
}

func (m *TransactionData) Reset()                    { *m = TransactionData{} }
func (m *TransactionData) String() string            { return proto.CompactTextString(m) }
func (*TransactionData) ProtoMessage()               {}
func (*TransactionData) Descriptor() ([]byte, []int) { return fileDescriptorTypes, []int{7} }

func (m *TransactionData) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

func (m *TransactionData) GetFee() uint64 {
	if m != nil {
		return m.Fee
	}
	return 0
}

func init() {
	proto.RegisterType((*AckBroadcast)(nil), "broadcast.AckBroadcast")
	proto.RegisterType((*Block)(nil), "broadcast.Block")
	proto.RegisterType((*BlockReference)(nil), "broadcast.BlockReference")
	proto.RegisterType((*GetBlocks)(nil), "broadcast.GetBlocks")
	proto.RegisterType((*GetRounds)(nil), "broadcast.GetRounds")
	proto.RegisterType((*ListBlocks)(nil), "broadcast.ListBlocks")
	proto.RegisterType((*SignedData)(nil), "broadcast.SignedData")
	proto.RegisterType((*TransactionData)(nil), "broadcast.TransactionData")
	proto.RegisterEnum("broadcast.OP", OP_name, OP_value)
}

func init() { proto.RegisterFile("broadcast/types.proto", fileDescriptorTypes) }

var fileDescriptorTypes = []byte{
	// 438 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x74, 0x52, 0x5d, 0x6b, 0xdb, 0x30,
	0x14, 0x9d, 0x63, 0x27, 0x5b, 0x6e, 0xd2, 0xd6, 0x13, 0xeb, 0xf0, 0xca, 0x1e, 0x82, 0xf7, 0x52,
	0x06, 0x4d, 0xe8, 0xc6, 0xd8, 0xc3, 0x20, 0x90, 0x8f, 0x51, 0x46, 0x82, 0x3d, 0xe4, 0x94, 0x3d,
	0x16, 0xd9, 0x56, 0x1c, 0xd3, 0xce, 0x0a, 0x92, 0x3c, 0xd8, 0x6f, 0xdd, 0x9f, 0x19, 0xbe, 0xa9,
	0x6c, 0xb7, 0xb4, 0x6f, 0xe7, 0x48, 0xe7, 0x9e, 0x73, 0x8f, 0x10, 0x9c, 0xc6, 0x52, 0xb0, 0x34,
	0x61, 0x4a, 0x4f, 0xf4, 0xdf, 0x3d, 0x57, 0xe3, 0xbd, 0x14, 0x5a, 0x90, 0x7e, 0x7d, 0x7c, 0x76,
	0x91, 0xe5, 0x7a, 0x57, 0xc6, 0xe3, 0x44, 0xfc, 0x9e, 0x64, 0x22, 0x13, 0x13, 0x54, 0xc4, 0xe5,
	0x16, 0x19, 0x12, 0x44, 0x87, 0x49, 0xdf, 0x87, 0xe1, 0x2c, 0xb9, 0x9d, 0x9b, 0x71, 0x42, 0xc0,
	0xb9, 0x63, 0x4a, 0x7b, 0xd6, 0xc8, 0x3a, 0x77, 0x28, 0x62, 0xff, 0x9f, 0x05, 0xdd, 0xf9, 0x9d,
	0x48, 0x6e, 0xab, 0xdb, 0x42, 0xa4, 0xdc, 0xdc, 0x56, 0x98, 0x5c, 0xc2, 0xab, 0xbd, 0xe4, 0x7f,
	0x72, 0x51, 0x2a, 0xaf, 0x33, 0xb2, 0xce, 0x07, 0x9f, 0x4e, 0xc7, 0xf5, 0x3a, 0xe3, 0x28, 0xcf,
	0x0a, 0x9e, 0x2e, 0x99, 0x66, 0xb4, 0x96, 0x91, 0x2f, 0x00, 0x92, 0x6f, 0xb9, 0xe4, 0x45, 0xc2,
	0x95, 0x67, 0x8f, 0xec, 0xe7, 0x87, 0x5a, 0x42, 0xf2, 0x06, 0xba, 0x52, 0x94, 0x45, 0xea, 0x39,
	0x18, 0x7f, 0x20, 0x64, 0x0a, 0x43, 0x2d, 0x59, 0xa1, 0x58, 0xa2, 0x73, 0x51, 0x28, 0xaf, 0x8b,
	0x76, 0x67, 0x2d, 0xbb, 0x4d, 0x73, 0x8d, 0x9e, 0x0f, 0xf4, 0x7e, 0x00, 0xc7, 0x58, 0x8e, 0x9a,
	0xa0, 0xaa, 0xe5, 0x8e, 0xa9, 0x1d, 0xb6, 0x1c, 0x52, 0xc4, 0x75, 0xf3, 0x4e, 0xab, 0x79, 0xbd,
	0x8f, 0xdd, 0xda, 0xc7, 0x9f, 0x42, 0xff, 0x8a, 0x6b, 0xb4, 0x54, 0xe4, 0x12, 0x7a, 0x31, 0x22,
	0xcf, 0xc2, 0xb5, 0xde, 0xb5, 0xd6, 0x7a, 0x98, 0x4a, 0xef, 0x85, 0xfe, 0x07, 0x9c, 0xa7, 0x95,
	0x97, 0x22, 0x6f, 0xa1, 0x87, 0xae, 0x87, 0x79, 0x87, 0xde, 0x33, 0xff, 0x1b, 0xc0, 0x3a, 0x57,
	0x26, 0xe5, 0xe2, 0x51, 0xca, 0x33, 0x6f, 0x69, 0x12, 0xa6, 0x00, 0xcd, 0x69, 0xd5, 0x2c, 0x65,
	0x9a, 0x99, 0xb6, 0x15, 0x26, 0xef, 0xa1, 0xaf, 0xf2, 0xac, 0x60, 0xba, 0x94, 0x87, 0xca, 0x43,
	0xda, 0x1c, 0xf8, 0x5f, 0xe1, 0xe4, 0xd1, 0x93, 0x3e, 0x69, 0xe2, 0x82, 0xbd, 0xe5, 0xe6, 0xc5,
	0x2a, 0xf8, 0x31, 0x85, 0x4e, 0xf8, 0x93, 0x0c, 0xe0, 0xe5, 0x75, 0xb0, 0x0a, 0xc2, 0x5f, 0x81,
	0xfb, 0x82, 0xbc, 0x86, 0xa3, 0xd9, 0x62, 0x75, 0x33, 0xa7, 0xe1, 0x6c, 0xb9, 0x98, 0x45, 0x1b,
	0xd7, 0x22, 0x47, 0xd0, 0x6f, 0x68, 0x87, 0x1c, 0x03, 0x5c, 0x7d, 0xdf, 0xdc, 0xcc, 0xd7, 0xe1,
	0x62, 0x15, 0xb9, 0xb6, 0xe1, 0x34, 0xbc, 0x0e, 0x96, 0x91, 0xeb, 0x90, 0x13, 0x18, 0xac, 0x7f,
	0x44, 0xb5, 0xa0, 0x1b, 0xf7, 0xf0, 0x67, 0x7f, 0xfe, 0x1f, 0x00, 0x00, 0xff, 0xff, 0xd9, 0x4d,
	0xb0, 0x5e, 0x2c, 0x03, 0x00, 0x00,
}
