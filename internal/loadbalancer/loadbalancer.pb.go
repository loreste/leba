// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v3.21.12
// source: loadbalancer.proto

package loadbalancer

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

// Request message for adding a backend.
type BackendRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Address            string `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
	Protocol           string `protobuf:"bytes,2,opt,name=protocol,proto3" json:"protocol,omitempty"`
	Port               int32  `protobuf:"varint,3,opt,name=port,proto3" json:"port,omitempty"`
	Weight             int32  `protobuf:"varint,4,opt,name=weight,proto3" json:"weight,omitempty"`
	MaxOpenConnections int32  `protobuf:"varint,5,opt,name=max_open_connections,json=maxOpenConnections,proto3" json:"max_open_connections,omitempty"`
	MaxIdleConnections int32  `protobuf:"varint,6,opt,name=max_idle_connections,json=maxIdleConnections,proto3" json:"max_idle_connections,omitempty"`
	ConnMaxLifetime    int32  `protobuf:"varint,7,opt,name=conn_max_lifetime,json=connMaxLifetime,proto3" json:"conn_max_lifetime,omitempty"`
}

func (x *BackendRequest) Reset() {
	*x = BackendRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_loadbalancer_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BackendRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BackendRequest) ProtoMessage() {}

func (x *BackendRequest) ProtoReflect() protoreflect.Message {
	mi := &file_loadbalancer_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BackendRequest.ProtoReflect.Descriptor instead.
func (*BackendRequest) Descriptor() ([]byte, []int) {
	return file_loadbalancer_proto_rawDescGZIP(), []int{0}
}

func (x *BackendRequest) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

func (x *BackendRequest) GetProtocol() string {
	if x != nil {
		return x.Protocol
	}
	return ""
}

func (x *BackendRequest) GetPort() int32 {
	if x != nil {
		return x.Port
	}
	return 0
}

func (x *BackendRequest) GetWeight() int32 {
	if x != nil {
		return x.Weight
	}
	return 0
}

func (x *BackendRequest) GetMaxOpenConnections() int32 {
	if x != nil {
		return x.MaxOpenConnections
	}
	return 0
}

func (x *BackendRequest) GetMaxIdleConnections() int32 {
	if x != nil {
		return x.MaxIdleConnections
	}
	return 0
}

func (x *BackendRequest) GetConnMaxLifetime() int32 {
	if x != nil {
		return x.ConnMaxLifetime
	}
	return 0
}

// Response message for backend operations.
type BackendResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Message string `protobuf:"bytes,1,opt,name=message,proto3" json:"message,omitempty"`
}

func (x *BackendResponse) Reset() {
	*x = BackendResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_loadbalancer_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BackendResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BackendResponse) ProtoMessage() {}

func (x *BackendResponse) ProtoReflect() protoreflect.Message {
	mi := &file_loadbalancer_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BackendResponse.ProtoReflect.Descriptor instead.
func (*BackendResponse) Descriptor() ([]byte, []int) {
	return file_loadbalancer_proto_rawDescGZIP(), []int{1}
}

func (x *BackendResponse) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

// Request message for removing a backend.
type RemoveBackendRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Address string `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
}

func (x *RemoveBackendRequest) Reset() {
	*x = RemoveBackendRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_loadbalancer_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RemoveBackendRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RemoveBackendRequest) ProtoMessage() {}

func (x *RemoveBackendRequest) ProtoReflect() protoreflect.Message {
	mi := &file_loadbalancer_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RemoveBackendRequest.ProtoReflect.Descriptor instead.
func (*RemoveBackendRequest) Descriptor() ([]byte, []int) {
	return file_loadbalancer_proto_rawDescGZIP(), []int{2}
}

func (x *RemoveBackendRequest) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

// Request message for listing all backends.
type ListBackendsRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ListBackendsRequest) Reset() {
	*x = ListBackendsRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_loadbalancer_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListBackendsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListBackendsRequest) ProtoMessage() {}

func (x *ListBackendsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_loadbalancer_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListBackendsRequest.ProtoReflect.Descriptor instead.
func (*ListBackendsRequest) Descriptor() ([]byte, []int) {
	return file_loadbalancer_proto_rawDescGZIP(), []int{3}
}

// Response message containing a list of backends.
type ListBackendsResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Backends []*BackendInfo `protobuf:"bytes,1,rep,name=backends,proto3" json:"backends,omitempty"`
}

func (x *ListBackendsResponse) Reset() {
	*x = ListBackendsResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_loadbalancer_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListBackendsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListBackendsResponse) ProtoMessage() {}

