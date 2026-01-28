package handlers

import (
	"net/http"

	"code.cloudfoundry.org/fileserver"
	"code.cloudfoundry.org/fileserver/handlers/static"
	"code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/rata"
)

func New(staticDirectory string, logger lager.Logger) (http.Handler, error) {
	staticRoute, err := fileserver.Routes.CreatePathForRoute(fileserver.StaticRoute, nil)
	if err != nil {
		return nil, err
	}

	return rata.NewRouter(fileserver.Routes, rata.Handlers{
		fileserver.StaticRoute: static.New(staticDirectory, staticRoute, logger),
	})
}
