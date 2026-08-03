package trace

import (
	"context"
	"net/http"
	"strings"

	"code.cloudfoundry.org/lager/v3"
	"github.com/openzipkin/zipkin-go/idgenerator"
	"github.com/openzipkin/zipkin-go/model"
)

const (
	RequestIdHeader = "X-Vcap-Request-Id"
)

type RequestIdHeaderCtxKeyType struct{}

var RequestIdHeaderCtxKey = RequestIdHeaderCtxKeyType{}

func ContextWithRequestId(req *http.Request) context.Context {
	return context.WithValue(req.Context(), RequestIdHeaderCtxKey, RequestIdFromRequest(req))
}

func RequestIdFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(RequestIdHeaderCtxKey).(string); ok {
		return val
	}

	return ""
}

func RequestIdFromRequest(req *http.Request) string {
	return req.Header.Get(RequestIdHeader)
}

func LoggerWithTraceInfo(logger lager.Logger, traceIDStr string) lager.Logger {
	if traceIDStr == "" {
		return logger.WithData(nil)
	}
	traceHex := strings.Replace(traceIDStr, "-", "", -1)
	traceID, err := model.TraceIDFromHex(traceHex)
	if err != nil {
		return logger.WithData(nil)
	}

	spanID := idgenerator.NewRandom128().SpanID(model.TraceID{})
	return logger.WithData(lager.Data{"trace-id": traceID.String(), "span-id": spanID.String()})
}

func GenerateTraceID() string {
	return idgenerator.NewRandom128().TraceID().String()
}
