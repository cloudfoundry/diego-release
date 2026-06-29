package sqldb

import (
	"context"

	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/diego-db-helpers/guidprovider"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
)

type SQLDB struct {
	db                            helpers.QueryableDB
	convergenceWorkersSize        int
	updateWorkersSize             int
	clock                         clock.Clock
	guidProvider                  guidprovider.GUIDProvider
	serializer                    format.Serializer
	cryptor                       encryption.Cryptor
	encoder                       format.Encoder
	flavor                        string
	helper                        helpers.SQLHelper
	metronClient                  loggingclient.IngressClient
	debugStartActualLRPHeartbeats bool
}

func NewSQLDB(
	db helpers.QueryableDB,
	convergenceWorkersSize int,
	updateWorkersSize int,
	cryptor encryption.Cryptor,
	guidProvider guidprovider.GUIDProvider,
	clock clock.Clock,
	flavor string,
	metronClient loggingclient.IngressClient,
	debugStartActualLRPHeartbeats bool,
) *SQLDB {
	helper := helpers.NewSQLHelper(flavor)
	return &SQLDB{
		db:                            db,
		convergenceWorkersSize:        convergenceWorkersSize,
		updateWorkersSize:             updateWorkersSize,
		clock:                         clock,
		guidProvider:                  guidProvider,
		serializer:                    format.NewSerializer(cryptor),
		cryptor:                       cryptor,
		encoder:                       format.NewEncoder(cryptor),
		flavor:                        flavor,
		helper:                        helper,
		metronClient:                  metronClient,
		debugStartActualLRPHeartbeats: debugStartActualLRPHeartbeats,
	}
}

func (db *SQLDB) transact(ctx context.Context, logger lager.Logger, f func(logger lager.Logger, tx helpers.Tx) error) error {
	err := db.helper.Transact(ctx, logger, db.db, f)
	if err != nil {
		return db.convertSQLError(err)
	}
	return nil
}

func (db *SQLDB) serializeModel(logger lager.Logger, model format.Model) ([]byte, error) {
	encodedPayload, err := db.serializer.Marshal(logger, model)
	if err != nil {
		logger.Error("failed-to-serialize-model", err)
		return nil, models.NewError(models.Error_InvalidRecord, err.Error())
	}
	return encodedPayload, nil
}

func (db *SQLDB) deserializeModel(logger lager.Logger, data []byte, model format.Model) error {
	err := db.serializer.Unmarshal(logger, data, model)
	if err != nil {
		logger.Error("failed-to-deserialize-model", err)
		return models.NewError(models.Error_InvalidRecord, err.Error())
	}
	return nil
}

func (db *SQLDB) convertSQLError(err error) *models.Error {
	converted := db.helper.ConvertSQLError(err)
	switch converted {
	case helpers.ErrResourceExists:
		return models.ErrResourceExists
	case helpers.ErrDeadlock:
		return models.ErrDeadlock
	case helpers.ErrBadRequest:
		return models.ErrBadRequest
	case helpers.ErrUnrecoverableError:
		return models.NewUnrecoverableError(err)
	case helpers.ErrResourceNotFound:
		return models.ErrResourceNotFound
	default:
		return models.ConvertError(err)
	}
}
