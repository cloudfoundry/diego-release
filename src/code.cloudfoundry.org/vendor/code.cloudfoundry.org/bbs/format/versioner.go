package format

import "github.com/gogo/protobuf/proto"

type Version byte

const (
	V0 Version = 0
	V1         = 1
	V2         = 2
	V3         = 3
)

type Model interface {
	proto.Message
}
