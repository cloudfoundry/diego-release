package format

import (
	"code.cloudfoundry.org/lager/v3"
	"github.com/gogo/protobuf/proto"
)

type EnvelopeFormat byte

const (
	PROTO EnvelopeFormat = 2
)

const EnvelopeOffset int = 2

func UnmarshalEnvelope(logger lager.Logger, unencodedPayload []byte, model Model) error {
	return UnmarshalProto(logger, unencodedPayload[EnvelopeOffset:], model)
}

// dummy version for backward compatability. old BBS used to serialize proto
// messages with a 2-byte header that has the envelope format (i.e. PROTO) and
// the version of the model (e.g. 0, 1 or 2). Adding the version was a
// pre-mature optimization that we decided to get rid of in #133215113. That
// said, we have the ensure the header is a 2-byte to avoid breaking older BBS
// Deprecated: do not use, see note above
const version = 0

func MarshalEnvelope(model Model) ([]byte, error) {
	var payload []byte
	var err error

	payload, err = MarshalProto(model)

	if err != nil {
		return nil, err
	}

	data := make([]byte, 0, len(payload)+EnvelopeOffset)
	data = append(data, byte(PROTO), byte(version))
	data = append(data, payload...)

	return data, nil
}

func UnmarshalProto(logger lager.Logger, marshaledPayload []byte, model Model) error {
	err := proto.Unmarshal(marshaledPayload, model)
	if err != nil {
		logger.Error("failed-to-proto-unmarshal-payload", err)
		return err
	}
	return nil
}

func MarshalProto(v Model) ([]byte, error) {
	bytes, err := proto.Marshal(v)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}
