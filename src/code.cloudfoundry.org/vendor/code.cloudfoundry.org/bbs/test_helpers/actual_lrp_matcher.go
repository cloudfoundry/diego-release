package test_helpers

import (
	"code.cloudfoundry.org/bbs/models"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

func MatchActualLRP(expected *models.ActualLRP) types.GomegaMatcher {
	removeUntestedLRPFields := func(lrp *models.ActualLRP) *models.ActualLRP {
		newLRP := *lrp

		newLRP.Since = 0
		newLRP.ModificationTag = models.ModificationTag{}
		return &newLRP
	}

	expected = removeUntestedLRPFields(expected)
	return gomega.WithTransform(removeUntestedLRPFields, gomega.Equal(expected))
}
