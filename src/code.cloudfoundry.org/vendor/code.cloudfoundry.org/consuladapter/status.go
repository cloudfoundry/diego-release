package consuladapter

import "github.com/hashicorp/consul/api"

//go:generate counterfeiter -o fakes/fake_status.go . Status

type Status interface {
	Leader() (string, error)
	Peers() ([]string, error)
}

type status struct {
	status *api.Status
}

func NewConsulStatus(s *api.Status) Status {
	return &status{status: s}
}

func (s *status) Leader() (string, error) {
	return s.status.Leader()
}

func (s *status) Peers() ([]string, error) {
	return s.status.Peers()
}
