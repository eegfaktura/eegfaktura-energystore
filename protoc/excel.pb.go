// Code generated by proto-gen-go. DO NOT EDIT.
// versions:
// 	proto-gen-go v1.30.0
// 	proto        v3.12.4
// source: proto/excel.proto

package protobuf

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

// The request message containing the user's name.
type SendExcelRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Tenant    string `protobuf:"bytes,1,opt,name=tenant,proto3" json:"tenant,omitempty"`
	Recipient string `protobuf:"bytes,2,opt,name=recipient,proto3" json:"recipient,omitempty"`
	Filename  string `protobuf:"bytes,3,opt,name=filename,proto3" json:"filename,omitempty"`
	Content   []byte `protobuf:"bytes,4,opt,name=content,proto3" json:"content,omitempty"`
}

func (x *SendExcelRequest) Reset() {
	*x = SendExcelRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_protoc_excel_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SendExcelRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SendExcelRequest) ProtoMessage() {}

func (x *SendExcelRequest) ProtoReflect() protoreflect.Message {
	mi := &file_protoc_excel_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SendExcelRequest.ProtoReflect.Descriptor instead.
func (*SendExcelRequest) Descriptor() ([]byte, []int) {
	return file_protoc_excel_proto_rawDescGZIP(), []int{0}
}

func (x *SendExcelRequest) GetTenant() string {
	if x != nil {
		return x.Tenant
	}
	return ""
}

func (x *SendExcelRequest) GetRecipient() string {
	if x != nil {
		return x.Recipient
	}
	return ""
}

func (x *SendExcelRequest) GetFilename() string {
	if x != nil {
		return x.Filename
	}
	return ""
}

func (x *SendExcelRequest) GetContent() []byte {
	if x != nil {
		return x.Content
	}
	return nil
}

// The response message containing the greetings
type SendExcelReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Status string `protobuf:"bytes,1,opt,name=status,proto3" json:"status,omitempty"`
}

func (x *SendExcelReply) Reset() {
	*x = SendExcelReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_protoc_excel_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SendExcelReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SendExcelReply) ProtoMessage() {}

func (x *SendExcelReply) ProtoReflect() protoreflect.Message {
	mi := &file_protoc_excel_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SendExcelReply.ProtoReflect.Descriptor instead.
func (*SendExcelReply) Descriptor() ([]byte, []int) {
	return file_protoc_excel_proto_rawDescGZIP(), []int{1}
}

func (x *SendExcelReply) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

var File_protoc_excel_proto protoreflect.FileDescriptor

var file_protoc_excel_proto_rawDesc = []byte{
	0x0a, 0x12, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x2f, 0x65, 0x78, 0x63, 0x65, 0x6c, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0d, 0x61, 0x74, 0x2e, 0x65, 0x6e, 0x65, 0x72, 0x67, 0x79, 0x64,
	0x61, 0x73, 0x68, 0x22, 0x7e, 0x0a, 0x10, 0x53, 0x65, 0x6e, 0x64, 0x45, 0x78, 0x63, 0x65, 0x6c,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x74, 0x65, 0x6e, 0x61, 0x6e,
	0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x12,
	0x1c, 0x0a, 0x09, 0x72, 0x65, 0x63, 0x69, 0x70, 0x69, 0x65, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x09, 0x72, 0x65, 0x63, 0x69, 0x70, 0x69, 0x65, 0x6e, 0x74, 0x12, 0x1a, 0x0a,
	0x08, 0x66, 0x69, 0x6c, 0x65, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x08, 0x66, 0x69, 0x6c, 0x65, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x6f, 0x6e,
	0x74, 0x65, 0x6e, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x07, 0x63, 0x6f, 0x6e, 0x74,
	0x65, 0x6e, 0x74, 0x22, 0x28, 0x0a, 0x0e, 0x53, 0x65, 0x6e, 0x64, 0x45, 0x78, 0x63, 0x65, 0x6c,
	0x52, 0x65, 0x70, 0x6c, 0x79, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x32, 0x62, 0x0a,
	0x11, 0x45, 0x78, 0x63, 0x65, 0x6c, 0x41, 0x64, 0x6d, 0x69, 0x6e, 0x53, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x12, 0x4d, 0x0a, 0x09, 0x53, 0x65, 0x6e, 0x64, 0x45, 0x78, 0x63, 0x65, 0x6c, 0x12,
	0x1f, 0x2e, 0x61, 0x74, 0x2e, 0x65, 0x6e, 0x65, 0x72, 0x67, 0x79, 0x64, 0x61, 0x73, 0x68, 0x2e,
	0x53, 0x65, 0x6e, 0x64, 0x45, 0x78, 0x63, 0x65, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x1a, 0x1d, 0x2e, 0x61, 0x74, 0x2e, 0x65, 0x6e, 0x65, 0x72, 0x67, 0x79, 0x64, 0x61, 0x73, 0x68,
	0x2e, 0x53, 0x65, 0x6e, 0x64, 0x45, 0x78, 0x63, 0x65, 0x6c, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x22,
	0x00, 0x42, 0x52, 0x0a, 0x19, 0x61, 0x74, 0x2e, 0x65, 0x6e, 0x65, 0x72, 0x67, 0x79, 0x64, 0x61,
	0x73, 0x68, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x65, 0x78, 0x63, 0x65, 0x6c, 0x42, 0x0f,
	0x45, 0x78, 0x63, 0x65, 0x6c, 0x41, 0x64, 0x6d, 0x69, 0x6e, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50,
	0x01, 0x5a, 0x22, 0x61, 0x74, 0x2e, 0x6f, 0x75, 0x72, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74,
	0x2f, 0x65, 0x6e, 0x65, 0x72, 0x67, 0x79, 0x73, 0x74, 0x6f, 0x72, 0x65, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_protoc_excel_proto_rawDescOnce sync.Once
	file_protoc_excel_proto_rawDescData = file_protoc_excel_proto_rawDesc
)

func file_protoc_excel_proto_rawDescGZIP() []byte {
	file_protoc_excel_proto_rawDescOnce.Do(func() {
		file_protoc_excel_proto_rawDescData = protoimpl.X.CompressGZIP(file_protoc_excel_proto_rawDescData)
	})
	return file_protoc_excel_proto_rawDescData
}

var file_protoc_excel_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_protoc_excel_proto_goTypes = []interface{}{
	(*SendExcelRequest)(nil), // 0: at.energydash.SendExcelRequest
	(*SendExcelReply)(nil),   // 1: at.energydash.SendExcelReply
}
var file_protoc_excel_proto_depIdxs = []int32{
	0, // 0: at.energydash.ExcelAdminService.SendExcel:input_type -> at.energydash.SendExcelRequest
	1, // 1: at.energydash.ExcelAdminService.SendExcel:output_type -> at.energydash.SendExcelReply
	1, // [1:2] is the sub-list for method output_type
	0, // [0:1] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_protoc_excel_proto_init() }
func file_protoc_excel_proto_init() {
	if File_protoc_excel_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_protoc_excel_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SendExcelRequest); i {
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
		file_protoc_excel_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SendExcelReply); i {
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
			RawDescriptor: file_protoc_excel_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_protoc_excel_proto_goTypes,
		DependencyIndexes: file_protoc_excel_proto_depIdxs,
		MessageInfos:      file_protoc_excel_proto_msgTypes,
	}.Build()
	File_protoc_excel_proto = out.File
	file_protoc_excel_proto_rawDesc = nil
	file_protoc_excel_proto_goTypes = nil
	file_protoc_excel_proto_depIdxs = nil
}