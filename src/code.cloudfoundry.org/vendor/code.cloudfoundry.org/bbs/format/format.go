package format

import (
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/lager/v3"
)

type serializer struct {
	encoder Encoder
}

type Serializer interface {
	Marshal(logger lager.Logger, model Model) ([]byte, error)
	Unmarshal(logger lager.Logger, encodedPayload []byte, model Model) error
}

func NewSerializer(cryptor encryption.Cryptor) Serializer {
	return &serializer{
		encoder: NewEncoder(cryptor),
	}
}

func (s *serializer) Marshal(logger lager.Logger, model Model) ([]byte, error) {
	envelopedPayload, err := MarshalEnvelope(model)
	if err != nil {
		return nil, err
	}

	return s.encoder.Encode(envelopedPayload)
}

func (s *serializer) Unmarshal(logger lager.Logger, encodedPayload []byte, model Model) error {
	unencodedPayload, err := s.encoder.Decode(encodedPayload)
	if err != nil {
		return err
	}
	return UnmarshalEnvelope(logger, unencodedPayload, model)
}
