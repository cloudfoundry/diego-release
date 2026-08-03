package vizzini_test

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/routing-info/cfroutes"

	. "code.cloudfoundry.org/vizzini/matchers"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const HealthyCheckInterval = 30 * time.Second
const ConvergerInterval = 30 * time.Second
const CrashRestartTimeout = 30 * time.Second

//Tasks

func TaskGetter(logger lager.Logger, guid string) func() (*models.Task, error) {
	return func() (*models.Task, error) {
		return bbsClient.TaskByGuid(logger, traceID, guid)
	}
}

func TasksByDomainGetter(logger lager.Logger, domain string) func() ([]*models.Task, error) {
	return func() ([]*models.Task, error) {
		return bbsClient.TasksByDomain(logger, traceID, domain)
	}
}

func ClearOutTasksInDomain(domain string) {
	tasks, err := bbsClient.TasksByDomain(logger, traceID, domain)
	Expect(err).NotTo(HaveOccurred())
	for _, task := range tasks {
		if task.State != models.Task_Completed {
			bbsClient.CancelTask(logger, traceID, task.TaskGuid)
			Eventually(TaskGetter(logger, task.TaskGuid)).Should(HaveTaskState(models.Task_Completed))
		}
		Expect(bbsClient.ResolvingTask(logger, traceID, task.TaskGuid)).To(Succeed())
		Expect(bbsClient.DeleteTask(logger, traceID, task.TaskGuid)).To(Succeed())
	}
	Eventually(TasksByDomainGetter(logger, domain)).Should(BeEmpty())
}

func Task() *models.TaskDefinition {
	return &models.TaskDefinition{
		Action: models.WrapAction(&models.RunAction{
			Path: "bash",
			Args: []string{"-c", "echo 'some output' > /tmp/bar"},
			User: "vcap",
		}),
		RootFs:        config.DefaultRootFS,
		MemoryMb:      128,
		DiskMb:        256,
		CpuWeight:     100,
		LogGuid:       guid,
		LogSource:     "VIZ",
		ResultFile:    "/tmp/bar",
		Annotation:    "arbitrary-data",
		PlacementTags: PlacementTags(),
	}
}

//LRPs

func LRPGetter(logger lager.Logger, guid string) func() (*models.DesiredLRP, error) {
	return func() (*models.DesiredLRP, error) {
		return bbsClient.DesiredLRPByProcessGuid(logger, traceID, guid)
	}
}

func ActualLRPByProcessGuidAndIndex(logger lager.Logger, guid string, index int) (models.ActualLRP, error) {
	i := int32(index)
	lrps, err := bbsClient.ActualLRPs(logger, traceID, models.ActualLRPFilter{ProcessGuid: guid, Index: &i})
	if err != nil {
		return models.ActualLRP{}, err
	}
	if len(lrps) != 1 {
		return models.ActualLRP{}, fmt.Errorf("found more than one or no matching ActualLRP ProcessGuid: %s Index: %d", guid, index)
	}
	return *lrps[0], nil
}

func ActualsByProcessGuid(logger lager.Logger, guid string) ([]models.ActualLRP, error) {
	lrps, err := bbsClient.ActualLRPs(logger, traceID, models.ActualLRPFilter{ProcessGuid: guid})
	if err != nil {
		return nil, err
	}
	actualLRPs := make([]models.ActualLRP, len(lrps))
	for k, v := range lrps {
		actualLRPs[k] = *v
	}

	return actualLRPs, nil
}

func ActualsByDomain(logger lager.Logger, domain string) ([]models.ActualLRP, error) {
	lrps, err := bbsClient.ActualLRPs(logger, traceID, models.ActualLRPFilter{Domain: domain})
	if err != nil {
		return nil, err
	}

	actualLRPs := make([]models.ActualLRP, len(lrps))
	for k, v := range lrps {
		actualLRPs[k] = *v
	}

	return actualLRPs, nil
}

func ActualGetter(logger lager.Logger, guid string, index int) func() (models.ActualLRP, error) {
	return func() (models.ActualLRP, error) {
		return ActualLRPByProcessGuidAndIndex(logger, guid, index)
	}
}

func ActualByProcessGuidGetter(logger lager.Logger, guid string) func() ([]models.ActualLRP, error) {
	return func() ([]models.ActualLRP, error) {
		return ActualsByProcessGuid(logger, guid)
	}
}

func ActualByDomainGetter(logger lager.Logger, domain string) func() ([]models.ActualLRP, error) {
	return func() ([]models.ActualLRP, error) {
		return ActualsByDomain(logger, domain)
	}
}

func ClearOutDesiredLRPsInDomain(domain string) {
	lrps, err := bbsClient.DesiredLRPs(logger, traceID, models.DesiredLRPFilter{Domain: domain})
	Expect(err).NotTo(HaveOccurred())
	for _, lrp := range lrps {
		Expect(bbsClient.RemoveDesiredLRP(logger, traceID, lrp.ProcessGuid)).To(Succeed())
	}
	// Wait enough time for the Grace app to exit if it was run with -catchTerminate
	Eventually(ActualByDomainGetter(logger, domain), timeout+8*time.Second).Should(BeEmpty())
}

