package helpers

import (
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/models"
)

type RouteRegister struct {
	database db.DB
	route    models.Route
	ticker   *time.Ticker
	logger   lager.Logger
}

func NewRouteRegister(database db.DB, route models.Route, ticker *time.Ticker, logger lager.Logger) *RouteRegister {
	return &RouteRegister{
		database: database,
		route:    route,
		ticker:   ticker,
		logger:   logger,
	}
}

func (r *RouteRegister) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := r.database.SaveRoute(r.route)
	if err != nil {
		return fmt.Errorf("registration error: %s", err.Error())
	}
	close(ready)
	r.logger.Info("starting-registration-loop", lager.Data{"route": r.route})

	for {
		select {
		case <-r.ticker.C:
			err = r.database.SaveRoute(r.route)
			if err != nil {
				r.logger.Error("registration-error", err)
			}
		case <-signals:
			err := r.database.DeleteRoute(r.route)
			if err != nil {
				r.logger.Error("unregistration-error", err)
				return err
			}
			return nil
		}
	}
}
