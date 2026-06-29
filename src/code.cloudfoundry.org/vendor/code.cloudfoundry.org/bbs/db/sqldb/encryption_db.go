package sqldb

import (
	"context"
	"fmt"
	"strings"

	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

const EncryptionKeyID = "encryption_key_label"

func (db *SQLDB) SetEncryptionKeyLabel(ctx context.Context, logger lager.Logger, label string) error {
	logger = logger.Session("db-set-encryption-key-label", lager.Data{"label": label})
	logger.Debug("starting")
	defer logger.Debug("complete")

	return db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		return db.setConfigurationValue(tx, ctx, logger, EncryptionKeyID, label)
	})
}

func (db *SQLDB) EncryptionKeyLabel(ctx context.Context, logger lager.Logger) (string, error) {
	logger = logger.Session("db-encryption-key-label")
	logger.Debug("starting")
	defer logger.Debug("complete")

	var ekLabel string
	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var getErr error
		ekLabel, getErr = db.getConfigurationValue(tx, ctx, logger, EncryptionKeyID)
		return getErr
	})
	if err != nil {
		return "", err
	}
	return ekLabel, nil
}

func (db *SQLDB) PerformEncryption(ctx context.Context, logger lager.Logger) error {
	errCh := make(chan error)

	funcs := []func(){
		func() {
			errCh <- db.reEncrypt(ctx, logger, encryptable{
				TableName:       tasksTable,
				PrimaryKeyNames: []string{"guid"},
				Columns:         []string{"task_definition"},
				EncryptIfEmpty:  true,
				PrimaryKeyFunc:  func() primaryKey { return &taskPrimaryKey{} },
			})
		},
		func() {
			errCh <- db.reEncrypt(ctx, logger, encryptable{
				TableName:       desiredLRPsTable,
				PrimaryKeyNames: []string{"process_guid"},
				Columns:         []string{"run_info", "volume_placement", "routes", "metric_tags"},
				EncryptIfEmpty:  true,
				PrimaryKeyFunc:  func() primaryKey { return &desiredLRPPrimaryKey{} },
			})
		},
		func() {
			errCh <- db.reEncrypt(ctx, logger, encryptable{
				TableName:       actualLRPsTable,
				PrimaryKeyNames: []string{"process_guid", "instance_index", "presence"},
				Columns:         []string{"net_info", "internal_routes", "metric_tags"},
				EncryptIfEmpty:  false,
				PrimaryKeyFunc:  func() primaryKey { return &actualLRPPrimaryKey{} },
			})
		},
	}

	for _, f := range funcs {
		go f()
	}

	for range funcs {
		err := <-errCh
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *SQLDB) reEncrypt(ctx context.Context, logger lager.Logger, toEncrypt encryptable) error {
	logger = logger.WithData(
		lager.Data{"table_name": toEncrypt.TableName, "primary_key": toEncrypt.PrimaryKeyNames, "blob_columns": toEncrypt.Columns},
	)
	rows, err := db.db.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s", strings.Join(toEncrypt.PrimaryKeyNames, ", "), toEncrypt.TableName))
	if err != nil {
		return err
	}
	defer rows.Close()

	pks := []primaryKey{}
	for rows.Next() {
		pk := toEncrypt.PrimaryKeyFunc()
		err := pk.Scan(rows)
		if err != nil {
			logger.Error("failed-to-scan-primary-key", err)
			continue
		}
		pks = append(pks, pk)
	}

	whereClauses := []string{}
	for _, name := range toEncrypt.PrimaryKeyNames {
		whereClauses = append(whereClauses, name+" = ?")

	}
	where := strings.Join(whereClauses, " AND ")

	for _, pk := range pks {
		err = db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
			blobs := make([]interface{}, len(toEncrypt.Columns))

			row := db.one(ctx, logger, tx, toEncrypt.TableName, toEncrypt.Columns, helpers.LockRow, where, pk.WhereBindings()...)
			for i := range toEncrypt.Columns {
				var blob []byte
				blobs[i] = &blob
			}

			err := row.Scan(blobs...)
			if err != nil {
				logger.Error("failed-to-scan-blob", err)
				return nil
			}

			updatedColumnValues := map[string]interface{}{}

			for columnIdx := range blobs {
				// This type assertion should not fail because we set the value to be a pointer to a byte array above
				blobPtr := blobs[columnIdx].(*[]byte)
				blob := *blobPtr

				// don't encrypt column if it doesn't contain any data, see #132626553 for more info
				if !toEncrypt.EncryptIfEmpty && len(blob) == 0 {
					return nil
				}

				encoder := format.NewEncoder(db.cryptor)
				payload, err := encoder.Decode(blob)
				if err != nil {
					logger.Error("failed-to-decode-blob", err)
					return nil
				}
				encryptedPayload, err := encoder.Encode(payload)
				if err != nil {
					logger.Error("failed-to-encode-blob", err)
					return err
				}

				columnName := toEncrypt.Columns[columnIdx]
				updatedColumnValues[columnName] = encryptedPayload
			}
			_, err = db.update(ctx, logger, tx, toEncrypt.TableName,
				updatedColumnValues,
				where, pk.WhereBindings()...,
			)
			if err != nil {
				logger.Error("failed-to-update-blob", err)
				return err
			}
			return nil
		})

		if err != nil {
			return err
		}
	}

	return nil
}

type encryptable struct {
	TableName       string
	PrimaryKeyNames []string
	Columns         []string
	EncryptIfEmpty  bool
	PrimaryKeyFunc  func() primaryKey
}

type primaryKey interface {
	Scan(row helpers.RowScanner) error
	WhereBindings() []interface{}
}

type actualLRPPrimaryKey struct {
	ProcessGuid   string
	InstanceIndex int32
	Presence      string
}

func (pk *actualLRPPrimaryKey) Scan(row helpers.RowScanner) error {
	return row.Scan(&pk.ProcessGuid, &pk.InstanceIndex, &pk.Presence)
}

func (pk *actualLRPPrimaryKey) WhereBindings() []interface{} {
	return []interface{}{pk.ProcessGuid, pk.InstanceIndex, pk.Presence}
}

type desiredLRPPrimaryKey struct {
	ProcessGuid string
}

func (pk *desiredLRPPrimaryKey) Scan(row helpers.RowScanner) error {
	return row.Scan(&pk.ProcessGuid)
}

func (pk *desiredLRPPrimaryKey) WhereBindings() []interface{} {
	return []interface{}{pk.ProcessGuid}
}

type taskPrimaryKey struct {
	Guid string
}

func (pk *taskPrimaryKey) Scan(row helpers.RowScanner) error {
	return row.Scan(&pk.Guid)
}

func (pk *taskPrimaryKey) WhereBindings() []interface{} {
	return []interface{}{pk.Guid}
}
