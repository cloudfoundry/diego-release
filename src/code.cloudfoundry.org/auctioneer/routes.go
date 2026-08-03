package auctioneer

import "github.com/tedsuo/rata"

const (
	CreateTaskAuctionsRoute = "CreateTaskAuctions"
	CreateLRPAuctionsRoute  = "CreateLRPAuctions"
)

var Routes = rata.Routes{
	{Path: "/v1/tasks", Method: "POST", Name: CreateTaskAuctionsRoute},
	{Path: "/v1/lrps", Method: "POST", Name: CreateLRPAuctionsRoute},
}
