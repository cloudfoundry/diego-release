package db

import (
	"context"

	"code.cloudfoundry.org/lager/v3"
)

//counterfeiter:generate . EncryptionDB

type EncryptionDB interface {
	EncryptionKeyLabel(ctx context.Context, logger lager.Logger) (string, error)
	SetEncryptionKeyLabel(ctx context.Context, logger lager.Logger, encryptionKeyLabel string) error
	PerformEncryption(ctx context.Context, logger lager.Logger) error
}
