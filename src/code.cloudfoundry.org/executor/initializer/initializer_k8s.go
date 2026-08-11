package initializer

import (
	"context"
	"errors"
	"os"

	"code.cloudfoundry.org/executor/depot/containerstore"
	"code.cloudfoundry.org/executor/initializer/configuration"
	GardenClient "code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/k8s-garden-client/pkg/containerd"
	"code.cloudfoundry.org/k8s-garden-client/pkg/k8sgarden"
	"code.cloudfoundry.org/k8s-garden-client/pkg/kubelet"
	"code.cloudfoundry.org/k8s-garden-client/pkg/log"
	"code.cloudfoundry.org/lager/v3"
	ctrdclient "github.com/containerd/containerd/v2/client"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	cconfig "sigs.k8s.io/controller-runtime/pkg/config"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const kubeletPort = "10250"

// newKubernetesGardenClient constructs the kubernetes-backed garden.Client from
// github.com/cloudfoundry/k8s-garden-client
func newKubernetesGardenClient(logger lager.Logger, config ExecutorConfig, sidecarRootFSPath string) (GardenClient.Client, containerstore.GardenClientFactory, configuration.RootFSSizer, error) {
	workloadsNamespace := config.WorkloadsNamespace
	if workloadsNamespace == "" {
		return nil, nil, nil, errors.New("workloads_namespace must be set when use_kubernetes_garden_client is enabled")
	}

	mgr, err := newControllerManager(logger, workloadsNamespace)
	if err != nil {
		return nil, nil, nil, err
	}

	go func() {
		if err := mgr.Start(context.Background()); err != nil {
			logger.Error("controller-manager-error", err)
			os.Exit(1)
		}
	}()

	if !mgr.GetCache().WaitForCacheSync(context.Background()) {
		return nil, nil, nil, errors.New("failed to sync controller-runtime cache")
	}

	kubeletClient, err := newKubeletClientFromConfig(mgr.GetConfig(), os.Getenv("NODE_IP"), kubeletPort)
	if err != nil {
		return nil, nil, nil, err
	}

	containerdClient, err := ctrdclient.New(config.GardenAddr, ctrdclient.WithDefaultNamespace("k8s.io"))
	if err != nil {
		return nil, nil, nil, err
	}

	gardenClient, err := k8sgarden.NewClient(
		logger.Session("k8sgarden"),
		mgr.GetClient(),
		k8sgarden.Config{
			NodeName:                      os.Getenv("NODE_NAME"),
			WorkloadsNamespace:            workloadsNamespace,
			SidecarRootfs:                 sidecarRootFSPath,
			TrustedSystemCertificatesPath: config.TrustedSystemCertificatesPath,
			EnableContainerProxy:          config.EnableContainerProxy,
		},
		k8sgarden.WithContainerdClient(containerd.NewClientWrapper(containerdClient)),
		k8sgarden.WithKubeletClient(kubeletClient),
	)
	if err != nil {
		return nil, nil, nil, err
	}

	return gardenClient, k8sgarden.NewFactory(gardenClient), k8sgarden.ZeroRootFSSizer{}, nil
}

func newKubeletClientFromConfig(config *rest.Config, addr, port string) (kubelet.Client, error) {
	configCopy := rest.CopyConfig(config)
	configCopy.Insecure = true
	configCopy.CAData = nil
	configCopy.CAFile = ""

	httpClient, err := rest.HTTPClientFor(configCopy)
	if err != nil {
		return nil, err
	}
	return kubelet.NewClient(httpClient, addr, port), nil
}

func newControllerManager(logger lager.Logger, workloadsNamespace string) (manager.Manager, error) {
	ctrllog.SetLogger(logr.New(log.NewSink(logger.Session("controller-manager"))))

	podSelector, err := labels.NewRequirement(k8sgarden.AppGUIDLabelKey, selection.Exists, nil)
	if err != nil {
		return nil, err
	}

	mgr, err := manager.New(ctrlconfig.GetConfigOrDie(), manager.Options{
		Scheme: clientgoscheme.Scheme,
		Controller: cconfig.Controller{
			NeedLeaderElection: ptr.To(false),
		},
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Pod{}: {
					Namespaces: map[string]cache.Config{
						workloadsNamespace: {
							FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": os.Getenv("NODE_NAME")}),
							LabelSelector: labels.NewSelector().Add(*podSelector),
						},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	_, err = mgr.GetCache().GetInformerForKind(context.Background(), corev1.SchemeGroupVersion.WithKind("Pod"), cache.BlockUntilSynced(true))
	if err != nil {
		return nil, err
	}

	return mgr, nil
}
