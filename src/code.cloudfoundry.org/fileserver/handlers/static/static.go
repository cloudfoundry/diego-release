package static

import (
	"net/http"

	"code.cloudfoundry.org/lager/v3"
)

func New(dir, pathPrefix string, logger lager.Logger) http.Handler {
	fileServer := NewFileServer(dir)
	stripped := http.StripPrefix(pathPrefix, fileServer)
	return loggingHandler{
		logger:          logger,
		originalHandler: stripped,
	}
}
