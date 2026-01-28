package handlers

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/handlers/middleware"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/rata"
)

const (
	RequestLatencyDuration = "RequestLatency"
	RequestCount           = "RequestCount"
)

func New(logger lager.Logger, runner auctiontypes.AuctionRunner, metronClient loggingclient.IngressClient) http.Handler {
	taskAuctionHandler := logWrap(NewTaskAuctionHandler(runner).Create, logger)
	lrpAuctionHandler := logWrap(NewLRPAuctionHandler(runner).Create, logger)

	emitter := &auctioneerEmitter{
		logger:       logger,
		metronClient: metronClient,
	}

	actions := rata.Handlers{
		auctioneer.CreateTaskAuctionsRoute: middleware.RecordLatency(taskAuctionHandler, emitter),
		auctioneer.CreateLRPAuctionsRoute:  middleware.RecordLatency(lrpAuctionHandler, emitter),
	}

	handler, err := rata.NewRouter(auctioneer.Routes, actions)
	if err != nil {
		panic("unable to create router: " + err.Error())
	}

	return middleware.RecordRequestCount(handler, emitter)
}

func logWrap(loggable func(http.ResponseWriter, *http.Request, lager.Logger), logger lager.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestLog := logger.Session("request", lager.Data{
			"method":  r.Method,
			"request": r.URL.String(),
		})

		requestLog.Info("serving")
		loggable(w, r, requestLog)
		requestLog.Info("done")
	}
}

type auctioneerEmitter struct {
	logger       lager.Logger
	metronClient loggingclient.IngressClient
}

func (e *auctioneerEmitter) IncrementRequestCounter(delta int, route string) {
	err := e.metronClient.IncrementCounter(RequestCount)
	if err != nil {
		e.logger.Error("failed-to-increment-request-counter", err)
	}
}

func (e *auctioneerEmitter) UpdateLatency(latency time.Duration, route string) {
	err := e.metronClient.SendDuration(RequestLatencyDuration, latency)
	if err != nil {
		e.logger.Error("failed-to-send-latency", err)
	}
}
