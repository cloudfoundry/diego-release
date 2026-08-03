package debugserver

import (
	lager "code.cloudfoundry.org/lager/v3"
	"errors"
	"net/http"
	"strings"
)

// zapLogLevelController is an interface that defines a method to set the minimum log level.
type zapLogLevelController interface {
	SetMinLevel(level lager.LogLevel)
}

// LagerAdapter is an adapter for the ReconfigurableSinkInterface to work with lager.LogLevel.
type LagerAdapter struct {
	Sink ReconfigurableSinkInterface
}

// SetMinLevel sets the minimum log level for the LagerAdapter.
func (l *LagerAdapter) SetMinLevel(level lager.LogLevel) {
	l.Sink.SetMinLevel(level)
}

// normalizeLogLevel returns a single value that represents
// various forms of the same input level. For example:
// "0", "d", "debug", all of these represents debug log level.
func normalizeLogLevel(input string) string {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "0", "d", "debug":
		return "debug"
	case "1", "i", "info":
		return "info"
	case "2", "w", "warn":
		return "warn"
	case "3", "e", "error":
		return "error"
	case "4", "f", "fatal":
		return "fatal"
	default:
		return ""
	}
}

// validateAndNormalize does two things:
// It validates the incoming request is HTTP type, uses POST method and has non-nil level specified.
// It also normalizes the various forms of the same log level type. For ex: 0, d, debug are all same.
func validateAndNormalize(w http.ResponseWriter, r *http.Request, level []byte) (string, error) {
	if r.Method != http.MethodPost {
		return "", errors.New("method not allowed, use POST")
	}

	if r.TLS != nil {
		return "", errors.New("invalid scheme, https is not allowed")
	}

	if len(level) == 0 {
		return "", errors.New("log level cannot be empty")
	}

	input := strings.TrimSpace(string(level))
	normalized := normalizeLogLevel(input)
	if normalized == "" {
		return "", errors.New("invalid log level: " + string(level))
	}

	return normalized, nil
}
