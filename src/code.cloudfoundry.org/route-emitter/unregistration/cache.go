package unregistration

import (
	"sync"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"github.com/mitchellh/hashstructure"
)

//go:generate counterfeiter -o fakes/fake_cache.go . Cache
type Cache interface {
	Add([]routingtable.RegistryMessage) error
	Remove([]routingtable.RegistryMessage) error
	List() []*Message
}

type cache struct {
	messages map[uint64]*Message
	mux      *sync.Mutex
	logger   lager.Logger
}

func NewCache(logger lager.Logger) Cache {
	cacheLogger := logger.Session("unregistration-cache")
	return &cache{
		messages: map[uint64]*Message{},
		mux:      &sync.Mutex{},
		logger:   cacheLogger,
	}
}

func (c *cache) Add(registryMessages []routingtable.RegistryMessage) error {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.logger.Debug("add", lager.Data{"cache": registryMessages})
	for _, registryMessage := range registryMessages {
		registryMessageHash, err := hashstructure.Hash(registryMessage, nil)
		if err != nil {
			return err
		}
		c.messages[registryMessageHash] = &Message{
			RegistryMessage: registryMessage,
		}
	}
	return nil
}

func (c *cache) Remove(registryMessages []routingtable.RegistryMessage) error {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.logger.Debug("remove", lager.Data{"cache": registryMessages})
	for _, registryMessage := range registryMessages {
		registryMessageHash, err := hashstructure.Hash(registryMessage, nil)
		if err != nil {
			return err
		}
		delete(c.messages, registryMessageHash)
	}
	return nil
}

func (c *cache) List() []*Message {
	c.mux.Lock()
	defer c.mux.Unlock()

	list := []*Message{}
	for _, message := range c.messages {
		list = append(list, message)
	}
	return list
}
