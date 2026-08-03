package routingtable

type MessagesToEmit struct {
	RegistrationMessages           []RegistryMessage
	UnregistrationMessages         []RegistryMessage
	InternalRegistrationMessages   []RegistryMessage
	InternalUnregistrationMessages []RegistryMessage
}

func (m MessagesToEmit) Merge(o MessagesToEmit) MessagesToEmit {
	return MessagesToEmit{
		RegistrationMessages:           append(m.RegistrationMessages, o.RegistrationMessages...),
		UnregistrationMessages:         append(m.UnregistrationMessages, o.UnregistrationMessages...),
		InternalRegistrationMessages:   append(m.InternalRegistrationMessages, o.InternalRegistrationMessages...),
		InternalUnregistrationMessages: append(m.InternalUnregistrationMessages, o.InternalUnregistrationMessages...),
	}
}

func (m MessagesToEmit) RouteRegistrationCount() uint64 {
	return routeCount(m.RegistrationMessages)
}

func (m MessagesToEmit) RouteUnregistrationCount() uint64 {
	return routeCount(m.UnregistrationMessages)
}

func (m MessagesToEmit) InternalRouteRegistrationCount() uint64 {
	return routeCount(m.InternalRegistrationMessages)
}

func (m MessagesToEmit) InternalRouteUnregistrationCount() uint64 {
	return routeCount(m.InternalUnregistrationMessages)
}

func routeCount(messages []RegistryMessage) uint64 {
	var count uint64
	for _, message := range messages {
		count += uint64(len(message.URIs))
	}
	return count
}
