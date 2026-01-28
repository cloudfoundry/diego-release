package commands_test

import (
	"encoding/json"
	"errors"
	"math"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cfdot/commands"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/openzipkin/zipkin-go/model"
)

var _ = Describe("ActualLRPGroupsForGuid", func() {
	Context("ValidateActualLRPGroupsForGuidArgs", func() {
		It("returns process guid", func() {
			processGuid, index, err := commands.ValidateActualLRPGroupsForGuidArgs([]string{"guid"}, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(processGuid).To(Equal("guid"))
			Expect(index).To(Equal(-1))
		})

		Context("when the process guid is not specified", func() {
			It("returns an error", func() {
				_, _, err := commands.ValidateActualLRPGroupsForGuidArgs([]string{}, "1")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the process guid is invalid", func() {
			It("returns an error", func() {
				_, _, err := commands.ValidateActualLRPGroupsForGuidArgs([]string{""}, "1")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when too many arguments are specified", func() {
			It("returns an error", func() {
				_, _, err := commands.ValidateActualLRPGroupsForGuidArgs([]string{"one", "two"}, "1")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when an index is provided", func() {
			It("returns the index as an int", func() {
				processGuid, index, err := commands.ValidateActualLRPGroupsForGuidArgs([]string{"guid"}, "2")
				Expect(err).NotTo(HaveOccurred())
				Expect(processGuid).To(Equal("guid"))
				Expect(index).To(Equal(2))
			})

			Context("when the index is negative", func() {
				It("returns an error", func() {
					_, _, err := commands.ValidateActualLRPGroupsForGuidArgs([]string{"guid"}, "-1")
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when the index is not an integer", func() {
				It("returns an error", func() {
					_, _, err := commands.ValidateActualLRPGroupsForGuidArgs([]string{"guid"}, "1.5")
					Expect(err).To(HaveOccurred())

					_, _, err = commands.ValidateActualLRPGroupsForGuidArgs([]string{"guid"}, "not-a-num")
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})

	Context("ActualLRPGroupsForGuid", func() {
		var (
			stdout, stderr *gbytes.Buffer
			fakeBBSClient  *fake_bbs.FakeClient
			//lint:ignore SA1019 - deprecated model used for testing deprecated functionality
			actualLRPGroups []*models.ActualLRPGroup
		)

		BeforeEach(func() {
			fakeBBSClient = &fake_bbs.FakeClient{}
			stdout = gbytes.NewBuffer()
			stderr = gbytes.NewBuffer()

			//lint:ignore SA1019 - deprecated model used for testing deprecated functionality
			actualLRPGroups = []*models.ActualLRPGroup{
				{
					Instance: &models.ActualLRP{
						CrashCount:  15,
						CrashReason: "I need some JSON",
					},
				},
				{
					Evacuating: &models.ActualLRP{
						CrashCount:  7,
						CrashReason: "I need some more JSON",
					},
				},
			}

			fakeBBSClient.ActualLRPGroupsByProcessGuidReturns(actualLRPGroups, nil)
		})

		It("writes the json representation of the actual lrp groups to stdout", func() {
			err := commands.ActualLRPGroupsForGuid(stdout, stderr, fakeBBSClient, "guid", -math.MaxInt64)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeBBSClient.ActualLRPGroupsByProcessGuidCallCount()).To(Equal(1))
			_, traceID, guid := fakeBBSClient.ActualLRPGroupsByProcessGuidArgsForCall(0)
			Expect(guid).To(Equal("guid"))
			_, err = model.TraceIDFromHex(traceID)
			Expect(err).NotTo(HaveOccurred())

			expectedOutput := ""
			for _, group := range actualLRPGroups {
				d, err := json.Marshal(group)
				Expect(err).NotTo(HaveOccurred())
				expectedOutput += string(d) + "\n"
			}

			Expect(string(stdout.Contents())).To(Equal(expectedOutput))
		})

		Context("when fetching actual lrp groups fails", func() {
			BeforeEach(func() {
				fakeBBSClient.ActualLRPGroupsByProcessGuidReturns(nil, errors.New("i-failed"))
			})

			It("returns the error", func() {
				err := commands.ActualLRPGroupsForGuid(stdout, stderr, fakeBBSClient, "guid", -math.MaxInt64)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when an index is specified", func() {
			//lint:ignore SA1019 - deprecated model used for testing deprecated functionality
			var actualLRPGroup *models.ActualLRPGroup

			BeforeEach(func() {
				//lint:ignore SA1019 - deprecated model used for testing deprecated functionality
				actualLRPGroup = &models.ActualLRPGroup{
					Instance: &models.ActualLRP{
						CrashCount:  15,
						CrashReason: "I need some JSON",
					},
				}

				fakeBBSClient.ActualLRPGroupByProcessGuidAndIndexReturns(actualLRPGroup, nil)
			})

			It("writes the json representation of the actual lrp group to stdout", func() {
				err := commands.ActualLRPGroupsForGuid(stdout, stderr, fakeBBSClient, "guid", 2)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBBSClient.ActualLRPGroupByProcessGuidAndIndexCallCount()).To(Equal(1))
				_, traceID, guid, index := fakeBBSClient.ActualLRPGroupByProcessGuidAndIndexArgsForCall(0)
				Expect(guid).To(Equal("guid"))
				Expect(index).To(Equal(2))
				_, err = model.TraceIDFromHex(traceID)
				Expect(err).NotTo(HaveOccurred())

				jsonData, err := json.Marshal(actualLRPGroup)
				Expect(err).NotTo(HaveOccurred())

				Expect(stdout).To(gbytes.Say(string(jsonData)))
			})

			Context("when fetching the actual lrp group fails", func() {
				BeforeEach(func() {
					fakeBBSClient.ActualLRPGroupByProcessGuidAndIndexReturns(nil, errors.New("i-failed"))
				})

				It("returns the error", func() {
					err := commands.ActualLRPGroupsForGuid(stdout, stderr, fakeBBSClient, "guid", 2)
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})
