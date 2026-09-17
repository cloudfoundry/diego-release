package processes

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

func WindowsEnvFor(bndl goci.Bndl, spec garden.ProcessSpec, _ int) []string {
	return append(bndl.Spec.Process.Env, spec.Env...)
}
