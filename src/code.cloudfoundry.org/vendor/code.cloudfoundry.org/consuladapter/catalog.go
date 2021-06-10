package consuladapter

import "github.com/hashicorp/consul/api"

//go:generate counterfeiter -o fakes/fake_catalog.go . Catalog

type Catalog interface {
	Nodes(q *api.QueryOptions) ([]*api.Node, *api.QueryMeta, error)
}

type catalog struct {
	catalog *api.Catalog
}

func NewConsulCatalog(c *api.Catalog) Catalog {
	return &catalog{catalog: c}
}

func (c *catalog) Nodes(q *api.QueryOptions) ([]*api.Node, *api.QueryMeta, error) {
	return c.catalog.Nodes(q)
}
