package guidprovider

import uuid "github.com/nu7hatch/gouuid"

//go:generate counterfeiter -generate

//counterfeiter:generate . GUIDProvider

type GUIDProvider interface {
	NextGUID() (string, error)
}

var DefaultGuidProvider GUIDProvider = &guidProvider{}

type guidProvider struct{}

func (*guidProvider) NextGUID() (string, error) {
	guid, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	return guid.String(), nil
}
