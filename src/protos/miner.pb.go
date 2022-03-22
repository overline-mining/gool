// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.0
// 	protoc        v3.6.1
// source: miner.proto

package protos

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type MinerResponseResult int32

const (
	MinerResponseResult_Canceled MinerResponseResult = 0
	MinerResponseResult_Ok       MinerResponseResult = 1
	MinerResponseResult_Error    MinerResponseResult = 2
)

// Enum value maps for MinerResponseResult.
var (
	MinerResponseResult_name = map[int32]string{
		0: "Canceled",
		1: "Ok",
		2: "Error",
	}
	MinerResponseResult_value = map[string]int32{
		"Canceled": 0,
		"Ok":       1,
		"Error":    2,
	}
)

func (x MinerResponseResult) Enum() *MinerResponseResult {
	p := new(MinerResponseResult)
	*p = x
	return p
}

func (x MinerResponseResult) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (MinerResponseResult) Descriptor() protoreflect.EnumDescriptor {
	return file_miner_proto_enumTypes[0].Descriptor()
}

func (MinerResponseResult) Type() protoreflect.EnumType {
	return &file_miner_proto_enumTypes[0]
}

func (x MinerResponseResult) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use MinerResponseResult.Descriptor instead.
func (MinerResponseResult) EnumDescriptor() ([]byte, []int) {
	return file_miner_proto_rawDescGZIP(), []int{0}
}

type BlockFingerprint struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Blockchain string `protobuf:"bytes,1,opt,name=blockchain,proto3" json:"blockchain,omitempty"`
	Hash       string `protobuf:"bytes,2,opt,name=hash,proto3" json:"hash,omitempty"`
	Timestamp  uint64 `protobuf:"varint,3,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	IsCurrent  bool   `protobuf:"varint,4,opt,name=is_current,json=isCurrent,proto3" json:"is_current,omitempty"`
}

func (x *BlockFingerprint) Reset() {
	*x = BlockFingerprint{}
	if protoimpl.UnsafeEnabled {
		mi := &file_miner_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BlockFingerprint) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BlockFingerprint) ProtoMessage() {}

func (x *BlockFingerprint) ProtoReflect() protoreflect.Message {
	mi := &file_miner_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BlockFingerprint.ProtoReflect.Descriptor instead.
func (*BlockFingerprint) Descriptor() ([]byte, []int) {
	return file_miner_proto_rawDescGZIP(), []int{0}
}

func (x *BlockFingerprint) GetBlockchain() string {
	if x != nil {
		return x.Blockchain
	}
	return ""
}

func (x *BlockFingerprint) GetHash() string {
	if x != nil {
		return x.Hash
	}
	return ""
}

func (x *BlockFingerprint) GetTimestamp() uint64 {
	if x != nil {
		return x.Timestamp
	}
	return 0
}

func (x *BlockFingerprint) GetIsCurrent() bool {
	if x != nil {
		return x.IsCurrent
	}
	return false
}

// Miner block input
type MinerRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	WorkId            string             `protobuf:"bytes,1,opt,name=work_id,json=workId,proto3" json:"work_id,omitempty"`
	CurrentTimestamp  uint64             `protobuf:"varint,2,opt,name=current_timestamp,json=currentTimestamp,proto3" json:"current_timestamp,omitempty"`
	Offset            int32              `protobuf:"varint,3,opt,name=offset,proto3" json:"offset,omitempty"`
	Work              string             `protobuf:"bytes,4,opt,name=work,proto3" json:"work,omitempty"`
	MinerKey          string             `protobuf:"bytes,5,opt,name=miner_key,json=minerKey,proto3" json:"miner_key,omitempty"`
	MerkleRoot        string             `protobuf:"bytes,6,opt,name=merkle_root,json=merkleRoot,proto3" json:"merkle_root,omitempty"`
	Difficulty        string             `protobuf:"bytes,7,opt,name=difficulty,proto3" json:"difficulty,omitempty"`
	LastPreviousBlock *BcBlock           `protobuf:"bytes,8,opt,name=last_previous_block,json=lastPreviousBlock,proto3" json:"last_previous_block,omitempty"`
	NewBlockHeaders   *BlockchainHeaders `protobuf:"bytes,9,opt,name=new_block_headers,json=newBlockHeaders,proto3" json:"new_block_headers,omitempty"`
}

func (x *MinerRequest) Reset() {
	*x = MinerRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_miner_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MinerRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MinerRequest) ProtoMessage() {}

func (x *MinerRequest) ProtoReflect() protoreflect.Message {
	mi := &file_miner_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MinerRequest.ProtoReflect.Descriptor instead.
func (*MinerRequest) Descriptor() ([]byte, []int) {
	return file_miner_proto_rawDescGZIP(), []int{1}
}

func (x *MinerRequest) GetWorkId() string {
	if x != nil {
		return x.WorkId
	}
	return ""
}

func (x *MinerRequest) GetCurrentTimestamp() uint64 {
	if x != nil {
		return x.CurrentTimestamp
	}
	return 0
}

func (x *MinerRequest) GetOffset() int32 {
	if x != nil {
		return x.Offset
	}
	return 0
}

func (x *MinerRequest) GetWork() string {
	if x != nil {
		return x.Work
	}
	return ""
}

func (x *MinerRequest) GetMinerKey() string {
	if x != nil {
		return x.MinerKey
	}
	return ""
}

func (x *MinerRequest) GetMerkleRoot() string {
	if x != nil {
		return x.MerkleRoot
	}
	return ""
}

func (x *MinerRequest) GetDifficulty() string {
	if x != nil {
		return x.Difficulty
	}
	return ""
}

func (x *MinerRequest) GetLastPreviousBlock() *BcBlock {
	if x != nil {
		return x.LastPreviousBlock
	}
	return nil
}

func (x *MinerRequest) GetNewBlockHeaders() *BlockchainHeaders {
	if x != nil {
		return x.NewBlockHeaders
	}
	return nil
}

// Miner block output
type MinerResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Result     MinerResponseResult `protobuf:"varint,1,opt,name=result,proto3,enum=bc.miner.MinerResponseResult" json:"result,omitempty"`
	Nonce      string              `protobuf:"bytes,2,opt,name=nonce,proto3" json:"nonce,omitempty"`
	Difficulty string              `protobuf:"bytes,3,opt,name=difficulty,proto3" json:"difficulty,omitempty"`
	Distance   string              `protobuf:"bytes,4,opt,name=distance,proto3" json:"distance,omitempty"`
	Timestamp  uint64              `protobuf:"varint,5,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	Iterations uint64              `protobuf:"varint,6,opt,name=iterations,proto3" json:"iterations,omitempty"`
	TimeDiff   uint64              `protobuf:"varint,7,opt,name=time_diff,json=timeDiff,proto3" json:"time_diff,omitempty"`
}

