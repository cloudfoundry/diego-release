package consuladapter

import "github.com/hashicorp/consul/api"

//go:generate counterfeiter -o fakes/fake_agent.go . Agent

type Agent interface {
	Checks() (map[string]*api.AgentCheck, error)
	Services() (map[string]*api.AgentService, error)
	ServiceRegister(service *api.AgentServiceRegistration) error
	ServiceDeregister(serviceID string) error
	PassTTL(checkID, note string) error
	WarnTTL(checkID, note string) error
	FailTTL(checkID, note string) error
	NodeName() (string, error)
	CheckDeregister(checkID string) error
}

type agent struct {
	agent *api.Agent
}

func NewConsulAgent(a *api.Agent) Agent {
	return &agent{agent: a}
}

func (a *agent) Checks() (map[string]*api.AgentCheck, error) {
	return a.agent.Checks()
}

func (a *agent) Services() (map[string]*api.AgentService, error) {
	return a.agent.Services()
}

func (a *agent) ServiceRegister(service *api.AgentServiceRegistration) error {
	return a.agent.ServiceRegister(service)
}

func (a *agent) ServiceDeregister(serviceID string) error {
	return a.agent.ServiceDeregister(serviceID)
}

func (a *agent) CheckDeregister(checkID string) error {
	return a.agent.CheckDeregister(checkID)
}

func (a *agent) PassTTL(checkID, note string) error {
	return a.agent.PassTTL(checkID, note)
}

func (a *agent) WarnTTL(checkID, note string) error {
	return a.agent.WarnTTL(checkID, note)
}

func (a *agent) FailTTL(checkID, note string) error {
	return a.agent.FailTTL(checkID, note)
}

func (a *agent) NodeName() (string, error) {
	return a.agent.NodeName()
}
