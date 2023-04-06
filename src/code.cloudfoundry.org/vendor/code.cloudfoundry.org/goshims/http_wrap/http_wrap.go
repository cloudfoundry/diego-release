package http_wrap

import os_http "net/http"

//go:generate counterfeiter -o http_fake/fake_http_client.go . Client

/*
Wraps http client side calls.
*/
type Client interface {
Do(req *os_http.Request) (resp *os_http.Response, err error)
}
