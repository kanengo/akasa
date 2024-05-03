// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        v5.27.0--rc1
// source: internal/tool/single/single.proto

package single

import (
	protos "github.com/kanengo/akasar/runtime/protos"
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

type SingleConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	App       *protos.AppConfig                        `protobuf:"bytes,1,opt,name=app,proto3" json:"app,omitempty"`
	Listeners map[string]*SingleConfig_ListenerOptions `protobuf:"bytes,2,rep,name=listeners,proto3" json:"listeners,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *SingleConfig) Reset() {
	*x = SingleConfig{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_tool_single_single_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SingleConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SingleConfig) ProtoMessage() {}

func (x *SingleConfig) ProtoReflect() protoreflect.Message {
	mi := &file_internal_tool_single_single_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SingleConfig.ProtoReflect.Descriptor instead.
func (*SingleConfig) Descriptor() ([]byte, []int) {
	return file_internal_tool_single_single_proto_rawDescGZIP(), []int{0}
}

func (x *SingleConfig) GetApp() *protos.AppConfig {
	if x != nil {
		return x.App
	}
	return nil
}

func (x *SingleConfig) GetListeners() map[string]*SingleConfig_ListenerOptions {
	if x != nil {
		return x.Listeners
	}
	return nil
}

type SingleConfig_ListenerOptions struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Address string `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
}

func (x *SingleConfig_ListenerOptions) Reset() {
	*x = SingleConfig_ListenerOptions{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_tool_single_single_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SingleConfig_ListenerOptions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SingleConfig_ListenerOptions) ProtoMessage() {}

func (x *SingleConfig_ListenerOptions) ProtoReflect() protoreflect.Message {
	mi := &file_internal_tool_single_single_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SingleConfig_ListenerOptions.ProtoReflect.Descriptor instead.
func (*SingleConfig_ListenerOptions) Descriptor() ([]byte, []int) {
	return file_internal_tool_single_single_proto_rawDescGZIP(), []int{0, 0}
}

func (x *SingleConfig_ListenerOptions) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

var File_internal_tool_single_single_proto protoreflect.FileDescriptor

var file_internal_tool_single_single_proto_rawDesc = []byte{
	0x0a, 0x21, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x74, 0x6f, 0x6f, 0x6c, 0x2f,
	0x73, 0x69, 0x6e, 0x67, 0x6c, 0x65, 0x2f, 0x73, 0x69, 0x6e, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x06, 0x73, 0x69, 0x6e, 0x67, 0x6c, 0x65, 0x1a, 0x1b, 0x72, 0x75, 0x6e,
	0x74, 0x69, 0x6d, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x73, 0x2f, 0x63, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x88, 0x02, 0x0a, 0x0c, 0x53, 0x69, 0x6e,
	0x67, 0x6c, 0x65, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x24, 0x0a, 0x03, 0x61, 0x70, 0x70,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x12, 0x2e, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65,
	0x2e, 0x41, 0x70, 0x70, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52, 0x03, 0x61, 0x70, 0x70, 0x12,
	0x41, 0x0a, 0x09, 0x6c, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x65, 0x72, 0x73, 0x18, 0x02, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x23, 0x2e, 0x73, 0x69, 0x6e, 0x67, 0x6c, 0x65, 0x2e, 0x53, 0x69, 0x6e, 0x67,
	0x6c, 0x65, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x65,
	0x72, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x09, 0x6c, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x65,
	0x72, 0x73, 0x1a, 0x2b, 0x0a, 0x0f, 0x4c, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x65, 0x72, 0x4f, 0x70,
	0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x1a,
	0x62, 0x0a, 0x0e, 0x4c, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x65, 0x72, 0x73, 0x45, 0x6e, 0x74, 0x72,
	0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03,
	0x6b, 0x65, 0x79, 0x12, 0x3a, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x24, 0x2e, 0x73, 0x69, 0x6e, 0x67, 0x6c, 0x65, 0x2e, 0x53, 0x69, 0x6e, 0x67,
	0x6c, 0x65, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x65,
	0x72, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a,
	0x02, 0x38, 0x01, 0x42, 0x30, 0x5a, 0x2e, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x6b, 0x61, 0x6e, 0x65, 0x6e, 0x67, 0x6f, 0x2f, 0x61, 0x6b, 0x61, 0x73, 0x61, 0x72,
	0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x74, 0x6f, 0x6f, 0x6c, 0x2f, 0x73,
	0x69, 0x6e, 0x67, 0x6c, 0x65, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_internal_tool_single_single_proto_rawDescOnce sync.Once
	file_internal_tool_single_single_proto_rawDescData = file_internal_tool_single_single_proto_rawDesc
)

func file_internal_tool_single_single_proto_rawDescGZIP() []byte {
	file_internal_tool_single_single_proto_rawDescOnce.Do(func() {
		file_internal_tool_single_single_proto_rawDescData = protoimpl.X.CompressGZIP(file_internal_tool_single_single_proto_rawDescData)
	})
	return file_internal_tool_single_single_proto_rawDescData
}

var file_internal_tool_single_single_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_internal_tool_single_single_proto_goTypes = []interface{}{
	(*SingleConfig)(nil),                 // 0: single.SingleConfig
	(*SingleConfig_ListenerOptions)(nil), // 1: single.SingleConfig.ListenerOptions
	nil,                                  // 2: single.SingleConfig.ListenersEntry
	(*protos.AppConfig)(nil),             // 3: runtime.AppConfig
}
var file_internal_tool_single_single_proto_depIdxs = []int32{
	3, // 0: single.SingleConfig.app:type_name -> runtime.AppConfig
	2, // 1: single.SingleConfig.listeners:type_name -> single.SingleConfig.ListenersEntry
	1, // 2: single.SingleConfig.ListenersEntry.value:type_name -> single.SingleConfig.ListenerOptions
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_internal_tool_single_single_proto_init() }
func file_internal_tool_single_single_proto_init() {
	if File_internal_tool_single_single_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_internal_tool_single_single_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SingleConfig); i {
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
		file_internal_tool_single_single_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SingleConfig_ListenerOptions); i {
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
			RawDescriptor: file_internal_tool_single_single_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_internal_tool_single_single_proto_goTypes,
		DependencyIndexes: file_internal_tool_single_single_proto_depIdxs,
		MessageInfos:      file_internal_tool_single_single_proto_msgTypes,
	}.Build()
	File_internal_tool_single_single_proto = out.File
	file_internal_tool_single_single_proto_rawDesc = nil
	file_internal_tool_single_single_proto_goTypes = nil
	file_internal_tool_single_single_proto_depIdxs = nil
}
