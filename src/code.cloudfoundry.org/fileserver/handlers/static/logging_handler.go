package static

import (
	"net/http"

	"code.cloudfoundry.org/lager/v3"
)

type loggingHandler struct {
	originalHandler http.Handler
	logger          lager.Logger
}

type responseLogger struct {
	w      http.ResponseWriter
	status int
	size   int
}

func (l *responseLogger) Write(b []byte) (int, error) {
	if l.status == 0 {
		// The status will be StatusOK if WriteHeader has not been called yet
		l.status = http.StatusOK
	}
	size, err := l.w.Write(b)
	l.size += size
	return size, err
}

func (l *responseLogger) WriteHeader(s int) {
	l.w.WriteHeader(s)
	l.status = s
}

func (l *responseLogger) Header() http.Header {
	return l.w.Header()
}

func (h loggingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	resLogger := &responseLogger{w: w}
	h.originalHandler.ServeHTTP(resLogger, req)

	requestLogger := h.logger.Session("static-file")
	requestLogger.Info("response", lager.Data{
		"status": resLogger.status,
		"size":   resLogger.size,
		"method": req.Method,
		"uri":    req.URL.RequestURI(),
	})
}
