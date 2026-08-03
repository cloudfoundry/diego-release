package db

import (
	"context"

	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/locket/models"
)

func lagerDataFromLock(resource *models.Resource) lager.Data {
	return lager.Data{
		"key":       resource.GetKey(),
		"owner":     resource.GetOwner(),
		"type-code": resource.GetTypeCode(),
	}
}

func (db *SQLDB) Lock(ctx context.Context, logger lager.Logger, resource *models.Resource, ttl int64) (*Lock, error) {
	logger = logger.Session("lock", lagerDataFromLock(resource))
	var lock *Lock

	var newLock bool

	err := db.helper.Transact(ctx, logger, db, func(logger lager.Logger, tx helpers.Tx) error {
		newLock = false
		res, index, id, _, err := db.fetchLock(ctx, logger, tx, resource.Key)
		if err != nil {
			sqlErr := db.helper.ConvertSQLError(err)
			if sqlErr != helpers.ErrResourceNotFound {
				logger.Error("failed-to-fetch-lock", err)
				return err
			}
			newLock = true
		} else if res.Owner != resource.Owner && res.Owner != "" {
			logger.Debug("lock-already-exists")
			return models.ErrLockCollision
		}

		index++

		modifiedId := id
		if modifiedId == "" {
			modifiedId, err = db.guidProvider.NextGUID()
			if err != nil {
				logger.Error("failed-to-generate-guid", err)
				return err
			}
		}

		lock = &Lock{
			Resource:      models.GetResource(resource),
			ModifiedIndex: index,
			ModifiedId:    modifiedId,
			TtlInSeconds:  ttl,
		}

		if newLock {
			_, err = db.helper.Insert(ctx, logger, tx, "locks",
				helpers.SQLAttributes{
					"path":           lock.Key,
					"owner":          lock.Owner,
					"value":          lock.Value,
					"type":           lock.Type,
					"modified_index": lock.ModifiedIndex,
					"modified_id":    lock.ModifiedId,
					"ttl":            lock.TtlInSeconds,
				},
			)
		} else {
			_, err = db.helper.Update(ctx, logger, tx, "locks",
				helpers.SQLAttributes{
					"owner":          lock.Owner,
					"value":          lock.Value,
					"type":           lock.Type,
					"modified_index": lock.ModifiedIndex,
					"modified_id":    lock.ModifiedId,
					"ttl":            lock.TtlInSeconds,
				},
				"path = ?", lock.Key,
			)
		}

		if err != nil {
			logger.Error("failed-updating-lock", err)
			return err
		}

		return nil
	})

	if err == nil && newLock {
		logger.Info("acquired-lock")
	}

	return lock, db.helper.ConvertSQLError(err)
}

func (db *SQLDB) Release(ctx context.Context, logger lager.Logger, resource *models.Resource) error {
	logger = logger.Session("release-lock", lagerDataFromLock(resource))

	err := db.helper.Transact(ctx, logger, db, func(logger lager.Logger, tx helpers.Tx) error {
		res, _, _, _, err := db.fetchLock(ctx, logger, tx, resource.Key)
		if err != nil {
			sqlErr := db.helper.ConvertSQLError(err)
			if sqlErr == helpers.ErrResourceNotFound {
				logger.Debug("lock-does-not-exist")
				return nil
			}
			logger.Error("failed-to-fetch-lock", err)
			return sqlErr
		}

		if res.Owner != resource.Owner {
			logger.Error("cannot-release-lock", models.ErrLockCollision)
			return models.ErrLockCollision
		}

		_, err = db.helper.Delete(ctx, logger, tx, "locks",
			"path = ?", resource.Key,
		)
		if err != nil {
			logger.Error("failed-to-release-lock", err)
			return db.helper.ConvertSQLError(err)
		}
		logger.Info("released-lock")
		return nil
	})
	return err
}

func (db *SQLDB) Fetch(ctx context.Context, logger lager.Logger, key string) (*Lock, error) {
	logger = logger.Session("fetch-lock", lager.Data{"key": key})
	var lock *Lock

	err := db.helper.Transact(ctx, logger, db, func(logger lager.Logger, tx helpers.Tx) error {
		res, index, id, ttl, err := db.fetchLock(ctx, logger, tx, key)
		if err != nil {
			logger.Error("failed-to-fetch-lock", err)
			sqlErr := db.helper.ConvertSQLError(err)
			if sqlErr == helpers.ErrResourceNotFound {
				return models.ErrResourceNotFound
			}
			return sqlErr
		}

		if res.Owner == "" {
			return models.ErrResourceNotFound
		}

		lock = &Lock{Resource: res, ModifiedIndex: index, ModifiedId: id, TtlInSeconds: ttl}

		return nil
	})

	return lock, err
}

