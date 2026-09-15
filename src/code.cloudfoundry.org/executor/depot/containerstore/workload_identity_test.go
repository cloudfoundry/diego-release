package containerstore

import (
	"code.cloudfoundry.org/executor"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("injectWorkloadIdentity", func() {
	var (
		baseConfig map[string]interface{}
		container  executor.Container
	)

	BeforeEach(func() {
		baseConfig = map[string]interface{}{
			"source":         "nfs://10.0.0.1/share",
			"mount-endpoint": "https://fss.example.com",
		}
	})

	Context("when the container is an LRP", func() {
		BeforeEach(func() {
			container = executor.Container{
				Guid: "instance-guid-abc",
				Tags: executor.Tags{
					executor.LifecycleTag:   executor.LRPLifecycle,
					executor.ProcessGuidTag: "app-guid-123",
				},
			}
		})

		It("injects the workload guid with the process-guid and the type as lrp", func() {
			result := injectWorkloadIdentity(baseConfig, container)
			Expect(result[executor.WorkloadGuidKey]).To(Equal("app-guid-123"))
			Expect(result[executor.WorkloadTypeKey]).To(Equal(executor.LRPLifecycle))
		})

		It("preserves existing config keys", func() {
			result := injectWorkloadIdentity(baseConfig, container)
			Expect(result["source"]).To(Equal("nfs://10.0.0.1/share"))
			Expect(result["mount-endpoint"]).To(Equal("https://fss.example.com"))
		})

		It("does not mutate the original config map", func() {
			injectWorkloadIdentity(baseConfig, container)
			Expect(baseConfig).NotTo(HaveKey(executor.WorkloadGuidKey))
			Expect(baseConfig).NotTo(HaveKey(executor.WorkloadTypeKey))
		})

		Context("but the process-guid tag is empty", func() {
			BeforeEach(func() {
				container.Tags[executor.ProcessGuidTag] = ""
			})

			It("returns the original config unchanged", func() {
				result := injectWorkloadIdentity(baseConfig, container)
				Expect(result).NotTo(HaveKey(executor.WorkloadGuidKey))
				Expect(result).NotTo(HaveKey(executor.WorkloadTypeKey))
				Expect(result).To(Equal(baseConfig))
			})
		})
	})

	Context("when the container is a Task", func() {
		BeforeEach(func() {
			container = executor.Container{
				Guid: "task-guid-xyz",
				Tags: executor.Tags{
					executor.LifecycleTag: executor.TaskLifecycle,
				},
			}
		})

		It("injects the workload guid with the task guid and the type as task", func() {
			result := injectWorkloadIdentity(baseConfig, container)
			Expect(result[executor.WorkloadGuidKey]).To(Equal("task-guid-xyz"))
			Expect(result[executor.WorkloadTypeKey]).To(Equal(executor.TaskLifecycle))
		})

		It("preserves existing config keys", func() {
			result := injectWorkloadIdentity(baseConfig, container)
			Expect(result["source"]).To(Equal("nfs://10.0.0.1/share"))
			Expect(result["mount-endpoint"]).To(Equal("https://fss.example.com"))
		})
	})

	Context("when the container has no lifecycle tag", func() {
		BeforeEach(func() {
			container = executor.Container{
				Guid: "some-guid",
				Tags: executor.Tags{},
			}
		})

		It("returns the original config unchanged", func() {
			result := injectWorkloadIdentity(baseConfig, container)
			Expect(result).NotTo(HaveKey(executor.WorkloadGuidKey))
			Expect(result).NotTo(HaveKey(executor.WorkloadTypeKey))
			Expect(result).To(Equal(baseConfig))
		})
	})

	Context("when the lifecycle tag has an unrecognized value", func() {
		BeforeEach(func() {
			container = executor.Container{
				Guid: "some-guid",
				Tags: executor.Tags{
					executor.LifecycleTag: "other",
				},
			}
		})

		It("injects no identity keys and preserves the existing config", func() {
			result := injectWorkloadIdentity(baseConfig, container)
			Expect(result).NotTo(HaveKey(executor.WorkloadGuidKey))
			Expect(result).NotTo(HaveKey(executor.WorkloadTypeKey))
			Expect(result["source"]).To(Equal("nfs://10.0.0.1/share"))
			Expect(result["mount-endpoint"]).To(Equal("https://fss.example.com"))
		})

		It("does not mutate the original config map", func() {
			injectWorkloadIdentity(baseConfig, container)
			Expect(baseConfig).NotTo(HaveKey(executor.WorkloadGuidKey))
			Expect(baseConfig).NotTo(HaveKey(executor.WorkloadTypeKey))
		})
	})

	Context("when config is nil", func() {
		It("returns an enriched map for an LRP (nil config is treated as empty)", func() {
			container = executor.Container{
				Guid: "instance-guid",
				Tags: executor.Tags{
					executor.LifecycleTag:   executor.LRPLifecycle,
					executor.ProcessGuidTag: "pg-1",
				},
			}
			result := injectWorkloadIdentity(nil, container)
			Expect(result).NotTo(BeNil())
			Expect(result[executor.WorkloadGuidKey]).To(Equal("pg-1"))
			Expect(result[executor.WorkloadTypeKey]).To(Equal(executor.LRPLifecycle))
		})

		It("returns an enriched map for a Task (nil config is treated as empty)", func() {
			container = executor.Container{
				Guid: "task-guid",
				Tags: executor.Tags{
					executor.LifecycleTag: executor.TaskLifecycle,
				},
			}
			result := injectWorkloadIdentity(nil, container)
			Expect(result).NotTo(BeNil())
			Expect(result[executor.WorkloadGuidKey]).To(Equal("task-guid"))
			Expect(result[executor.WorkloadTypeKey]).To(Equal(executor.TaskLifecycle))
		})

		It("returns nil when there is no lifecycle tag", func() {
			container = executor.Container{
				Guid: "some-guid",
				Tags: executor.Tags{},
			}
			result := injectWorkloadIdentity(nil, container)
			Expect(result).To(BeNil())
		})

		It("returns nil when the lifecycle tag is unrecognized", func() {
			container = executor.Container{
				Guid: "some-guid",
				Tags: executor.Tags{
					executor.LifecycleTag: "other",
				},
			}
			result := injectWorkloadIdentity(nil, container)
			Expect(result).To(BeNil())
		})
	})
})
