package protomsg

import "google.golang.org/protobuf/proto"

func Clone[P proto.Message](m P) P {
	return proto.Clone(m).(P)
}
