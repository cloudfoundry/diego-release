package unregistration

import "code.cloudfoundry.org/route-emitter/routingtable"

type Message struct {
	RegistryMessage routingtable.RegistryMessage
	SentCount       int
}
