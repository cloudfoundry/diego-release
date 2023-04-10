package gardener

import "code.cloudfoundry.org/lager/v3"

type NoopRestorer struct{}

func (n *NoopRestorer) Restore(_ lager.Logger, handles []string) []string {
	return handles
}
