package middleware

import (
	"net/http"
	"slices"
	"time"

	"code.cloudfoundry.org/bbs/cmd/bbs/config"
	"code.cloudfoundry.org/lager/v3"
)

type LoggableHandlerFunc func(logger lager.Logger, w http.ResponseWriter, r *http.Request)

//go:generate counterfeiter -generate

//counterfeiter:generate -o fakes/fake_emitter.go . Emitter
type Emitter interface {
	IncrementRequestCounter(delta int, route string)
	UpdateLatency(latency time.Duration, route string)
}

func LogWrap(logger, accessLogger lager.Logger, loggableHandlerFunc LoggableHandlerFunc) http.HandlerFunc {
	lagerDataFromReq := func(r *http.Request) lager.Data {
		lagerData := lager.Data{
			"method":      r.Method,
			"remote_addr": r.RemoteAddr,
			"request":     r.URL.String(),
		}

		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			indexLeafCertConnectionIsVerifiedAgainst := 0 // see also https://github.com/golang/go/blob/e929fb78e47dc191a402d34ca949d2e0c67e31b8/src/crypto/tls/common.go#L281-L282
			cert := r.TLS.PeerCertificates[indexLeafCertConnectionIsVerifiedAgainst]

			lagerData["peer_cert_subject_common_name"] = cert.Subject.CommonName
			lagerData["peer_cert_subject_organizational_unit"] = cert.Subject.OrganizationalUnit
			lagerData["peer_cert_subject_organization"] = cert.Subject.Organization

			lagerData["peer_cert_issuer_common_name"] = cert.Issuer.CommonName
			lagerData["peer_cert_issuer_organizational_unit"] = cert.Issuer.OrganizationalUnit
			lagerData["peer_cert_issuer_organization"] = cert.Issuer.Organization
		}

		return lagerData
	}

	if accessLogger != nil {
		return func(w http.ResponseWriter, r *http.Request) {
			requestLog := logger.Session("request")
			requestAccessLogger := accessLogger.Session("request")

			requestAccessLogger.Info("serving", lagerDataFromReq(r))
			requestLog.Debug("serving", lagerDataFromReq(r))

			start := time.Now()
			defer requestLog.Debug("done", lagerDataFromReq(r))
			defer func() {
				requestTime := time.Since(start)
				lagerData := lagerDataFromReq(r)
				lagerData["duration"] = requestTime
				requestAccessLogger.Info("done", lagerData)
			}()
			loggableHandlerFunc(requestLog, w, r)
		}
	} else {
		return func(w http.ResponseWriter, r *http.Request) {
			requestLog := logger.Session("request")

			requestLog.Debug("serving", lagerDataFromReq(r))
			defer requestLog.Debug("done", lagerDataFromReq(r))

			loggableHandlerFunc(requestLog, w, r)
		}
	}
}

type metadata struct {
	latency time.Duration
}

type handlerWithMetadata func(w http.ResponseWriter, r *http.Request) metadata

func initHandlerWithMetadata(f http.Handler) handlerWithMetadata {
	return func(w http.ResponseWriter, r *http.Request) metadata {
		f.ServeHTTP(w, r)
		return metadata{}
	}
}

func stripMetadata(f handlerWithMetadata) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f(w, r)
	}
}

func recordLatency(f handlerWithMetadata) handlerWithMetadata {
	return func(w http.ResponseWriter, r *http.Request) metadata {
		startTime := time.Now()
		metadata := f(w, r)
		metadata.latency = time.Since(startTime)
		return metadata
	}
}

func updateLatency(f handlerWithMetadata, emitter Emitter, route string) handlerWithMetadata {
	return func(w http.ResponseWriter, r *http.Request) metadata {
		metadata := f(w, r)
		emitter.UpdateLatency(metadata.latency, route)
		return metadata
	}
}

func incrementRequestCount(f handlerWithMetadata, emitter Emitter, route string) handlerWithMetadata {
	return func(w http.ResponseWriter, r *http.Request) metadata {
		metadata := f(w, r)
		emitter.IncrementRequestCounter(1, route)
		return metadata
	}
}

func RecordLatency(f http.Handler, emitter Emitter) http.HandlerFunc {
	handlerMeta := initHandlerWithMetadata(f)
	handlerMeta = recordLatency(handlerMeta)
	handlerMeta = updateLatency(handlerMeta, emitter, "")
	return stripMetadata(handlerMeta)
}

func RecordRequestCount(f http.Handler, emitter Emitter) http.HandlerFunc {
	handlerMeta := initHandlerWithMetadata(f)
	handlerMeta = incrementRequestCount(handlerMeta, emitter, "")
	return stripMetadata(handlerMeta)
}

func RecordMetrics(f http.HandlerFunc, emitter Emitter, advancedMetricsConfig config.AdvancedMetrics, calledRoute string) http.HandlerFunc {
	// Record Default Metrics
	handlerMeta := initHandlerWithMetadata(f)
	handlerMeta = recordLatency(handlerMeta)

	handlerMeta = updateLatency(handlerMeta, emitter, "")
	handlerMeta = incrementRequestCount(handlerMeta, emitter, "")

	// Record Advanced Metrics
	if advancedMetricsConfig.Enabled {
		if calledRoute == "" {
			panic("calledRoute is required for advanced metrics")
		}

		handlerMeta = recordAdvancedMetrics(handlerMeta, emitter, advancedMetricsConfig, calledRoute)
	}

	return stripMetadata(handlerMeta)
}

func recordAdvancedMetrics(handlerMeta handlerWithMetadata, emitter Emitter, advancedMetricsConfig config.AdvancedMetrics, calledRoute string) handlerWithMetadata {
	isRouteFound := slices.Contains(advancedMetricsConfig.RouteConfig.RequestCountRoutes, calledRoute)
	if isRouteFound {
		handlerMeta = incrementRequestCount(handlerMeta, emitter, calledRoute)
	}

	isRouteFound = slices.Contains(advancedMetricsConfig.RouteConfig.RequestLatencyRoutes, calledRoute)
	if isRouteFound {
		handlerMeta = updateLatency(handlerMeta, emitter, calledRoute)
	}

	return handlerMeta
}
