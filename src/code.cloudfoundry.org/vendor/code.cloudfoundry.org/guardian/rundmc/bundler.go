package rundmc

import (
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

//counterfeiter:generate . BundlerRule
type BundlerRule interface {
	Apply(bndle goci.Bndl, desiredContainerSpec spec.DesiredContainerSpec) (goci.Bndl, error)
}

type BundleTemplate struct {
	Rules []BundlerRule
}

func (b BundleTemplate) Generate(spec spec.DesiredContainerSpec) (goci.Bndl, error) {
	var bndl goci.Bndl

	for _, rule := range b.Rules {
		var err error
		bndl, err = rule.Apply(bndl, spec)
		if err != nil {
			return goci.Bndl{}, err
		}
	}

	return bndl, nil
}
