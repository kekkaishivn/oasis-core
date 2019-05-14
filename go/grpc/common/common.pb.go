// Code generated by protoc-gen-go. DO NOT EDIT.
// source: common/common.proto

package common

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type Address_Transport int32

const (
	Address_TCPv4 Address_Transport = 0
	Address_TCPv6 Address_Transport = 1
)

var Address_Transport_name = map[int32]string{
	0: "TCPv4",
	1: "TCPv6",
}

var Address_Transport_value = map[string]int32{
	"TCPv4": 0,
	"TCPv6": 1,
}

func (x Address_Transport) String() string {
	return proto.EnumName(Address_Transport_name, int32(x))
}

func (Address_Transport) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_8f954d82c0b891f6, []int{0, 0}
}

type CapabilitiesTEE_Hardware int32

const (
	CapabilitiesTEE_Invalid  CapabilitiesTEE_Hardware = 0
	CapabilitiesTEE_IntelSGX CapabilitiesTEE_Hardware = 1
)

var CapabilitiesTEE_Hardware_name = map[int32]string{
	0: "Invalid",
	1: "IntelSGX",
}

var CapabilitiesTEE_Hardware_value = map[string]int32{
	"Invalid":  0,
	"IntelSGX": 1,
}

func (x CapabilitiesTEE_Hardware) String() string {
	return proto.EnumName(CapabilitiesTEE_Hardware_name, int32(x))
}

func (CapabilitiesTEE_Hardware) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_8f954d82c0b891f6, []int{6, 0}
}

type Address struct {
	Transport            Address_Transport `protobuf:"varint,1,opt,name=transport,proto3,enum=common.Address_Transport" json:"transport,omitempty"`
	Address              []byte            `protobuf:"bytes,2,opt,name=address,proto3" json:"address,omitempty"`
	Port                 uint32            `protobuf:"varint,3,opt,name=port,proto3" json:"port,omitempty"`
	XXX_NoUnkeyedLiteral struct{}          `json:"-"`
	XXX_unrecognized     []byte            `json:"-"`
	XXX_sizecache        int32             `json:"-"`
}

func (m *Address) Reset()         { *m = Address{} }
func (m *Address) String() string { return proto.CompactTextString(m) }
func (*Address) ProtoMessage()    {}
func (*Address) Descriptor() ([]byte, []int) {
	return fileDescriptor_8f954d82c0b891f6, []int{0}
}

func (m *Address) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Address.Unmarshal(m, b)
}
func (m *Address) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Address.Marshal(b, m, deterministic)
}
func (m *Address) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Address.Merge(m, src)
}
func (m *Address) XXX_Size() int {
	return xxx_messageInfo_Address.Size(m)
}
func (m *Address) XXX_DiscardUnknown() {
	xxx_messageInfo_Address.DiscardUnknown(m)
}

var xxx_messageInfo_Address proto.InternalMessageInfo

func (m *Address) GetTransport() Address_Transport {
	if m != nil {
		return m.Transport
	}
	return Address_TCPv4
}

func (m *Address) GetAddress() []byte {
	if m != nil {
		return m.Address
	}
	return nil
}

func (m *Address) GetPort() uint32 {
	if m != nil {
		return m.Port
	}
	return 0
}

