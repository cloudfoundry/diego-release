package vizzini_test

import (
	"net"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	. "code.cloudfoundry.org/vizzini/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Security groups", func() {
	var gorouterLBIP string

	BeforeEach(func() {
		routeToResolve := "www.example.com"

		ips, err := net.LookupIP(routeToResolve)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(ips)).To(BeNumerically(">", 0), "failed to resolve "+routeToResolve)

		ip := ips[0]
		Expect(ip.To4()).NotTo(BeNil(), "failed to resolve "+routeToResolve+" to an IPv4 address")

		gorouterLBIP = ip.String()
	})

	Context("for LRPs", func() {
		var allowedCaller, disallowedCaller *models.DesiredLRP
		var allowedCallerGuid, disallowedCallerGuid string

		BeforeEach(func() {
			allowedCallerGuid, disallowedCallerGuid = NewGuid(), NewGuid()
			allowedCaller, disallowedCaller = DesiredLRPWithGuid(allowedCallerGuid), DesiredLRPWithGuid(disallowedCallerGuid)

			Expect(bbsClient.DesireLRP(logger, traceID, disallowedCaller)).To(Succeed())
			Eventually(ActualGetter(logger, disallowedCallerGuid, 0)).Should(BeActualLRPWithState(disallowedCallerGuid, 0, models.ActualLRPStateRunning))
			Eventually(EndpointCurler("http://" + RouteForGuid(disallowedCallerGuid) + "/env")).Should(Equal(http.StatusOK))

			allowedCaller.EgressRules = []*models.SecurityGroupRule{
				{
					Protocol:     models.AllProtocol,
					Destinations: []string{"0.0.0.0/0"},
				},
			}

			Expect(bbsClient.DesireLRP(logger, traceID, allowedCaller)).To(Succeed())
			Eventually(ActualGetter(logger, allowedCallerGuid, 0)).Should(BeActualLRPWithState(allowedCallerGuid, 0, models.ActualLRPStateRunning))
			Eventually(EndpointCurler("http://" + RouteForGuid(allowedCallerGuid) + "/env")).Should(Equal(http.StatusOK))
		})

		It("should allow access to an internal IP", func() {
			endpoint := "/curl?url=http://" + gorouterLBIP + ":80"
			urlToProxyThroughDisallowedCaller := "http://" + RouteForGuid(disallowedCallerGuid) + endpoint
			urlToProxyThroughAllowedCaller := "http://" + RouteForGuid(allowedCallerGuid) + endpoint

			By("verifiying that without egress rules, this network call is disallowed")
			resp, err := http.Get(urlToProxyThroughDisallowedCaller)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			By("asserting that opening up the security group rule allows us to call into the internal IP")
			resp, err = http.Get(urlToProxyThroughAllowedCaller)
			Expect(err).NotTo(HaveOccurred())
			// Any reply from the gorouter indicates that the application security group is in place
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})

	Context("for Tasks", func() {
		var allowedTask, disallowedTask *models.TaskDefinition
		var allowedTaskGuid, disallowedTaskGuid string

		BeforeEach(func() {
			allowedTaskGuid, disallowedTaskGuid = NewGuid(), NewGuid()
			allowedTask, disallowedTask = Task(), Task()
			allowedTask.ResultFile, disallowedTask.ResultFile = "", ""

			// Test whether the process can establish a tcp connection on port 80 to the internal IP
			ncAction := models.WrapAction(&models.RunAction{
				Path: "bash",
				Args: []string{"-c", "nc -w 2 " + gorouterLBIP + " 80"},
				User: "vcap",
			})

			disallowedTask.Action = ncAction
			allowedTask.Action = ncAction

			allowedTask.EgressRules = []*models.SecurityGroupRule{
				{
					Protocol:     models.AllProtocol,
					Destinations: []string{"0.0.0.0/0"},
				},
			}
		})

		It("should allow access to an internal IP", func() {
			Expect(bbsClient.DesireTask(logger, traceID, allowedTaskGuid, domain, allowedTask)).To(Succeed())
			Expect(bbsClient.DesireTask(logger, traceID, disallowedTaskGuid, domain, disallowedTask)).To(Succeed())

			Eventually(TaskGetter(logger, allowedTaskGuid)).Should(HaveTaskState(models.Task_Completed))
			Eventually(TaskGetter(logger, disallowedTaskGuid)).Should(HaveTaskState(models.Task_Completed))

			By("verifiying that without egress rules, this network call is disallowed")
			task, err := bbsClient.TaskByGuid(logger, traceID, disallowedTaskGuid)
			Expect(err).NotTo(HaveOccurred())
			Expect(task.Failed).To(Equal(true))

			By("asserting that opening up the security group rule allows us to call into the internal IP")
			task, err = bbsClient.TaskByGuid(logger, traceID, allowedTaskGuid)
			Expect(err).NotTo(HaveOccurred())
			Expect(task.Failed).To(Equal(false))

			Expect(bbsClient.ResolvingTask(logger, traceID, allowedTaskGuid)).To(Succeed())
			Expect(bbsClient.DeleteTask(logger, traceID, allowedTaskGuid)).To(Succeed())
			Expect(bbsClient.ResolvingTask(logger, traceID, disallowedTaskGuid)).To(Succeed())
			Expect(bbsClient.DeleteTask(logger, traceID, disallowedTaskGuid)).To(Succeed())
		})
	})
})
