package consuladapter

import "github.com/hashicorp/consul/api"

//go:generate counterfeiter -o fakes/fake_session.go . Session

type Session interface {
	Create(se *api.SessionEntry, q *api.WriteOptions) (string, *api.WriteMeta, error)
	CreateNoChecks(se *api.SessionEntry, q *api.WriteOptions) (string, *api.WriteMeta, error)
	Destroy(id string, q *api.WriteOptions) (*api.WriteMeta, error)
	Info(id string, q *api.QueryOptions) (*api.SessionEntry, *api.QueryMeta, error)
	List(q *api.QueryOptions) ([]*api.SessionEntry, *api.QueryMeta, error)
	Node(node string, q *api.QueryOptions) ([]*api.SessionEntry, *api.QueryMeta, error)
	Renew(id string, q *api.WriteOptions) (*api.SessionEntry, *api.WriteMeta, error)
	RenewPeriodic(initialTTL string, id string, q *api.WriteOptions, doneCh chan struct{}) error
}

type session struct {
	session *api.Session
}

func NewConsulSession(s *api.Session) Session {
	return &session{session: s}
}

func (s *session) Create(se *api.SessionEntry, q *api.WriteOptions) (string, *api.WriteMeta, error) {
	return s.session.Create(se, q)
}

func (s *session) CreateNoChecks(se *api.SessionEntry, q *api.WriteOptions) (string, *api.WriteMeta, error) {
	return s.session.CreateNoChecks(se, q)
}

func (s *session) Destroy(id string, q *api.WriteOptions) (*api.WriteMeta, error) {
	return s.session.Destroy(id, q)
}

func (s *session) Info(id string, q *api.QueryOptions) (*api.SessionEntry, *api.QueryMeta, error) {
	return s.session.Info(id, q)
}

func (s *session) List(q *api.QueryOptions) ([]*api.SessionEntry, *api.QueryMeta, error) {
	return s.session.List(q)
}

func (s *session) Node(node string, q *api.QueryOptions) ([]*api.SessionEntry, *api.QueryMeta, error) {
	return s.session.Node(node, q)
}

func (s *session) Renew(id string, q *api.WriteOptions) (*api.SessionEntry, *api.WriteMeta, error) {
	return s.session.Renew(id, q)
}

func (s *session) RenewPeriodic(initialTTL string, id string, q *api.WriteOptions, doneCh chan struct{}) error {
	return s.session.RenewPeriodic(initialTTL, id, q, doneCh)
}
