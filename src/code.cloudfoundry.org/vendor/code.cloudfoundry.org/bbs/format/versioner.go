package format

import "github.com/gogo/protobuf/proto"

type Version byte

const (
	V0 Version = 0
	V1 Version = 1
	V2 Version = 2
	V3 Version = 3
)

type Model interface {
	proto.Message
}
