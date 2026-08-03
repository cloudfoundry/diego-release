package sqldb

import (
	"context"
	"math"
	"time"

	"code.cloudfoundry.org/diego-db-helpers/sqldb/helpers"
	"code.cloudfoundry.org/lager/v3"
)

func (db *SQLDB) FreshDomains(ctx context.Context, logger lager.Logger) ([]string, error) {
	logger = logger.Session("db-fresh-domains")
	logger.Debug("starting")
	defer logger.Debug("complete")

	var domainNames []string

	err := db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		expireTime := db.clock.Now().Round(time.Second)
		domains, err := db.domains(ctx, logger, tx, expireTime)
		if err != nil {
			return err
		}

		domainNames = nil
		for _, d := range domains {
			domainNames = append(domainNames, d.name)
		}
		return nil
	})

	return domainNames, err
}

type domain struct {
	name      string
	expiresAt time.Time
}

func (db *SQLDB) domains(ctx context.Context, logger lager.Logger, tx helpers.Queryable, expiresAfter time.Time) ([]domain, error) {
	rows, err := db.all(ctx, logger, tx, domainsTable,
		domainColumns, helpers.NoLockRow,
		"expire_time > ?",
		expiresAfter.UnixNano(),
	)
	if err != nil {
		logger.Error("failed-query", err)
		return nil, err
	}

	defer rows.Close()

	var results []domain

	for rows.Next() {
		var name string
		var expiresAt int64
		err = rows.Scan(&name, &expiresAt)
		if err != nil {
			logger.Error("failed-scan-row", err)
			return nil, err
		}

		results = append(results, domain{name, time.Unix(0, int64(expiresAt))})
	}

	if rows.Err() != nil {
		logger.Error("failed-fetching-row", err)
		return nil, err
	}

	return results, nil
}

func (db *SQLDB) UpsertDomain(ctx context.Context, logger lager.Logger, domain string, ttl uint32) error {
	logger = logger.Session("db-upsert-domain", lager.Data{"domain": domain, "ttl": ttl})
	logger.Debug("starting")
	defer logger.Debug("complete")

	return db.transact(ctx, logger, func(logger lager.Logger, tx helpers.Tx) error {
		expireTime := db.clock.Now().Add(time.Duration(ttl) * time.Second).UnixNano()
		if ttl == 0 {
			expireTime = math.MaxInt64
		}

		ok, err := db.upsert(ctx, logger, tx, domainsTable,
			helpers.SQLAttributes{"domain": domain, "expire_time": expireTime},
			"domain = ?", domain,
		)

		if err != nil {
			logger.Error("failed-inserting-domain", err)
			return err
		}

		if ok {
			logger.Info("added-domain", lager.Data{"domain": domain})
		}

		return nil
	})
}