func EndpointCurler(endpoint string) func() int {
	return func() int {
		resp, err := http.Get(endpoint)
		if err != nil {
			return -1
		}
		resp.Body.Close()
		return resp.StatusCode
	}
}

func EndpointContentCurler(endpoint string) func() string {
	return func() string {
		resp, err := http.Get(endpoint)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		content, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		return string(content)
	}
}

func IndexCounter(guid string, optionalHttpClient ...*http.Client) func() int {
	return IndexCounterWithAttempts(guid, 100, optionalHttpClient...)
}

func IndexCounterWithAttempts(guid string, attempts int, optionalHttpClient ...*http.Client) func() int {
	return func() int {
		counts := map[int]bool{}
		for i := 0; i < attempts; i++ {
			index := GetIndexFromEndpointFor(guid, optionalHttpClient...)
			if index == -1 {
				continue
			}
			counts[index] = true
		}
		return len(counts)
	}
}

func GetIndexFromEndpointFor(guid string, optionalHttpClient ...*http.Client) int {
	httpClient := http.DefaultClient
	if len(optionalHttpClient) == 1 {
		httpClient = optionalHttpClient[0]
	}
	url := "http://" + RouteForGuid(guid) + "/index"
	resp, err := httpClient.Get(url)
	if err != nil {
		return -1
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return -1
	}
	content, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	Expect(err).NotTo(HaveOccurred())

	index, err := strconv.Atoi(string(content))
	Expect(err).NotTo(HaveOccurred())
	return index
}

func GraceCounterGetter(guid string) func() (int, error) {
	return func() (int, error) {
		resp, err := http.Get("http://" + RouteForGuid(guid) + "/counter")
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()
		content, err := io.ReadAll(resp.Body)
		if err != nil {
			return 0, err
		}
		return strconv.Atoi(string(content))
	}
}

func StartedAtGetter(guid string) func() (int64, error) {
	url := "http://" + RouteForGuid(guid) + "/started-at"
	return func() (int64, error) {
		resp, err := http.Get(url)
		if err != nil {
			return 0, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return 0, fmt.Errorf("invalid status code: %d", resp.StatusCode)
		}
		content, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return 0, err
		}
		return strconv.ParseInt(string(content), 10, 64)
	}
}

func RouteForGuid(guid string) string {
	return fmt.Sprintf("%s.%s", guid, config.RoutableDomainSuffix)
}

func TLSDirectAddressFor(guid string, index int, containerPort uint32) string {
	actualLRP, err := ActualGetter(logger, guid, index)()
	Expect(err).NotTo(HaveOccurred())
	Expect(actualLRP).NotTo(BeZero())

	for _, portMapping := range actualLRP.Ports {
		if portMapping.ContainerPort == containerPort {
			return fmt.Sprintf("%s:%d", actualLRP.Address, portMapping.HostTlsProxyPort)
		}
	}

	ginkgo.Fail(fmt.Sprintf("could not find port %d for ActualLRP %d with ProcessGuid %s", containerPort, index, guid))
	return ""
}

func DesiredLRPWithGuid(guid string) *models.DesiredLRP {
	routingInfo := cfroutes.CFRoutes{
		{Port: 8080, Hostnames: []string{RouteForGuid(guid)}},
	}.RoutingInfo()

	return &models.DesiredLRP{
		ProcessGuid:   guid,
		PlacementTags: PlacementTags(),
		Domain:        domain,
		Instances:     1,
		CachedDependencies: []*models.CachedDependency{
			&models.CachedDependency{
				From:              config.GraceTarballURL,
				To:                "/tmp/grace",
				CacheKey:          "grace",
				ChecksumAlgorithm: "sha1",
				ChecksumValue:     config.GraceTarballChecksum,
			},
		},
		Action: models.WrapAction(&models.RunAction{
			Path: "/tmp/grace/grace",
			User: "vcap",
			Env: []*models.EnvironmentVariable{
				{Name: "PORT", Value: "8080"},
				{Name: "ACTION_LEVEL", Value: "COYOTE"},
				{Name: "OVERRIDE", Value: "DAQUIRI"}},
		}),
		Monitor: models.WrapAction(&models.RunAction{
			Path: "nc",
			Args: []string{"-z", "0.0.0.0", "8080"},
			User: "vcap",
		}),
		RootFs:     config.DefaultRootFS,
		MemoryMb:   128,
		DiskMb:     256,
		CpuWeight:  100,
		Ports:      []uint32{8080},
		Routes:     &routingInfo,
		LogGuid:    guid,
		LogSource:  "VIZ",
		MetricTags: map[string]*models.MetricTagValue{"source_id": {Static: guid}},
		Annotation: "arbitrary-data",
	}
}

func PlacementTags() []string {
	return config.RepPlacementTags
}