func (x *MinerResponse) Reset() {
	*x = MinerResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_miner_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MinerResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MinerResponse) ProtoMessage() {}

func (x *MinerResponse) ProtoReflect() protoreflect.Message {
	mi := &file_miner_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MinerResponse.ProtoReflect.Descriptor instead.
func (*MinerResponse) Descriptor() ([]byte, []int) {
	return file_miner_proto_rawDescGZIP(), []int{2}
}

func (x *MinerResponse) GetResult() MinerResponseResult {
	if x != nil {
		return x.Result
	}
	return MinerResponseResult_Canceled
}

func (x *MinerResponse) GetNonce() string {
	if x != nil {
		return x.Nonce
	}
	return ""
}

func (x *MinerResponse) GetDifficulty() string {
	if x != nil {
		return x.Difficulty
	}
	return ""
}

func (x *MinerResponse) GetDistance() string {
	if x != nil {
		return x.Distance
	}
	return ""
}

func (x *MinerResponse) GetTimestamp() uint64 {
	if x != nil {
		return x.Timestamp
	}
	return 0
}

func (x *MinerResponse) GetIterations() uint64 {
	if x != nil {
		return x.Iterations
	}
	return 0
}

func (x *MinerResponse) GetTimeDiff() uint64 {
	if x != nil {
		return x.TimeDiff
	}
	return 0
}

var File_miner_proto protoreflect.FileDescriptor

var file_miner_proto_rawDesc = []byte{
	0x0a, 0x0b, 0x6d, 0x69, 0x6e, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x08, 0x62,
	0x63, 0x2e, 0x6d, 0x69, 0x6e, 0x65, 0x72, 0x1a, 0x0a, 0x63, 0x6f, 0x72, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x22, 0x83, 0x01, 0x0a, 0x10, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x46, 0x69, 0x6e,
	0x67, 0x65, 0x72, 0x70, 0x72, 0x69, 0x6e, 0x74, 0x12, 0x1e, 0x0a, 0x0a, 0x62, 0x6c, 0x6f, 0x63,
	0x6b, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x62, 0x6c,
	0x6f, 0x63, 0x6b, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x68, 0x61, 0x73, 0x68,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x68, 0x61, 0x73, 0x68, 0x12, 0x1c, 0x0a, 0x09,
	0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04, 0x52,
	0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x1d, 0x0a, 0x0a, 0x69, 0x73,
	0x5f, 0x63, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09,
	0x69, 0x73, 0x43, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x74, 0x22, 0xe8, 0x02, 0x0a, 0x0c, 0x4d, 0x69,
	0x6e, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x17, 0x0a, 0x07, 0x77, 0x6f,
	0x72, 0x6b, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x77, 0x6f, 0x72,
	0x6b, 0x49, 0x64, 0x12, 0x2b, 0x0a, 0x11, 0x63, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x74, 0x5f, 0x74,
	0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x10,
	0x63, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70,
	0x12, 0x16, 0x0a, 0x06, 0x6f, 0x66, 0x66, 0x73, 0x65, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x05,
	0x52, 0x06, 0x6f, 0x66, 0x66, 0x73, 0x65, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x77, 0x6f, 0x72, 0x6b,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x77, 0x6f, 0x72, 0x6b, 0x12, 0x1b, 0x0a, 0x09,
	0x6d, 0x69, 0x6e, 0x65, 0x72, 0x5f, 0x6b, 0x65, 0x79, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x08, 0x6d, 0x69, 0x6e, 0x65, 0x72, 0x4b, 0x65, 0x79, 0x12, 0x1f, 0x0a, 0x0b, 0x6d, 0x65, 0x72,
	0x6b, 0x6c, 0x65, 0x5f, 0x72, 0x6f, 0x6f, 0x74, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a,
	0x6d, 0x65, 0x72, 0x6b, 0x6c, 0x65, 0x52, 0x6f, 0x6f, 0x74, 0x12, 0x1e, 0x0a, 0x0a, 0x64, 0x69,
	0x66, 0x66, 0x69, 0x63, 0x75, 0x6c, 0x74, 0x79, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a,
	0x64, 0x69, 0x66, 0x66, 0x69, 0x63, 0x75, 0x6c, 0x74, 0x79, 0x12, 0x40, 0x0a, 0x13, 0x6c, 0x61,
	0x73, 0x74, 0x5f, 0x70, 0x72, 0x65, 0x76, 0x69, 0x6f, 0x75, 0x73, 0x5f, 0x62, 0x6c, 0x6f, 0x63,
	0x6b, 0x18, 0x08, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x10, 0x2e, 0x62, 0x63, 0x2e, 0x63, 0x6f, 0x72,
	0x65, 0x2e, 0x42, 0x63, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x52, 0x11, 0x6c, 0x61, 0x73, 0x74, 0x50,
	0x72, 0x65, 0x76, 0x69, 0x6f, 0x75, 0x73, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x12, 0x46, 0x0a, 0x11,
	0x6e, 0x65, 0x77, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72,
	0x73, 0x18, 0x09, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x62, 0x63, 0x2e, 0x63, 0x6f, 0x72,
	0x65, 0x2e, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x48, 0x65, 0x61, 0x64,
	0x65, 0x72, 0x73, 0x52, 0x0f, 0x6e, 0x65, 0x77, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x65, 0x61,
	0x64, 0x65, 0x72, 0x73, 0x22, 0xf3, 0x01, 0x0a, 0x0d, 0x4d, 0x69, 0x6e, 0x65, 0x72, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x35, 0x0a, 0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x1d, 0x2e, 0x62, 0x63, 0x2e, 0x6d, 0x69, 0x6e, 0x65,
	0x72, 0x2e, 0x4d, 0x69, 0x6e, 0x65, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x52,
	0x65, 0x73, 0x75, 0x6c, 0x74, 0x52, 0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x12, 0x14, 0x0a,
	0x05, 0x6e, 0x6f, 0x6e, 0x63, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x6e, 0x6f,
	0x6e, 0x63, 0x65, 0x12, 0x1e, 0x0a, 0x0a, 0x64, 0x69, 0x66, 0x66, 0x69, 0x63, 0x75, 0x6c, 0x74,
	0x79, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x64, 0x69, 0x66, 0x66, 0x69, 0x63, 0x75,
	0x6c, 0x74, 0x79, 0x12, 0x1a, 0x0a, 0x08, 0x64, 0x69, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65, 0x18,
	0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x64, 0x69, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65, 0x12,
	0x1c, 0x0a, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x05, 0x20, 0x01,
	0x28, 0x04, 0x52, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x1e, 0x0a,
	0x0a, 0x69, 0x74, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x06, 0x20, 0x01, 0x28,
	0x04, 0x52, 0x0a, 0x69, 0x74, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x1b, 0x0a,
	0x09, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x64, 0x69, 0x66, 0x66, 0x18, 0x07, 0x20, 0x01, 0x28, 0x04,
	0x52, 0x08, 0x74, 0x69, 0x6d, 0x65, 0x44, 0x69, 0x66, 0x66, 0x2a, 0x36, 0x0a, 0x13, 0x4d, 0x69,
	0x6e, 0x65, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x52, 0x65, 0x73, 0x75, 0x6c,
	0x74, 0x12, 0x0c, 0x0a, 0x08, 0x43, 0x61, 0x6e, 0x63, 0x65, 0x6c, 0x65, 0x64, 0x10, 0x00, 0x12,
	0x06, 0x0a, 0x02, 0x4f, 0x6b, 0x10, 0x01, 0x12, 0x09, 0x0a, 0x05, 0x45, 0x72, 0x72, 0x6f, 0x72,
	0x10, 0x02, 0x32, 0x42, 0x0a, 0x05, 0x4d, 0x69, 0x6e, 0x65, 0x72, 0x12, 0x39, 0x0a, 0x04, 0x4d,
	0x69, 0x6e, 0x65, 0x12, 0x16, 0x2e, 0x62, 0x63, 0x2e, 0x6d, 0x69, 0x6e, 0x65, 0x72, 0x2e, 0x4d,
	0x69, 0x6e, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x17, 0x2e, 0x62, 0x63,
	0x2e, 0x6d, 0x69, 0x6e, 0x65, 0x72, 0x2e, 0x4d, 0x69, 0x6e, 0x65, 0x72, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x1c, 0x5a, 0x1a, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62,
	0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x67, 0x6f, 0x6f, 0x6c, 0x2f, 0x73, 0x72, 0x63, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_miner_proto_rawDescOnce sync.Once
	file_miner_proto_rawDescData = file_miner_proto_rawDesc
)

func file_miner_proto_rawDescGZIP() []byte {
	file_miner_proto_rawDescOnce.Do(func() {
		file_miner_proto_rawDescData = protoimpl.X.CompressGZIP(file_miner_proto_rawDescData)
	})
	return file_miner_proto_rawDescData
}

var file_miner_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_miner_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_miner_proto_goTypes = []interface{}{
	(MinerResponseResult)(0),  // 0: bc.miner.MinerResponseResult
	(*BlockFingerprint)(nil),  // 1: bc.miner.BlockFingerprint
	(*MinerRequest)(nil),      // 2: bc.miner.MinerRequest
	(*MinerResponse)(nil),     // 3: bc.miner.MinerResponse
	(*BcBlock)(nil),           // 4: bc.core.BcBlock
	(*BlockchainHeaders)(nil), // 5: bc.core.BlockchainHeaders
}
var file_miner_proto_depIdxs = []int32{
	4, // 0: bc.miner.MinerRequest.last_previous_block:type_name -> bc.core.BcBlock
	5, // 1: bc.miner.MinerRequest.new_block_headers:type_name -> bc.core.BlockchainHeaders
	0, // 2: bc.miner.MinerResponse.result:type_name -> bc.miner.MinerResponseResult
	2, // 3: bc.miner.Miner.Mine:input_type -> bc.miner.MinerRequest
	3, // 4: bc.miner.Miner.Mine:output_type -> bc.miner.MinerResponse
	4, // [4:5] is the sub-list for method output_type
	3, // [3:4] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_miner_proto_init() }
func file_miner_proto_init() {
	if File_miner_proto != nil {
		return
	}
	file_core_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_miner_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BlockFingerprint); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_miner_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*MinerRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_miner_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*MinerResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_miner_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_miner_proto_goTypes,
		DependencyIndexes: file_miner_proto_depIdxs,
		EnumInfos:         file_miner_proto_enumTypes,
		MessageInfos:      file_miner_proto_msgTypes,
	}.Build()
	File_miner_proto = out.File
	file_miner_proto_rawDesc = nil
	file_miner_proto_goTypes = nil
	file_miner_proto_depIdxs = nil
}
