package sqldb

import (
	"context"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

func (db *SQLDB) RemoveSuspectActualLRP(ctx context.Context, logger lager.Logger, lrpKey *models.ActualLRPKey) (*models.ActualLRP, error) {
	logger = logger.Session("db-remove-suspect-actual-lrp", lager.Data{"lrp_key": lrpKey})
	logger.Debug("starting")
	defer logger.Debug("complete")

	var (
		lrp *models.ActualLRP
		err error
	)

	err = db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		processGuid := lrpKey.ProcessGuid
		index := lrpKey.Index

		lrp, err = db.fetchActualLRPForUpdate(ctx, logger, processGuid, index, models.ActualLRP_Suspect, tx)
		if err == models.ErrResourceNotFound {
			logger.Debug("suspect-lrp-does-not-exist")
			return nil
		}

		if err != nil {
			logger.Error("failed-fetching-actual-lrp", err)
			return err
		}

		_, err = db.delete(ctx, logger, tx, "actual_lrps",
			"process_guid = ? AND instance_index = ? AND presence = ?",
			processGuid, index, models.ActualLRP_Suspect,
		)

		if err != nil {
			logger.Error("failed-delete", err)
			return models.ErrActualLRPCannotBeRemoved
		}

		return nil
	})

	return lrp, err
}

func (db *SQLDB) PromoteSuspectActualLRP(ctx context.Context, logger lager.Logger, processGuid string, index int32) (*models.ActualLRP, *models.ActualLRP, *models.ActualLRP, error) {
	logger = logger.Session("promote-suspect-actual-lrp", lager.Data{"process_guid": processGuid, "index": index})
	logger.Info("starting")
	defer logger.Info("complete")

	var (
		beforeLRP   *models.ActualLRP
		afterLRP    models.ActualLRP
		ordinaryLRP *models.ActualLRP
	)
	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		beforeLRP, err = db.fetchActualLRPForUpdate(ctx, logger, processGuid, index, models.ActualLRP_Suspect, tx)
		if err != nil {
			logger.Error("failed-fetching-suspect-actual-lrp", err)
			return err
		}

		ordinaryLRP, err = db.fetchActualLRPForUpdate(ctx, logger, processGuid, index, models.ActualLRP_Ordinary, tx)
		if err != nil && err != models.ErrResourceNotFound {
			logger.Error("failed-fetching-ordinary-actual-lrp", err)
			return err
		}
		if err != models.ErrResourceNotFound {
			_, err = db.delete(ctx, logger, tx, actualLRPsTable,
				"process_guid = ? AND instance_index = ? AND presence = ?",
				processGuid, index, models.ActualLRP_Ordinary,
			)
			if err != nil {
				logger.Error("failed-removing-ordinaryactual-lrp", err)
				return err
			}
		}

		afterLRP = *beforeLRP
		afterLRP.Presence = models.ActualLRP_Ordinary
		wheres := "process_guid = ? AND instance_index = ? AND presence = ?"
		_, err = db.update(ctx, logger, tx, actualLRPsTable, helpers.SQLAttributes{
			"presence": afterLRP.Presence,
		}, wheres, processGuid, index, beforeLRP.Presence)
		if err != nil {
			logger.Error("failed-updating-lrp", err)
		}

		return nil
	})

	return beforeLRP, &afterLRP, ordinaryLRP, err
}
