package k8sgarden

import (
	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/k8s-garden-client/pkg/containerd"
	"code.cloudfoundry.org/k8s-garden-client/pkg/kubelet"
)

type Config struct {
	NodeName                      string
	WorkloadsNamespace            string
	SidecarRootfs                 string
	TrustedSystemCertificatesPath string
	EnableContainerProxy          bool
}

type Option func(*client)

func WithContainerdClient(c containerd.Client) Option {
	return func(cl *client) { cl.containerdClient = c }
}

func WithKubeletClient(k kubelet.Client) Option {
	return func(cl *client) { cl.kubeletClient = k }
}

func WithCommandRunner(r commandrunner.CommandRunner) Option {
	return func(cl *client) { cl.cmdRunner = r }
}

func WithSandboxPath(sandboxPath string) Option {
	return func(cl *client) { cl.sandboxPath = sandboxPath }
}

func WithUserLookupper(u users.UserLookupper) Option {
	return func(cl *client) { cl.userLookupper = u }
}
