package helpers

import (
	"strings"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"

	. "github.com/onsi/gomega"
)

func filteredActualLRPs(logger lager.Logger, client bbs.InternalClient, processGuid string, filter func(lrp *models.ActualLRP) bool) []models.ActualLRP {
	lrps, err := client.ActualLRPs(logger, "", models.ActualLRPFilter{ProcessGuid: processGuid})
	Expect(err).NotTo(HaveOccurred())

	startedLRPs := make([]models.ActualLRP, 0, len(lrps))
	for _, lrp := range lrps {
		if filter(lrp) {
			startedLRPs = append(startedLRPs, *lrp)
		}
	}

	return startedLRPs
}

func ActiveActualLRPs(logger lager.Logger, client bbs.InternalClient, processGuid string) []models.ActualLRP {
	return filteredActualLRPs(logger, client, processGuid, func(lrp *models.ActualLRP) bool {
		return lrp.State != models.ActualLRPStateUnclaimed
	})
}

func RunningActualLRPs(logger lager.Logger, client bbs.InternalClient, processGuid string) []models.ActualLRP {
	return filteredActualLRPs(logger, client, processGuid, func(lrp *models.ActualLRP) bool {
		return lrp.State == models.ActualLRPStateRunning
	})
}

func TaskStatePoller(logger lager.Logger, client bbs.InternalClient, taskGuid string, task *models.Task) func() models.Task_State {
	return func() models.Task_State {
		rTask, err := client.TaskByGuid(logger, "", taskGuid)
		Expect(err).NotTo(HaveOccurred())

		if task != nil {
			*task = *rTask
		}

		return rTask.State
	}
}

func TaskFailedPoller(logger lager.Logger, client bbs.InternalClient, taskGuid string, task *models.Task) func() bool {
	return func() bool {
		rTask, err := client.TaskByGuid(logger, "", taskGuid)
		Expect(err).NotTo(HaveOccurred())

		if task != nil {
			*task = *rTask
		}

		return rTask.Failed
	}
}

func LRPStatePoller(logger lager.Logger, client bbs.InternalClient, processGuid string, lrp *models.ActualLRP) func() string {
	return func() string {
		var foundLRP *models.ActualLRP

		lrps, err := client.ActualLRPs(logger, "", models.ActualLRPFilter{ProcessGuid: processGuid})
		if err != nil && strings.Contains(err.Error(), "Invalid Response with status code: 404") {
			lrpGroups, err := client.ActualLRPGroupsByProcessGuid(logger, "", processGuid)
			Expect(err).NotTo(HaveOccurred())
			if len(lrpGroups) == 0 {
				return ""
			}
			//lint:ignore SA1019 - inigo needs to process deprecated ActualLRP data until it is removed from BBS
			foundLRP, _, err = lrpGroups[0].Resolve()
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).NotTo(HaveOccurred())
			if len(lrps) == 0 {
				return ""
			}
			foundLRP = lrps[0]
		}
		if lrp != nil {
			*lrp = *foundLRP
		}
		return foundLRP.State
	}
}

func LRPInstanceStatePoller(logger lager.Logger, client bbs.InternalClient, processGuid string, index int, lrp *models.ActualLRP) func() string {
	return func() string {
		i := int32(index)
		var foundLRP *models.ActualLRP
		lrps, err := client.ActualLRPs(logger, "", models.ActualLRPFilter{ProcessGuid: processGuid, Index: &i})
		if err != nil && strings.Contains(err.Error(), "Invalid Response with status code: 404") {
			lrpGroup, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, "", processGuid, index)
			Expect(err).NotTo(HaveOccurred())
			//lint:ignore SA1019 - inigo needs to process deprecated ActualLRP data until it is removed from BBS
			foundLRP, _, err = lrpGroup.Resolve()
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).NotTo(HaveOccurred())
			Expect(lrps).To(HaveLen(1))
			foundLRP = lrps[0]
		}
		if lrp != nil {
			*lrp = *foundLRP
		}
		return foundLRP.State
	}
}
