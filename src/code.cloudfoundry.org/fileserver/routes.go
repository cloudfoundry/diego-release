package fileserver

import "github.com/tedsuo/rata"

const (
	StaticRoute = "Static"
)

var Routes = rata.Routes{
	{Name: StaticRoute, Method: "GET", Path: "/v1/static/"},
}