func (x *ListBackendsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_loadbalancer_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListBackendsResponse.ProtoReflect.Descriptor instead.
func (*ListBackendsResponse) Descriptor() ([]byte, []int) {
	return file_loadbalancer_proto_rawDescGZIP(), []int{4}
}

func (x *ListBackendsResponse) GetBackends() []*BackendInfo {
	if x != nil {
		return x.Backends
	}
	return nil
}

// Information about a single backend.
type BackendInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Address            string `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
	Protocol           string `protobuf:"bytes,2,opt,name=protocol,proto3" json:"protocol,omitempty"`
	Port               int32  `protobuf:"varint,3,opt,name=port,proto3" json:"port,omitempty"`
	Weight             int32  `protobuf:"varint,4,opt,name=weight,proto3" json:"weight,omitempty"`
	MaxOpenConnections int32  `protobuf:"varint,5,opt,name=max_open_connections,json=maxOpenConnections,proto3" json:"max_open_connections,omitempty"`
	MaxIdleConnections int32  `protobuf:"varint,6,opt,name=max_idle_connections,json=maxIdleConnections,proto3" json:"max_idle_connections,omitempty"`
	ConnMaxLifetime    int32  `protobuf:"varint,7,opt,name=conn_max_lifetime,json=connMaxLifetime,proto3" json:"conn_max_lifetime,omitempty"`
	Health             bool   `protobuf:"varint,8,opt,name=health,proto3" json:"health,omitempty"`
}

func (x *BackendInfo) Reset() {
	*x = BackendInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_loadbalancer_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BackendInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BackendInfo) ProtoMessage() {}

func (x *BackendInfo) ProtoReflect() protoreflect.Message {
	mi := &file_loadbalancer_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BackendInfo.ProtoReflect.Descriptor instead.
func (*BackendInfo) Descriptor() ([]byte, []int) {
	return file_loadbalancer_proto_rawDescGZIP(), []int{5}
}

func (x *BackendInfo) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

func (x *BackendInfo) GetProtocol() string {
	if x != nil {
		return x.Protocol
	}
	return ""
}

func (x *BackendInfo) GetPort() int32 {
	if x != nil {
		return x.Port
	}
	return 0
}

func (x *BackendInfo) GetWeight() int32 {
	if x != nil {
		return x.Weight
	}
	return 0
}

func (x *BackendInfo) GetMaxOpenConnections() int32 {
	if x != nil {
		return x.MaxOpenConnections
	}
	return 0
}

func (x *BackendInfo) GetMaxIdleConnections() int32 {
	if x != nil {
		return x.MaxIdleConnections
	}
	return 0
}

func (x *BackendInfo) GetConnMaxLifetime() int32 {
	if x != nil {
		return x.ConnMaxLifetime
	}
	return 0
}

func (x *BackendInfo) GetHealth() bool {
	if x != nil {
		return x.Health
	}
	return false
}

var File_loadbalancer_proto protoreflect.FileDescriptor

var file_loadbalancer_proto_rawDesc = []byte{
	0x0a, 0x12, 0x6c, 0x6f, 0x61, 0x64, 0x62, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x72, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0c, 0x6c, 0x6f, 0x61, 0x64, 0x62, 0x61, 0x6c, 0x61, 0x6e, 0x63,
	0x65, 0x72, 0x22, 0x82, 0x02, 0x0a, 0x0e, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12,
	0x1a, 0x0a, 0x08, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x08, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x12, 0x12, 0x0a, 0x04, 0x70,
	0x6f, 0x72, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x05, 0x52, 0x04, 0x70, 0x6f, 0x72, 0x74, 0x12,
	0x16, 0x0a, 0x06, 0x77, 0x65, 0x69, 0x67, 0x68, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x05, 0x52,
	0x06, 0x77, 0x65, 0x69, 0x67, 0x68, 0x74, 0x12, 0x30, 0x0a, 0x14, 0x6d, 0x61, 0x78, 0x5f, 0x6f,
	0x70, 0x65, 0x6e, 0x5f, 0x63, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18,
	0x05, 0x20, 0x01, 0x28, 0x05, 0x52, 0x12, 0x6d, 0x61, 0x78, 0x4f, 0x70, 0x65, 0x6e, 0x43, 0x6f,
	0x6e, 0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x30, 0x0a, 0x14, 0x6d, 0x61, 0x78,
	0x5f, 0x69, 0x64, 0x6c, 0x65, 0x5f, 0x63, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x18, 0x06, 0x20, 0x01, 0x28, 0x05, 0x52, 0x12, 0x6d, 0x61, 0x78, 0x49, 0x64, 0x6c, 0x65,
	0x43, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x2a, 0x0a, 0x11, 0x63,
	0x6f, 0x6e, 0x6e, 0x5f, 0x6d, 0x61, 0x78, 0x5f, 0x6c, 0x69, 0x66, 0x65, 0x74, 0x69, 0x6d, 0x65,
	0x18, 0x07, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0f, 0x63, 0x6f, 0x6e, 0x6e, 0x4d, 0x61, 0x78, 0x4c,
	0x69, 0x66, 0x65, 0x74, 0x69, 0x6d, 0x65, 0x22, 0x2b, 0x0a, 0x0f, 0x42, 0x61, 0x63, 0x6b, 0x65,
	0x6e, 0x64, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73,
	0x73, 0x61, 0x67, 0x65, 0x22, 0x30, 0x0a, 0x14, 0x52, 0x65, 0x6d, 0x6f, 0x76, 0x65, 0x42, 0x61,
	0x63, 0x6b, 0x65, 0x6e, 0x64, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x18, 0x0a, 0x07,
	0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61,
	0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x22, 0x15, 0x0a, 0x13, 0x4c, 0x69, 0x73, 0x74, 0x42, 0x61,
	0x63, 0x6b, 0x65, 0x6e, 0x64, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x4d, 0x0a,
	0x14, 0x4c, 0x69, 0x73, 0x74, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x73, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x35, 0x0a, 0x08, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64,
	0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x6c, 0x6f, 0x61, 0x64, 0x62, 0x61,
	0x6c, 0x61, 0x6e, 0x63, 0x65, 0x72, 0x2e, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x49, 0x6e,
	0x66, 0x6f, 0x52, 0x08, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x73, 0x22, 0x97, 0x02, 0x0a,
	0x0b, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x18, 0x0a, 0x07,
	0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61,
	0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63,
	0x6f, 0x6c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63,
	0x6f, 0x6c, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x05,
	0x52, 0x04, 0x70, 0x6f, 0x72, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x77, 0x65, 0x69, 0x67, 0x68, 0x74,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x05, 0x52, 0x06, 0x77, 0x65, 0x69, 0x67, 0x68, 0x74, 0x12, 0x30,
	0x0a, 0x14, 0x6d, 0x61, 0x78, 0x5f, 0x6f, 0x70, 0x65, 0x6e, 0x5f, 0x63, 0x6f, 0x6e, 0x6e, 0x65,
	0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x05, 0x52, 0x12, 0x6d, 0x61,
	0x78, 0x4f, 0x70, 0x65, 0x6e, 0x43, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x12, 0x30, 0x0a, 0x14, 0x6d, 0x61, 0x78, 0x5f, 0x69, 0x64, 0x6c, 0x65, 0x5f, 0x63, 0x6f, 0x6e,
	0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x06, 0x20, 0x01, 0x28, 0x05, 0x52, 0x12,
	0x6d, 0x61, 0x78, 0x49, 0x64, 0x6c, 0x65, 0x43, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x12, 0x2a, 0x0a, 0x11, 0x63, 0x6f, 0x6e, 0x6e, 0x5f, 0x6d, 0x61, 0x78, 0x5f, 0x6c,
	0x69, 0x66, 0x65, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0f, 0x63,
	0x6f, 0x6e, 0x6e, 0x4d, 0x61, 0x78, 0x4c, 0x69, 0x66, 0x65, 0x74, 0x69, 0x6d, 0x65, 0x12, 0x16,
	0x0a, 0x06, 0x68, 0x65, 0x61, 0x6c, 0x74, 0x68, 0x18, 0x08, 0x20, 0x01, 0x28, 0x08, 0x52, 0x06,
	0x68, 0x65, 0x61, 0x6c, 0x74, 0x68, 0x32, 0x84, 0x02, 0x0a, 0x0c, 0x4c, 0x6f, 0x61, 0x64, 0x42,
	0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x72, 0x12, 0x49, 0x0a, 0x0a, 0x41, 0x64, 0x64, 0x42, 0x61,
	0x63, 0x6b, 0x65, 0x6e, 0x64, 0x12, 0x1c, 0x2e, 0x6c, 0x6f, 0x61, 0x64, 0x62, 0x61, 0x6c, 0x61,
	0x6e, 0x63, 0x65, 0x72, 0x2e, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x1a, 0x1d, 0x2e, 0x6c, 0x6f, 0x61, 0x64, 0x62, 0x61, 0x6c, 0x61, 0x6e, 0x63,
	0x65, 0x72, 0x2e, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x12, 0x52, 0x0a, 0x0d, 0x52, 0x65, 0x6d, 0x6f, 0x76, 0x65, 0x42, 0x61, 0x63, 0x6b,
	0x65, 0x6e, 0x64, 0x12, 0x22, 0x2e, 0x6c, 0x6f, 0x61, 0x64, 0x62, 0x61, 0x6c, 0x61, 0x6e, 0x63,
	0x65, 0x72, 0x2e, 0x52, 0x65, 0x6d, 0x6f, 0x76, 0x65, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1d, 0x2e, 0x6c, 0x6f, 0x61, 0x64, 0x62, 0x61,
	0x6c, 0x61, 0x6e, 0x63, 0x65, 0x72, 0x2e, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x55, 0x0a, 0x0c, 0x4c, 0x69, 0x73, 0x74, 0x42, 0x61,
	0x63, 0x6b, 0x65, 0x6e, 0x64, 0x73, 0x12, 0x21, 0x2e, 0x6c, 0x6f, 0x61, 0x64, 0x62, 0x61, 0x6c,
	0x61, 0x6e, 0x63, 0x65, 0x72, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e,
	0x64, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x22, 0x2e, 0x6c, 0x6f, 0x61, 0x64,
	0x62, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x72, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x42, 0x61, 0x63,
	0x6b, 0x65, 0x6e, 0x64, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x28, 0x5a,
	0x26, 0x6c, 0x65, 0x62, 0x61, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x6c,
	0x6f, 0x61, 0x64, 0x62, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x72, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x3b, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_loadbalancer_proto_rawDescOnce sync.Once
	file_loadbalancer_proto_rawDescData = file_loadbalancer_proto_rawDesc
)

func file_loadbalancer_proto_rawDescGZIP() []byte {
	file_loadbalancer_proto_rawDescOnce.Do(func() {
		file_loadbalancer_proto_rawDescData = protoimpl.X.CompressGZIP(file_loadbalancer_proto_rawDescData)
	})
	return file_loadbalancer_proto_rawDescData
}

var file_loadbalancer_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_loadbalancer_proto_goTypes = []any{
	(*BackendRequest)(nil),       // 0: loadbalancer.BackendRequest
	(*BackendResponse)(nil),      // 1: loadbalancer.BackendResponse
	(*RemoveBackendRequest)(nil), // 2: loadbalancer.RemoveBackendRequest
	(*ListBackendsRequest)(nil),  // 3: loadbalancer.ListBackendsRequest
	(*ListBackendsResponse)(nil), // 4: loadbalancer.ListBackendsResponse
	(*BackendInfo)(nil),          // 5: loadbalancer.BackendInfo
}
var file_loadbalancer_proto_depIdxs = []int32{
	5, // 0: loadbalancer.ListBackendsResponse.backends:type_name -> loadbalancer.BackendInfo
	0, // 1: loadbalancer.LoadBalancer.AddBackend:input_type -> loadbalancer.BackendRequest
	2, // 2: loadbalancer.LoadBalancer.RemoveBackend:input_type -> loadbalancer.RemoveBackendRequest
	3, // 3: loadbalancer.LoadBalancer.ListBackends:input_type -> loadbalancer.ListBackendsRequest
	1, // 4: loadbalancer.LoadBalancer.AddBackend:output_type -> loadbalancer.BackendResponse
	1, // 5: loadbalancer.LoadBalancer.RemoveBackend:output_type -> loadbalancer.BackendResponse
	4, // 6: loadbalancer.LoadBalancer.ListBackends:output_type -> loadbalancer.ListBackendsResponse
	4, // [4:7] is the sub-list for method output_type
	1, // [1:4] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_loadbalancer_proto_init() }
func file_loadbalancer_proto_init() {
	if File_loadbalancer_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_loadbalancer_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*BackendRequest); i {
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
		file_loadbalancer_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*BackendResponse); i {
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
		file_loadbalancer_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*RemoveBackendRequest); i {
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
		file_loadbalancer_proto_msgTypes[3].Exporter = func(v any, i int) any {
			switch v := v.(*ListBackendsRequest); i {
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
		file_loadbalancer_proto_msgTypes[4].Exporter = func(v any, i int) any {
			switch v := v.(*ListBackendsResponse); i {
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
		file_loadbalancer_proto_msgTypes[5].Exporter = func(v any, i int) any {
			switch v := v.(*BackendInfo); i {
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
			RawDescriptor: file_loadbalancer_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_loadbalancer_proto_goTypes,
		DependencyIndexes: file_loadbalancer_proto_depIdxs,
		MessageInfos:      file_loadbalancer_proto_msgTypes,
	}.Build()
	File_loadbalancer_proto = out.File
	file_loadbalancer_proto_rawDesc = nil
	file_loadbalancer_proto_goTypes = nil
	file_loadbalancer_proto_depIdxs = nil
}
