package fakes

type FakeClientComponents struct {
	Agent   *FakeAgent
	KV      *FakeKV
	Session *FakeSession
	Catalog *FakeCatalog
}

func NewFakeClient() (*FakeClient, *FakeClientComponents) {
	client := &FakeClient{}

	agent := &FakeAgent{}
	kv := &FakeKV{}
	session := &FakeSession{}
	catalog := &FakeCatalog{}

	client.AgentReturns(agent)
	client.KVReturns(kv)
	client.SessionReturns(session)
	client.CatalogReturns(catalog)
	return client, &FakeClientComponents{
		Agent:   agent,
		KV:      kv,
		Session: session,
		Catalog: catalog,
	}
}