type Certificate struct {
	Der                  []byte   `protobuf:"bytes,1,opt,name=der,proto3" json:"der,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Certificate) Reset()         { *m = Certificate{} }
func (m *Certificate) String() string { return proto.CompactTextString(m) }
func (*Certificate) ProtoMessage()    {}
func (*Certificate) Descriptor() ([]byte, []int) {
	return fileDescriptor_8f954d82c0b891f6, []int{1}
}

func (m *Certificate) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Certificate.Unmarshal(m, b)
}
func (m *Certificate) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Certificate.Marshal(b, m, deterministic)
}
func (m *Certificate) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Certificate.Merge(m, src)
}
func (m *Certificate) XXX_Size() int {
	return xxx_messageInfo_Certificate.Size(m)
}
func (m *Certificate) XXX_DiscardUnknown() {
	xxx_messageInfo_Certificate.DiscardUnknown(m)
}

var xxx_messageInfo_Certificate proto.InternalMessageInfo

func (m *Certificate) GetDer() []byte {
	if m != nil {
		return m.Der
	}
	return nil
}

type Entity struct {
	Id                   []byte   `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	RegistrationTime     uint64   `protobuf:"varint,2,opt,name=registration_time,json=registrationTime,proto3" json:"registration_time,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Entity) Reset()         { *m = Entity{} }
func (m *Entity) String() string { return proto.CompactTextString(m) }
func (*Entity) ProtoMessage()    {}
func (*Entity) Descriptor() ([]byte, []int) {
	return fileDescriptor_8f954d82c0b891f6, []int{2}
}

func (m *Entity) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Entity.Unmarshal(m, b)
}
func (m *Entity) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Entity.Marshal(b, m, deterministic)
}
func (m *Entity) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Entity.Merge(m, src)
}
func (m *Entity) XXX_Size() int {
	return xxx_messageInfo_Entity.Size(m)
}
func (m *Entity) XXX_DiscardUnknown() {
	xxx_messageInfo_Entity.DiscardUnknown(m)
}

var xxx_messageInfo_Entity proto.InternalMessageInfo

func (m *Entity) GetId() []byte {
	if m != nil {
		return m.Id
	}
	return nil
}

func (m *Entity) GetRegistrationTime() uint64 {
	if m != nil {
		return m.RegistrationTime
	}
	return 0
}

type Node struct {
	Id                   []byte         `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	EntityId             []byte         `protobuf:"bytes,2,opt,name=entity_id,json=entityId,proto3" json:"entity_id,omitempty"`
	Expiration           uint64         `protobuf:"varint,3,opt,name=expiration,proto3" json:"expiration,omitempty"`
	Addresses            []*Address     `protobuf:"bytes,4,rep,name=addresses,proto3" json:"addresses,omitempty"`
	Certificate          *Certificate   `protobuf:"bytes,5,opt,name=certificate,proto3" json:"certificate,omitempty"`
	RegistrationTime     uint64         `protobuf:"varint,6,opt,name=registration_time,json=registrationTime,proto3" json:"registration_time,omitempty"`
	Runtimes             []*NodeRuntime `protobuf:"bytes,7,rep,name=runtimes,proto3" json:"runtimes,omitempty"`
	Roles                uint32         `protobuf:"varint,8,opt,name=roles,proto3" json:"roles,omitempty"`
	XXX_NoUnkeyedLiteral struct{}       `json:"-"`
	XXX_unrecognized     []byte         `json:"-"`
	XXX_sizecache        int32          `json:"-"`
}

func (m *Node) Reset()         { *m = Node{} }
func (m *Node) String() string { return proto.CompactTextString(m) }
func (*Node) ProtoMessage()    {}
func (*Node) Descriptor() ([]byte, []int) {
	return fileDescriptor_8f954d82c0b891f6, []int{3}
}

func (m *Node) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Node.Unmarshal(m, b)
}
func (m *Node) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Node.Marshal(b, m, deterministic)
}
func (m *Node) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Node.Merge(m, src)
}
func (m *Node) XXX_Size() int {
	return xxx_messageInfo_Node.Size(m)
}
func (m *Node) XXX_DiscardUnknown() {
	xxx_messageInfo_Node.DiscardUnknown(m)
}

var xxx_messageInfo_Node proto.InternalMessageInfo

func (m *Node) GetId() []byte {
	if m != nil {
		return m.Id
	}
	return nil
}

func (m *Node) GetEntityId() []byte {
	if m != nil {
		return m.EntityId
	}
	return nil
}

func (m *Node) GetExpiration() uint64 {
	if m != nil {
		return m.Expiration
	}
	return 0
}

func (m *Node) GetAddresses() []*Address {
	if m != nil {
		return m.Addresses
	}
	return nil
}

func (m *Node) GetCertificate() *Certificate {
	if m != nil {
		return m.Certificate
	}
	return nil
}

func (m *Node) GetRegistrationTime() uint64 {
	if m != nil {
		return m.RegistrationTime
	}
	return 0
}

func (m *Node) GetRuntimes() []*NodeRuntime {
	if m != nil {
		return m.Runtimes
	}
	return nil
}

func (m *Node) GetRoles() uint32 {
	if m != nil {
		return m.Roles
	}
	return 0
}

