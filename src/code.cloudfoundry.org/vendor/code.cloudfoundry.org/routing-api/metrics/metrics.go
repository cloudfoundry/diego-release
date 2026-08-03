package metrics

import (
	"errors"
	"os"
	"time"

	"sync/atomic"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/routing-api/db"
)

const (
	TotalHttpSubscriptions = "total_http_subscriptions"
	TotalHttpRoutes        = "total_http_routes"
	TotalTcpSubscriptions  = "total_tcp_subscriptions"
	TotalTcpRoutes         = "total_tcp_routes"
	TotalTokenErrors       = "total_token_errors"
	KeyRefreshEvents       = "key_refresh_events"
)

type PartialStatsdClient interface {
	GaugeDelta(stat string, value int64, rate float32) error
	Gauge(stat string, value int64, rate float32) error
}

type MetricsReporter struct {
	db     db.DB
	stats  PartialStatsdClient
	ticker *time.Ticker
	logger lager.Logger
}

var (
	totalTokenErrors          int64
	totalKeyRefreshEventCount int64
)

func NewMetricsReporter(database db.DB, stats PartialStatsdClient, ticker *time.Ticker, logger lager.Logger) *MetricsReporter {
	return &MetricsReporter{db: database, stats: stats, ticker: ticker, logger: logger}
}

func (r *MetricsReporter) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	httpEventChan, httpErrChan, _ := r.db.WatchChanges(db.HTTP_WATCH)
	tcpEventChan, tcpErrChan, _ := r.db.WatchChanges(db.TCP_WATCH)
	close(ready)
	ready = nil

	err := r.stats.Gauge(TotalHttpSubscriptions, 0, 1.0)
	if err != nil {
		r.logger.Info("error-streaming-totalhttpsubscriptions-metrics", lager.Data{"error": err})
	}
	err = r.stats.Gauge(TotalTcpSubscriptions, 0, 1.0)
	if err != nil {
		r.logger.Info("error-streaming-totaltcpsubscriptions-metrics", lager.Data{"error": err})
	}

	for {
		select {
		case event := <-httpEventChan:
			statsDelta := getStatsEventType(event)
			err = r.stats.GaugeDelta(TotalHttpRoutes, statsDelta, 1.0)
			if err != nil {
				r.logger.Info("error-streaming-totalhttpsubscriptions-metrics", lager.Data{"error": err})
			}
		case event := <-tcpEventChan:
			statsDelta := getStatsEventType(event)
			err = r.stats.GaugeDelta(TotalTcpRoutes, statsDelta, 1.0)
			if err != nil {
				r.logger.Info("error-streaming-totaltcpsubscriptions-metrics", lager.Data{"error": err})
			}
		case <-r.ticker.C:
			var errs []error
			err = r.stats.Gauge(TotalHttpRoutes, r.getTotalRoutes(), 1.0)
			errs = append(errs, err)
			err = r.stats.GaugeDelta(TotalHttpSubscriptions, 0, 1.0)
			errs = append(errs, err)
			err = r.stats.Gauge(TotalTcpRoutes, r.getTotalTcpRoutes(), 1.0)
			errs = append(errs, err)
			err = r.stats.GaugeDelta(TotalTcpSubscriptions, 0, 1.0)
			errs = append(errs, err)
			err = r.stats.Gauge(TotalTokenErrors, GetTokenErrors(), 1.0)
			errs = append(errs, err)
			err = r.stats.Gauge(KeyRefreshEvents, GetKeyVerificationRefreshCount(), 1.0)
			errs = append(errs, err)
			if len(errs) > 0 {
				r.logger.Info("error-emitting-metrics", lager.Data{"error": errors.Join(errs...)})
			}
		case <-signals:
			return nil
		case err := <-httpErrChan:
			return err
		case err := <-tcpErrChan:
			return err
		}
	}
}

func (r MetricsReporter) getTotalRoutes() int64 {
	routes, _ := r.db.ReadRoutes()
	return int64(len(routes))
}

func (r MetricsReporter) getTotalTcpRoutes() int64 {
	routes, _ := r.db.ReadTcpRouteMappings()
	return int64(len(routes))
}

func getStatsEventType(event db.Event) int64 {
	if event.Type == db.CreateEvent {
		return 1
	} else if event.Type == db.ExpireEvent || event.Type == db.DeleteEvent {
		return -1
	} else {
		return 0
	}
}

func GetTokenErrors() int64 {
	return atomic.LoadInt64(&totalTokenErrors)
}

func IncrementTokenError() {
	atomic.AddInt64(&totalTokenErrors, 1)
}

func GetKeyVerificationRefreshCount() int64 {
	return atomic.LoadInt64(&totalKeyRefreshEventCount)
}

func IncrementKeyVerificationRefreshCount() {
	atomic.AddInt64(&totalKeyRefreshEventCount, 1)
}
