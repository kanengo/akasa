package proto

import (
	"encoding/base64"

	"github.com/kanengo/akasar/runtime/protos"
	"google.golang.org/protobuf/proto"
)

func ToEnv(args *protos.AkasaletArgs) (string, error) {
	data, err := proto.Marshal(args)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

func FromEnv(in string, msg proto.Message) error {
	data, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return err
	}

	return proto.Unmarshal(data, msg)
}
