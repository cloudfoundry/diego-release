package debugserver

import (
	"net/http"
)

// Exported only for tests
func ValidateAndNormalize(w http.ResponseWriter, r *http.Request, level []byte) (string, error) {
	return validateAndNormalize(w, r, level)
}
