package diegonats

import (
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	uuid "github.com/nu7hatch/gouuid"
)

type FakeNATSClient struct {
	subscriptions        map[string]map[*nats.Subscription]nats.MsgHandler
	publishedMessages    map[string][]*nats.Msg
	unsubscriptions      []*nats.Subscription
	unsubscribedSubjects []string

	connectError     error
	unsubscribeError error

	whenSubscribing map[string]func(nats.MsgHandler) error
	whenPublishing  map[string]func(*nats.Msg) error

	onPing       func() bool
	pingResponse bool
	pingInterval time.Duration

	sync.RWMutex
}

func NewFakeClient() *FakeNATSClient {
	fake := &FakeNATSClient{}
	fake.Reset()
	return fake
}

func (f *FakeNATSClient) Reset() {
	f.Lock()
	defer f.Unlock()

	f.publishedMessages = map[string][]*nats.Msg{}
	f.subscriptions = map[string]map[*nats.Subscription]nats.MsgHandler{}
	f.unsubscriptions = []*nats.Subscription{}
	f.unsubscribedSubjects = []string{}

	f.connectError = nil
	f.unsubscribeError = nil
	f.pingInterval = -1

	f.whenSubscribing = map[string]func(nats.MsgHandler) error{}
	f.whenPublishing = map[string]func(*nats.Msg) error{}

	f.pingResponse = true
}

func (f *FakeNATSClient) Connect(urls []string) (chan struct{}, error) {
	f.Lock()
	defer f.Unlock()

	return nil, f.connectError
}

func (f *FakeNATSClient) SetPingInterval(interval time.Duration) {
	f.Lock()
	defer f.Unlock()

	f.pingInterval = interval
}

func (f *FakeNATSClient) Close() {
	f.Lock()
	defer f.Unlock()
}

func (f *FakeNATSClient) OnPing(onPingCallback func() bool) {
	f.Lock()
	f.onPing = onPingCallback
	f.Unlock()
}

func (f *FakeNATSClient) Ping() bool {
	f.RLock()
	onPing := f.onPing
	response := f.pingResponse
	f.RUnlock()

	if onPing != nil {
		return onPing()
	}

	return response
}

func (f *FakeNATSClient) Publish(subject string, payload []byte) error {
	return f.PublishRequest(subject, "", payload)
}

func (f *FakeNATSClient) PublishRequest(subject, reply string, payload []byte) error {
	f.RLock()

	injectedCallback, injected := f.whenPublishing[subject]

	callbacks := []nats.MsgHandler{}

	if subs := f.subscriptions[subject]; subs != nil {
		callbacks = make([]nats.MsgHandler, 0)
		for _, cb := range subs {
			callbacks = append(callbacks, cb)
		}
	}

	f.RUnlock()

	message := &nats.Msg{
		Subject: subject,
		Reply:   reply,
		Data:    payload,
	}

	if injected {
		err := injectedCallback(message)
		if err != nil {
			return err
		}
	}

	f.Lock()
	f.publishedMessages[subject] = append(f.publishedMessages[subject], message)
	f.Unlock()

	for _, cb := range callbacks {
		cb(message)
	}

	return nil
}

func (f *FakeNATSClient) Request(subj string, data []byte, timeout time.Duration) (m *nats.Msg, err error) {
	guid, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	respChan := make(chan *nats.Msg, 1)
	_, err = f.Subscribe(guid.String(), func(msg *nats.Msg) {
		respChan <- msg
	})
	if err != nil {
		return nil, err
	}

	err = f.PublishRequest(subj, guid.String(), data)
	if err != nil {
		return nil, err
	}

	select {
	case msg := <-respChan:
		return msg, nil
	case <-time.After(timeout):
		return nil, nats.ErrTimeout
	}
}

func (f *FakeNATSClient) Subscribe(subject string, callback nats.MsgHandler) (*nats.Subscription, error) {
	return f.QueueSubscribe(subject, "", callback)
}

func (f *FakeNATSClient) QueueSubscribe(subject, queue string, callback nats.MsgHandler) (*nats.Subscription, error) {
	f.RLock()

	injectedCallback, injected := f.whenSubscribing[subject]

	f.RUnlock()

	subscription := &nats.Subscription{
		Subject: subject,
		Queue:   queue,
	}

	if injected {
		err := injectedCallback(callback)
		if err != nil {
			return nil, err
		}
	}

	f.addSubscriptionHandler(subscription, callback)

	return subscription, nil
}

func (f *FakeNATSClient) Unsubscribe(subscription *nats.Subscription) error {
	f.Lock()
	defer f.Unlock()

	if f.unsubscribeError != nil {
		return f.unsubscribeError
	}

	f.unsubscriptions = append(f.unsubscriptions, subscription)

	return nil
}

func (f *FakeNATSClient) addSubscriptionHandler(subscription *nats.Subscription, handler nats.MsgHandler) {
	f.Lock()
	subs := f.subscriptions[subscription.Subject]
	if subs == nil {
		subs = make(map[*nats.Subscription]nats.MsgHandler)
		f.subscriptions[subscription.Subject] = subs
	}
	subs[subscription] = handler
	f.Unlock()
}

func (f *FakeNATSClient) WhenSubscribing(subject string, callback func(nats.MsgHandler) error) {
	f.Lock()
	f.whenSubscribing[subject] = callback
	f.Unlock()
}

func (f *FakeNATSClient) SubjectCallbacks(subject string) []nats.MsgHandler {
	f.RLock()
	values := make([]nats.MsgHandler, 0)
	for _, v := range f.subscriptions[subject] {
		values = append(values, v)
	}
	f.RUnlock()

	return values
}
func (f *FakeNATSClient) Subscriptions(subject string) []*nats.Subscription {
	f.RLock()

	keys := make([]*nats.Subscription, 0)
	for k := range f.subscriptions[subject] {
		keys = append(keys, k)
	}
	f.RUnlock()

	return keys
}

func (f *FakeNATSClient) SubscriptionCount() int {
	cnt := 0
	f.RLock()
	for _, subs := range f.subscriptions {
		cnt += len(subs)
	}
	f.RUnlock()

	return cnt
}

func (f *FakeNATSClient) WhenPublishing(subject string, callback func(*nats.Msg) error) {
	f.Lock()
	defer f.Unlock()

	f.whenPublishing[subject] = callback
}

func (f *FakeNATSClient) PublishedMessages(subject string) []*nats.Msg {
	f.RLock()
	defer f.RUnlock()

	return f.publishedMessages[subject]
}

func (f *FakeNATSClient) PublishedMessageCount() int {
	f.RLock()
	defer f.RUnlock()

	return len(f.publishedMessages)
}
