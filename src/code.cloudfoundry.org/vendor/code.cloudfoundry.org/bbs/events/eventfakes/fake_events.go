package eventfakes

import "errors"

type FakeEvent struct{ Token string }

func (FakeEvent) EventType() string          { return "fake" }
func (FakeEvent) Key() string                { return "fake" }
func (FakeEvent) ProtoMessage()              {}
func (FakeEvent) Reset()                     {}
func (FakeEvent) String() string             { return "fake" }
func (e FakeEvent) Marshal() ([]byte, error) { return []byte(e.Token), nil }

type UnmarshalableEvent struct{ Fn func() }

func (UnmarshalableEvent) EventType() string        { return "unmarshalable" }
func (UnmarshalableEvent) Key() string              { return "unmarshalable" }
func (UnmarshalableEvent) ProtoMessage()            {}
func (UnmarshalableEvent) Reset()                   {}
func (UnmarshalableEvent) String() string           { return "unmarshalable" }
func (UnmarshalableEvent) Marshal() ([]byte, error) { return nil, errors.New("no workie") }
