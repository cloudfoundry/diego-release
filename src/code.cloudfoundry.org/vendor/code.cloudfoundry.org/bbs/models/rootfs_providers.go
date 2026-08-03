package models

// RootFS provider types moved here from code.cloudfoundry.org/rep
// to break the bbs <-> rep module cycle.

import (
	"encoding/json"
	"net/url"
)

type RootFSProvider interface {
	Type() RootFSProviderType
	Match(url.URL) bool
}

type RootFSProviderType string

const (
	RootFSProviderTypeArbitrary RootFSProviderType = "arbitrary"
	RootFSProviderTypeFixedSet  RootFSProviderType = "fixed_set"
)

type RootFSProviders map[string]RootFSProvider

func (p RootFSProviders) Copy() RootFSProviders {
	pCopy := RootFSProviders{}
	for key := range p {
		pCopy[key] = p[key]
	}
	return pCopy
}

func (p RootFSProviders) Match(rootFS url.URL) bool {
	provider, ok := p[rootFS.Scheme]
	if !ok {
		return false
	}
	return provider.Match(rootFS)
}

func (providers *RootFSProviders) UnmarshalJSON(payload []byte) error {
	var providerEnvelope map[string]json.RawMessage
	err := json.Unmarshal(payload, &providerEnvelope)
	if err != nil {
		return err
	}

	*providers = RootFSProviders{}

	for key, value := range providerEnvelope {
		provider, err := unmarshalRootFSProvider(value)
		if err != nil {
			return err
		}
		(*providers)[key] = provider
	}

	return nil
}

type rootFSProviderEnvelope struct {
	Type RootFSProviderType `json:"type"`
}

func unmarshalRootFSProvider(payload []byte) (RootFSProvider, error) {
	var envelope rootFSProviderEnvelope
	err := json.Unmarshal(payload, &envelope)
	if err != nil {
		return nil, err
	}

	switch envelope.Type {
	case RootFSProviderTypeArbitrary:
		return ArbitraryRootFSProvider{}, nil
	case RootFSProviderTypeFixedSet:
		var provider FixedSetRootFSProvider
		err := provider.UnmarshalJSON(payload)
		return provider, err
	}

	return nil, nil
}

type ArbitraryRootFSProvider struct{}

func (ArbitraryRootFSProvider) Type() RootFSProviderType { return RootFSProviderTypeArbitrary }

func (ArbitraryRootFSProvider) Match(url.URL) bool { return true }

func (provider ArbitraryRootFSProvider) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{"type": string(provider.Type())})
}

type FixedSetRootFSProvider struct {
	FixedSet StringSet
}

func NewFixedSetRootFSProvider(rootfses ...string) FixedSetRootFSProvider {
	return FixedSetRootFSProvider{
		FixedSet: NewStringSet(rootfses...),
	}
}

func (FixedSetRootFSProvider) Type() RootFSProviderType { return RootFSProviderTypeFixedSet }

func (provider FixedSetRootFSProvider) Match(rootfs url.URL) bool {
	return provider.FixedSet.Contains(rootfs.Opaque)
}

func (provider FixedSetRootFSProvider) MarshalJSON() ([]byte, error) {
	setPayload, err := json.Marshal(provider.FixedSet)
	if err != nil {
		return nil, err
	}

	typePayload, err := json.Marshal(provider.Type())
	if err != nil {
		return nil, err
	}

	setValue := json.RawMessage(setPayload)
	typeValue := json.RawMessage(typePayload)

	return json.Marshal(map[string]*json.RawMessage{
		"type": &typeValue,
		"set":  &setValue,
	})
}

func (provider *FixedSetRootFSProvider) UnmarshalJSON(payload []byte) error {
	type fixed struct {
		Set StringSet `json:"set"`
	}

	var f fixed
	err := json.Unmarshal(payload, &f)
	if err != nil {
		return err
	}

	provider.FixedSet = f.Set
	return nil
}

type StringSet map[string]struct{}

func NewStringSet(entries ...string) StringSet {
	set := StringSet{}
	for _, entry := range entries {
		set[entry] = struct{}{}
	}
	return set
}

func (set StringSet) Contains(candidate string) bool {
	_, ok := set[candidate]
	return ok
}