type NodeRuntime struct {
	Id                   []byte        `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Capabilities         *Capabilities `protobuf:"bytes,2,opt,name=capabilities,proto3" json:"capabilities,omitempty"`
	XXX_NoUnkeyedLiteral struct{}      `json:"-"`
	XXX_unrecognized     []byte        `json:"-"`
	XXX_sizecache        int32         `json:"-"`
}

func (m *NodeRuntime) Reset()         { *m = NodeRuntime{} }
func (m *NodeRuntime) String() string { return proto.CompactTextString(m) }
func (*NodeRuntime) ProtoMessage()    {}
func (*NodeRuntime) Descriptor() ([]byte, []int) {
	return fileDescriptor_8f954d82c0b891f6, []int{4}
}

func (m *NodeRuntime) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NodeRuntime.Unmarshal(m, b)
}
func (m *NodeRuntime) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NodeRuntime.Marshal(b, m, deterministic)
}
func (m *NodeRuntime) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodeRuntime.Merge(m, src)
}
func (m *NodeRuntime) XXX_Size() int {
	return xxx_messageInfo_NodeRuntime.Size(m)
}
func (m *NodeRuntime) XXX_DiscardUnknown() {
	xxx_messageInfo_NodeRuntime.DiscardUnknown(m)
}

var xxx_messageInfo_NodeRuntime proto.InternalMessageInfo

func (m *NodeRuntime) GetId() []byte {
	if m != nil {
		return m.Id
	}
	return nil
}

func (m *NodeRuntime) GetCapabilities() *Capabilities {
	if m != nil {
		return m.Capabilities
	}
	return nil
}

type Capabilities struct {
	Tee                  *CapabilitiesTEE `protobuf:"bytes,1,opt,name=tee,proto3" json:"tee,omitempty"`
	XXX_NoUnkeyedLiteral struct{}         `json:"-"`
	XXX_unrecognized     []byte           `json:"-"`
	XXX_sizecache        int32            `json:"-"`
}

func (m *Capabilities) Reset()         { *m = Capabilities{} }
func (m *Capabilities) String() string { return proto.CompactTextString(m) }
func (*Capabilities) ProtoMessage()    {}
func (*Capabilities) Descriptor() ([]byte, []int) {
	return fileDescriptor_8f954d82c0b891f6, []int{5}
}

func (m *Capabilities) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Capabilities.Unmarshal(m, b)
}
func (m *Capabilities) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Capabilities.Marshal(b, m, deterministic)
}
func (m *Capabilities) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Capabilities.Merge(m, src)
}
func (m *Capabilities) XXX_Size() int {
	return xxx_messageInfo_Capabilities.Size(m)
}
func (m *Capabilities) XXX_DiscardUnknown() {
	xxx_messageInfo_Capabilities.DiscardUnknown(m)
}

var xxx_messageInfo_Capabilities proto.InternalMessageInfo

func (m *Capabilities) GetTee() *CapabilitiesTEE {
	if m != nil {
		return m.Tee
	}
	return nil
}

type CapabilitiesTEE struct {
	Hardware             CapabilitiesTEE_Hardware `protobuf:"varint,1,opt,name=hardware,proto3,enum=common.CapabilitiesTEE_Hardware" json:"hardware,omitempty"`
	Rak                  []byte                   `protobuf:"bytes,2,opt,name=rak,proto3" json:"rak,omitempty"`
	Attestation          []byte                   `protobuf:"bytes,3,opt,name=attestation,proto3" json:"attestation,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                 `json:"-"`
	XXX_unrecognized     []byte                   `json:"-"`
	XXX_sizecache        int32                    `json:"-"`
}

func (m *CapabilitiesTEE) Reset()         { *m = CapabilitiesTEE{} }
func (m *CapabilitiesTEE) String() string { return proto.CompactTextString(m) }
func (*CapabilitiesTEE) ProtoMessage()    {}
func (*CapabilitiesTEE) Descriptor() ([]byte, []int) {
	return fileDescriptor_8f954d82c0b891f6, []int{6}
}

