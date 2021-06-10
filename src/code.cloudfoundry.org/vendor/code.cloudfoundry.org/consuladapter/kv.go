package consuladapter

import "github.com/hashicorp/consul/api"

//go:generate counterfeiter -o fakes/fake_kv.go . KV

type KV interface {
	Get(key string, q *api.QueryOptions) (*api.KVPair, *api.QueryMeta, error)
	List(prefix string, q *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error)
	Put(p *api.KVPair, q *api.WriteOptions) (*api.WriteMeta, error)
	Release(p *api.KVPair, q *api.WriteOptions) (bool, *api.WriteMeta, error)
	DeleteTree(prefix string, w *api.WriteOptions) (*api.WriteMeta, error)
}

type keyValue struct {
	keyValue *api.KV
}

func NewConsulKV(kv *api.KV) KV {
	return &keyValue{keyValue: kv}
}

func (kv *keyValue) Get(key string, q *api.QueryOptions) (*api.KVPair, *api.QueryMeta, error) {
	return kv.keyValue.Get(key, q)
}

func (kv *keyValue) List(prefix string, q *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error) {
	return kv.keyValue.List(prefix, q)
}

func (kv *keyValue) Put(p *api.KVPair, q *api.WriteOptions) (*api.WriteMeta, error) {
	return kv.keyValue.Put(p, q)
}

func (kv *keyValue) Release(p *api.KVPair, q *api.WriteOptions) (bool, *api.WriteMeta, error) {
	return kv.keyValue.Release(p, q)
}

func (kv *keyValue) DeleteTree(prefix string, w *api.WriteOptions) (*api.WriteMeta, error) {
	return kv.keyValue.DeleteTree(prefix, w)
}
