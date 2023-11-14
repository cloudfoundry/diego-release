package gardener

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/kawasaki/netns"
	"code.cloudfoundry.org/lager/v3"
	"github.com/docker/docker/pkg/reexec"
	"github.com/vishvananda/netlink"
)

func init() {
	reexec.Register("fetch-container-network-metrics", func() {
		var netNsPath, ifName string

		flag.StringVar(&netNsPath, "netNsPath", "", "netNsPath")
		flag.StringVar(&ifName, "ifName", "", "ifName")
		flag.Parse()

		fd, err := os.Open(netNsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "opening netns '%s': %s", netNsPath, err)
			os.Exit(1)
		}
		defer fd.Close()

		if err = (&netns.Execer{}).Exec(fd, func() error {
			link, err := netlink.LinkByName(ifName)
			if err != nil {
				return fmt.Errorf("could not get link '%s', %w", ifName, err)
			}
			fmt.Print((&ContainerNetworkStatMarshaller{}).MarshalLink(link))
			return nil
		}); err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
	})
}

type Opener func(path string) (*os.File, error)

func (o Opener) Open(path string) (*os.File, error) {
	return o(path)
}

type ContainerNetworkStatMarshaller struct {
}

func (c *ContainerNetworkStatMarshaller) Unmarshal(s string) (*garden.ContainerNetworkStat, error) {
	stats := strings.Split(s, ",")
	if len(stats) != 2 {
		return nil, fmt.Errorf("expected two values but got %q", s)
	}

	rxBytes, err := strconv.ParseUint(stats[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("could not parse rx_bytes value %q, %w", stats[0], err)
	}

	txBytes, err := strconv.ParseUint(stats[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("could not parse tx_bytes value %q, %w", stats[1], err)
	}

	return &garden.ContainerNetworkStat{
		RxBytes: rxBytes,
		TxBytes: txBytes,
	}, nil
}

func (c *ContainerNetworkStatMarshaller) MarshalLink(link netlink.Link) string {
	statistics := link.Attrs().Statistics
	return fmt.Sprintf("%d,%d", statistics.RxBytes, statistics.TxBytes)
}

type LinuxContainerNetworkMetricsProvider struct {
	containerizer                  Containerizer
	propertyManager                PropertyManager
	fileOpener                     Opener
	containerNetworkStatMarshaller *ContainerNetworkStatMarshaller
}

func NewLinuxContainerNetworkMetricsProvider(
	containerizer Containerizer,
	propertyManager PropertyManager,
	fileOpener Opener,
) *LinuxContainerNetworkMetricsProvider {
	return &LinuxContainerNetworkMetricsProvider{
		containerizer:                  containerizer,
		propertyManager:                propertyManager,
		fileOpener:                     fileOpener,
		containerNetworkStatMarshaller: &ContainerNetworkStatMarshaller{},
	}
}

func (l *LinuxContainerNetworkMetricsProvider) Get(log lager.Logger, handle string) (*garden.ContainerNetworkStat, error) {
	log = log.Session("container-network-metrics")

	ifName, found := l.propertyManager.Get(handle, ContainerInterfaceKey)
	if !found || ifName == "" {
		return nil, nil
	}

	info, err := l.containerizer.Info(log, handle)
	if err != nil {
		return nil, err
	}

	containerNetNs, err := l.fileOpener.Open(fmt.Sprintf("/proc/%d/ns/net", info.Pid))
	if err != nil {
		return nil, err
	}
	defer containerNetNs.Close()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	cmd := reexec.Command("fetch-container-network-metrics",
		"-ifName", ifName,
		"-netNsPath", containerNetNs.Name(),
	)
	cmd.Stderr = stderr
	cmd.Stdout = stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("could not fetch container network metrics, %q, %w", stderr.String(), err)
	}

	return l.containerNetworkStatMarshaller.Unmarshal(stdout.String())
}