func (m *CapabilitiesTEE) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CapabilitiesTEE.Unmarshal(m, b)
}
func (m *CapabilitiesTEE) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CapabilitiesTEE.Marshal(b, m, deterministic)
}
func (m *CapabilitiesTEE) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CapabilitiesTEE.Merge(m, src)
}
func (m *CapabilitiesTEE) XXX_Size() int {
	return xxx_messageInfo_CapabilitiesTEE.Size(m)
}
func (m *CapabilitiesTEE) XXX_DiscardUnknown() {
	xxx_messageInfo_CapabilitiesTEE.DiscardUnknown(m)
}

var xxx_messageInfo_CapabilitiesTEE proto.InternalMessageInfo

func (m *CapabilitiesTEE) GetHardware() CapabilitiesTEE_Hardware {
	if m != nil {
		return m.Hardware
	}
	return CapabilitiesTEE_Invalid
}

func (m *CapabilitiesTEE) GetRak() []byte {
	if m != nil {
		return m.Rak
	}
	return nil
}

func (m *CapabilitiesTEE) GetAttestation() []byte {
	if m != nil {
		return m.Attestation
	}
	return nil
}

type Signature struct {
	Pubkey               []byte   `protobuf:"bytes,1,opt,name=pubkey,proto3" json:"pubkey,omitempty"`
	Signature            []byte   `protobuf:"bytes,2,opt,name=signature,proto3" json:"signature,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Signature) Reset()         { *m = Signature{} }
func (m *Signature) String() string { return proto.CompactTextString(m) }
func (*Signature) ProtoMessage()    {}
func (*Signature) Descriptor() ([]byte, []int) {
	return fileDescriptor_8f954d82c0b891f6, []int{7}
}

func (m *Signature) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Signature.Unmarshal(m, b)
}
func (m *Signature) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Signature.Marshal(b, m, deterministic)
}
func (m *Signature) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Signature.Merge(m, src)
}
func (m *Signature) XXX_Size() int {
	return xxx_messageInfo_Signature.Size(m)
}
func (m *Signature) XXX_DiscardUnknown() {
	xxx_messageInfo_Signature.DiscardUnknown(m)
}

var xxx_messageInfo_Signature proto.InternalMessageInfo

func (m *Signature) GetPubkey() []byte {
	if m != nil {
		return m.Pubkey
	}
	return nil
}

func (m *Signature) GetSignature() []byte {
	if m != nil {
		return m.Signature
	}
	return nil
}

type Signed struct {
	Blob                 []byte     `protobuf:"bytes,1,opt,name=blob,proto3" json:"blob,omitempty"`
	Signature            *Signature `protobuf:"bytes,2,opt,name=signature,proto3" json:"signature,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *Signed) Reset()         { *m = Signed{} }
func (m *Signed) String() string { return proto.CompactTextString(m) }
func (*Signed) ProtoMessage()    {}
func (*Signed) Descriptor() ([]byte, []int) {
	return fileDescriptor_8f954d82c0b891f6, []int{8}
}

func (m *Signed) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Signed.Unmarshal(m, b)
}
func (m *Signed) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Signed.Marshal(b, m, deterministic)
}
func (m *Signed) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Signed.Merge(m, src)
}
func (m *Signed) XXX_Size() int {
	return xxx_messageInfo_Signed.Size(m)
}
func (m *Signed) XXX_DiscardUnknown() {
	xxx_messageInfo_Signed.DiscardUnknown(m)
}

var xxx_messageInfo_Signed proto.InternalMessageInfo

func (m *Signed) GetBlob() []byte {
	if m != nil {
		return m.Blob
	}
	return nil
}

func (m *Signed) GetSignature() *Signature {
	if m != nil {
		return m.Signature
	}
	return nil
}

func init() {
	proto.RegisterEnum("common.Address_Transport", Address_Transport_name, Address_Transport_value)
	proto.RegisterEnum("common.CapabilitiesTEE_Hardware", CapabilitiesTEE_Hardware_name, CapabilitiesTEE_Hardware_value)
	proto.RegisterType((*Address)(nil), "common.Address")
	proto.RegisterType((*Certificate)(nil), "common.Certificate")
	proto.RegisterType((*Entity)(nil), "common.Entity")
	proto.RegisterType((*Node)(nil), "common.Node")
	proto.RegisterType((*NodeRuntime)(nil), "common.NodeRuntime")
	proto.RegisterType((*Capabilities)(nil), "common.Capabilities")
	proto.RegisterType((*CapabilitiesTEE)(nil), "common.CapabilitiesTEE")
	proto.RegisterType((*Signature)(nil), "common.Signature")
	proto.RegisterType((*Signed)(nil), "common.Signed")
}