func (db *SQLDB) FetchAll(ctx context.Context, logger lager.Logger, lockType string) ([]*Lock, error) {
	logger = logger.Session("fetch-all-locks", lager.Data{"type": lockType})
	var locks []*Lock

	err := db.helper.Transact(ctx, logger, db, func(logger lager.Logger, tx helpers.Tx) error {
		var where string
		whereBindings := make([]interface{}, 0)

		if lockType != "" {
			where = "type = ?"
			whereBindings = append(whereBindings, lockType)
		}

		rows, err := db.helper.All(ctx, logger, tx, "locks",
			helpers.ColumnList{"path", "owner", "value", "type", "modified_index", "modified_id", "ttl"},
			helpers.NoLockRow, where, whereBindings...,
		)
		if err != nil {
			logger.Error("failed-to-fetch-locks", err)
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var key, owner, value, lockType, id string
			var index, ttl int64

			err := rows.Scan(&key, &owner, &value, &lockType, &index, &id, &ttl)
			if err != nil {
				logger.Error("failed-to-scan-lock", err)
				continue
			}

			if owner == "" {
				continue
			}

			locks = append(locks, &Lock{
				Resource: &models.Resource{
					Key:      key,
					Owner:    owner,
					Value:    value,
					Type:     lockType,
					TypeCode: models.GetTypeCode(lockType),
				},
				ModifiedIndex: index,
				ModifiedId:    id,
				TtlInSeconds:  ttl,
			})
		}

		return nil
	})

	return locks, db.helper.ConvertSQLError(err)
}

func (db *SQLDB) Count(ctx context.Context, logger lager.Logger, lockType string) (int, error) {
	whereBindings := make([]interface{}, 0)
	wheres := "owner <> ?"
	whereBindings = append(whereBindings, "")

	if lockType != "" {
		wheres += " AND type = ?"
		whereBindings = append(whereBindings, lockType)
	}

	logger = logger.Session("count-locks")
	count, err := db.helper.Count(ctx, logger, db, "locks", wheres, whereBindings...)
	return count, db.helper.ConvertSQLError(err)
}

func (db *SQLDB) fetchLock(ctx context.Context, logger lager.Logger, q helpers.Queryable, key string) (*models.Resource, int64, string, int64, error) {
	row := db.helper.One(ctx, logger, q, "locks",
		helpers.ColumnList{"owner", "value", "type", "modified_index", "modified_id", "ttl"},
		helpers.LockRow,
		"path = ?", key,
	)

	var owner, value, lockType, id string
	var index, ttl int64
	err := row.Scan(&owner, &value, &lockType, &index, &id, &ttl)
	if err != nil {
		return nil, 0, "", 0, err
	}

	return &models.Resource{
		Key:      key,
		Owner:    owner,
		Value:    value,
		Type:     lockType,
		TypeCode: models.GetTypeCode(lockType),
	}, index, id, ttl, nil
}

func (db *SQLDB) FetchAndRelease(ctx context.Context, logger lager.Logger, lock *Lock) (bool, error) {
	logger = logger.Session("fetch-and-release-lock", lagerDataFromLock(lock.Resource))

	err := db.helper.Transact(ctx, logger, db, func(logger lager.Logger, tx helpers.Tx) error {
		res, index, id, ttl, err := db.fetchLock(ctx, logger, tx, lock.Resource.Key)

		if err != nil {
			sqlErr := db.helper.ConvertSQLError(err)
			if sqlErr == helpers.ErrResourceNotFound {
				logger.Debug("lock-does-not-exist")
				return models.ErrResourceNotFound
			}
			logger.Error("failed-to-fetch-lock", err)
			return sqlErr
		}

		logger.Info("fetched-lock")

		fetchedLock := &Lock{Resource: res, ModifiedIndex: index, ModifiedId: id, TtlInSeconds: ttl}

		if fetchedLock.Resource.Owner != lock.Resource.Owner {
			logger.Error("fetch-failed-owner-mismatch", models.ErrLockCollision, lager.Data{"fetched-owner": fetchedLock.Owner})
			return models.ErrLockCollision
		}

		if fetchedLock.ModifiedId != lock.ModifiedId {
			logger.Error("release-failed-id-mismatch", models.ErrLockCollision, lager.Data{"lock-modified-id": lock.ModifiedId, "fetched-modified-id": fetchedLock.ModifiedId})
			return models.ErrLockCollision
		}

		if fetchedLock.ModifiedIndex != lock.ModifiedIndex {
			logger.Error("release-failed-index-mismatch", models.ErrLockCollision, lager.Data{"lock-modified-index": lock.ModifiedIndex, "fetched-modified-index": fetchedLock.ModifiedIndex})
			return models.ErrLockCollision
		}

		_, err = db.helper.Delete(ctx, logger, tx, "locks",
			"path = ?", fetchedLock.Resource.Key,
		)

		if err != nil {
			logger.Error("failed-to-release-lock", err)
			return err
		}

		logger.Info("released-lock")

		return nil
	})

	if err != nil {
		if err == models.ErrResourceNotFound {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
