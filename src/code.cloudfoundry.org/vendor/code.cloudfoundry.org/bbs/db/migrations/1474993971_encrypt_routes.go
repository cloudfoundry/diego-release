package migrations

import (
	"database/sql"

	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

func init() {
	appendMigration(NewEncryptRoutes())
}

type EncryptRoutes struct {
	encoder  format.Encoder
	clock    clock.Clock
	dbFlavor string
}

func NewEncryptRoutes() migration.Migration {
	return &EncryptRoutes{}
}

func (e *EncryptRoutes) String() string {
	return migrationString(e)
}

func (e *EncryptRoutes) Version() int64 {
	return 1474993971
}

func (e *EncryptRoutes) SetCryptor(cryptor encryption.Cryptor) {
	e.encoder = format.NewEncoder(cryptor)
}

func (e *EncryptRoutes) SetClock(c clock.Clock)    { e.clock = c }
func (e *EncryptRoutes) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *EncryptRoutes) Up(tx *sql.Tx, logger lager.Logger) error {
	logger = logger.Session("encrypt-route-column")
	logger.Info("starting")
	defer logger.Info("completed")

	query := "SELECT process_guid, routes FROM desired_lrps"

	rows, err := tx.Query(query)
	if err != nil {
		logger.Error("failed-query", err)
		return err
	}

	routeDataMap := map[string][]byte{}

	var processGuid string
	var routeData []byte

	if rows.Err() != nil {
		logger.Error("failed-fetching-row", rows.Err())
		return rows.Err()
	}

	for rows.Next() {
		err := rows.Scan(&processGuid, &routeData)
		if err != nil {
			logger.Error("failed-reading-row", err)
			continue
		}
		routeDataMap[processGuid] = routeData
	}
	err = rows.Close()
	if err != nil {
		logger.Error("failed-to-close-row", err)
	}

	for pGuid, rData := range routeDataMap {
		encodedData, err := e.encoder.Encode(rData)
		if err != nil {
			logger.Error("failed-encrypting-routes", err)
			return models.ErrBadRequest
		}

		bindings := make([]interface{}, 0, 3)
		updateQuery := "UPDATE desired_lrps SET routes = ? WHERE process_guid = ?"
		bindings = append(bindings, encodedData)
		bindings = append(bindings, pGuid)
		_, err = tx.Exec(helpers.RebindForFlavor(updateQuery, e.dbFlavor), bindings...)
		if err != nil {
			logger.Error("failed-updating-desired-lrp-record", err)
			return models.ErrBadRequest
		}
	}

	return nil
}