func init() { proto.RegisterFile("common/common.proto", fileDescriptor_8f954d82c0b891f6) }

var fileDescriptor_8f954d82c0b891f6 = []byte{
	// 567 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x74, 0x54, 0xdd, 0x6e, 0xd3, 0x4c,
	0x10, 0xad, 0x93, 0xd4, 0xb1, 0xc7, 0xfe, 0x5a, 0x77, 0x5b, 0x7d, 0x18, 0x81, 0xc0, 0x58, 0x42,
	0x0a, 0x7f, 0xb1, 0x64, 0xfe, 0x25, 0x6e, 0x4a, 0x15, 0x41, 0x2f, 0x40, 0x68, 0x1b, 0x09, 0xc4,
	0x4d, 0xb5, 0xce, 0x2e, 0xe9, 0x2a, 0x8e, 0x6d, 0xed, 0x6e, 0x0a, 0x7d, 0x0e, 0xde, 0x02, 0x89,
	0x77, 0x44, 0xb6, 0xd7, 0x8e, 0x49, 0xc3, 0x55, 0x66, 0xe6, 0x9c, 0x39, 0x9e, 0x33, 0xbb, 0x59,
	0x38, 0x9c, 0xe5, 0xcb, 0x65, 0x9e, 0x45, 0xf5, 0xcf, 0xb8, 0x10, 0xb9, 0xca, 0x91, 0x59, 0x67,
	0xe1, 0x4f, 0x03, 0x86, 0xc7, 0x94, 0x0a, 0x26, 0x25, 0x7a, 0x09, 0xb6, 0x12, 0x24, 0x93, 0x45,
	0x2e, 0x94, 0x6f, 0x04, 0xc6, 0x68, 0x2f, 0xbe, 0x39, 0xd6, 0x5d, 0x9a, 0x33, 0x9e, 0x36, 0x04,
	0xbc, 0xe6, 0x22, 0x1f, 0x86, 0xa4, 0xc6, 0xfd, 0x5e, 0x60, 0x8c, 0x5c, 0xdc, 0xa4, 0x08, 0xc1,
	0xa0, 0x52, 0xeb, 0x07, 0xc6, 0xe8, 0x3f, 0x5c, 0xc5, 0xe1, 0x3d, 0xb0, 0x5b, 0x15, 0x64, 0xc3,
	0xee, 0xf4, 0xe4, 0xd3, 0xe5, 0x33, 0x6f, 0xa7, 0x09, 0x5f, 0x78, 0x46, 0x78, 0x17, 0x9c, 0x13,
	0x26, 0x14, 0xff, 0xc6, 0x67, 0x44, 0x31, 0xe4, 0x41, 0x9f, 0x32, 0x51, 0x8d, 0xe4, 0xe2, 0x32,
	0x0c, 0x27, 0x60, 0x4e, 0x32, 0xc5, 0xd5, 0x15, 0xda, 0x83, 0x1e, 0xa7, 0x1a, 0xea, 0x71, 0x8a,
	0x1e, 0xc1, 0x81, 0x60, 0x73, 0x2e, 0x95, 0x20, 0x8a, 0xe7, 0xd9, 0xb9, 0xe2, 0x4b, 0x56, 0x4d,
	0x35, 0xc0, 0x5e, 0x17, 0x98, 0xf2, 0x25, 0x0b, 0x7f, 0xf7, 0x60, 0xf0, 0x31, 0xa7, 0xec, 0x9a,
	0xca, 0x2d, 0xb0, 0x59, 0xa5, 0x7f, 0xce, 0xa9, 0xf6, 0x64, 0xd5, 0x85, 0x53, 0x8a, 0xee, 0x00,
	0xb0, 0x1f, 0x05, 0xaf, 0x75, 0x2a, 0x6b, 0x03, 0xdc, 0xa9, 0xa0, 0x27, 0x60, 0x6b, 0xff, 0x4c,
	0xfa, 0x83, 0xa0, 0x3f, 0x72, 0xe2, 0xfd, 0x8d, 0x3d, 0xe2, 0x35, 0x03, 0x3d, 0x07, 0x67, 0xb6,
	0x36, 0xeb, 0xef, 0x06, 0xc6, 0xc8, 0x89, 0x0f, 0x9b, 0x86, 0xce, 0x1e, 0x70, 0x97, 0xb7, 0xdd,
	0xa8, 0xb9, 0xdd, 0x28, 0x8a, 0xc0, 0x12, 0xab, 0xac, 0xa4, 0x48, 0x7f, 0x58, 0x4d, 0xd4, 0x7e,
	0xa0, 0xf4, 0x8f, 0x6b, 0x0c, 0xb7, 0x24, 0x74, 0x04, 0xbb, 0x22, 0x4f, 0x99, 0xf4, 0xad, 0xea,
	0xe4, 0xea, 0x24, 0xfc, 0x0c, 0x4e, 0x87, 0x7e, 0x6d, 0x6b, 0xaf, 0xc0, 0x9d, 0x91, 0x82, 0x24,
	0x3c, 0xe5, 0x8a, 0xb3, 0xfa, 0x32, 0x38, 0xf1, 0x51, 0x6b, 0xa5, 0x83, 0xe1, 0xbf, 0x98, 0xe1,
	0x6b, 0x70, 0xbb, 0x28, 0x7a, 0x00, 0x7d, 0xc5, 0x58, 0x25, 0xed, 0xc4, 0x37, 0xb6, 0x09, 0x4c,
	0x27, 0x13, 0x5c, 0x72, 0xc2, 0x5f, 0x06, 0xec, 0x6f, 0x00, 0xe8, 0x0d, 0x58, 0x17, 0x44, 0xd0,
	0xef, 0x44, 0x30, 0x7d, 0x91, 0x83, 0x7f, 0x68, 0x8c, 0xdf, 0x6b, 0x1e, 0x6e, 0x3b, 0xca, 0xeb,
	0x26, 0xc8, 0x42, 0x1f, 0x7b, 0x19, 0xa2, 0x00, 0x1c, 0xa2, 0x14, 0x93, 0x6a, 0x7d, 0xe4, 0x2e,
	0xee, 0x96, 0xc2, 0xfb, 0x60, 0x35, 0x4a, 0xc8, 0x81, 0xe1, 0x69, 0x76, 0x49, 0x52, 0x4e, 0xbd,
	0x1d, 0xe4, 0x82, 0x75, 0x9a, 0x29, 0x96, 0x9e, 0xbd, 0xfb, 0xe2, 0x19, 0xe1, 0x31, 0xd8, 0x67,
	0x7c, 0x9e, 0x11, 0xb5, 0x12, 0x0c, 0xfd, 0x0f, 0x66, 0xb1, 0x4a, 0x16, 0xec, 0x4a, 0xaf, 0x50,
	0x67, 0xe8, 0x36, 0xd8, 0xb2, 0x21, 0xe9, 0x29, 0xd6, 0x85, 0xf0, 0x03, 0x98, 0xa5, 0x04, 0xa3,
	0xe5, 0x9f, 0x2b, 0x49, 0xf3, 0x44, 0x77, 0x57, 0x31, 0x8a, 0x36, 0x7b, 0x9d, 0xf8, 0xa0, 0xb1,
	0xde, 0x7e, 0xb9, 0x23, 0xf7, 0xf6, 0xf1, 0xd7, 0x87, 0x73, 0xae, 0x2e, 0x56, 0x49, 0xc9, 0x8a,
	0x72, 0x22, 0xb9, 0x4c, 0x49, 0x22, 0x23, 0xb6, 0xe0, 0x94, 0x65, 0xd1, 0x3c, 0x8f, 0xe6, 0xa2,
	0x98, 0xe9, 0xc7, 0x23, 0x31, 0xab, 0xd7, 0xe3, 0xe9, 0x9f, 0x00, 0x00, 0x00, 0xff, 0xff, 0xc2,
	0x86, 0xf5, 0xd5, 0x54, 0x04, 0x00, 0x00,
}
