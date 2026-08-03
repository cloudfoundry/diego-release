package test_helpers

import (
	"code.cloudfoundry.org/bbs/models"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

//lint:ignore SA1019 - deprecated model used for testing deprecated functionality
func MatchActualLRPGroup(expected *models.ActualLRPGroup) types.GomegaMatcher {
	//lint:ignore SA1019 - deprecated model used for testing deprecated functionality
	removeUntestedLRPFields := func(lrp *models.ActualLRPGroup) *models.ActualLRPGroup {
		newLRP := *lrp

		if newLRP.Instance != nil {
			newLRP.Instance.Since = 0
			newLRP.Instance.ModificationTag = models.ModificationTag{}
		}

		if newLRP.Evacuating != nil {
			newLRP.Evacuating.Since = 0
			newLRP.Evacuating.ModificationTag = models.ModificationTag{}
		}
		return &newLRP
	}

	expected = removeUntestedLRPFields(expected)
	return gomega.WithTransform(removeUntestedLRPFields, gomega.Equal(expected))
}
