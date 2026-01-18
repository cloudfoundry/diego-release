package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/pcap"
	"github.com/gopacket/gopacket/pcapgo"
)

var (
	interfaceName = flag.String("interface", "", "Network interface to capture from (e.g. eth0, any)")
	snaplen       = flag.Int("snaplen", 65535, "Snapshot length - max bytes to capture per packet")
	filter        = flag.String("filter", "", "BPF filter expression (e.g. 'tcp port 80')")
	verbose       = flag.Bool("v", false, "Verbose output")
)

var (
	logLevel = &slog.LevelVar{}
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))
}

func main() {
	os.Exit(Main())
}

func Main() int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flag.Parse()

	if *interfaceName == "" {
		slog.Error("interface flag is required")
		return 1
	}

	if *verbose {
		logLevel.Set(slog.LevelDebug)
	}

	slog.Debug("parsed flags",
		"interface", *interfaceName,
		"snaplen", *snaplen,
		"filter", *filter,
	)

	errC, err := capture(ctx, *interfaceName, *snaplen, *filter)
	if err != nil {
		slog.Error("failed to set up capture", "error", err)
		return 1
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)

	select {
	case err := <-errC:
		if err != nil {
			slog.Error("capture failed", "error", err)
			return 1
		}
	case sig := <-sigChan:
		slog.Info("received signal, stopping capture", "signal", sig.String())
		cancel()
	}

	// drain channel to ensure clean shutdown
	for range errC {
	}

	slog.Info("stopped capture")

	return 0
}

func capture(ctx context.Context, interfaceName string, snaplen int, filter string) (<-chan error, error) {
	handle, err := pcap.OpenLive(interfaceName, int32(snaplen), true, pcap.BlockForever)
	if err != nil {
		return nil, fmt.Errorf("failed to open device %s: %w", interfaceName, err)
	}

	defaultFilter := genDefaultFilter(interfaceName)
	switch {
	case filter != "" && defaultFilter != "":
		filter = fmt.Sprintf("(%s) and (%s)", defaultFilter, filter)
		slog.Info("adjusted filter to avoid capturing SSH session", "filter", filter)
	case filter == "" && defaultFilter != "":
		filter = defaultFilter
		slog.Info("set filter to avoid capturing SSH session", "filter", filter)
	case defaultFilter == "": // && filter != ""
		slog.Warn("failed to determine default filter, capture may include SSH session")
	}

	err = handle.SetBPFFilter(filter)
	if err != nil {
		handle.Close()
		return nil, fmt.Errorf("failed to set BPF filter '%s': %w", filter, err)
	}

	pcapWriter := pcapgo.NewWriter(os.Stdout)
	err = pcapWriter.WriteFileHeader(uint32(snaplen), handle.LinkType())
	if err != nil {
		handle.Close()
		return nil, fmt.Errorf("failed to write pcap header: %w", err)
	}

	errC := make(chan error, 1)
	go func(ctx context.Context, h *pcap.Handle, w *pcapgo.Writer, errC chan<- error) {
		defer h.Close()
		defer close(errC)
		s := gopacket.NewPacketSource(h, h.LinkType())

		var err error
	packetLoop:
		for {
			select {
			case <-ctx.Done():
				h.Close()
			case packet, ok := <-s.Packets():
				if !ok {
					break packetLoop
				} else if packet == nil {
					continue
				}

				err = w.WritePacket(packet.Metadata().CaptureInfo, packet.Data())
				if err != nil {
					break packetLoop
				}
			}
		}

		errC <- err
	}(ctx, handle, pcapWriter, errC)

	return errC, nil
}

func genDefaultFilter(iface string) string {
	var port int
	switch iface {
	case "lo":
		port = 2222
		return "not tcp port 2222"
	case "eth0":
		port = envoySSHPort()
	default:
		port = -1
	}
	if port < 0 {
		return ""
	}

	return fmt.Sprintf("not tcp port %d", port)
}

func envoySSHPort() int {
	instancePortsJSON, ok := os.LookupEnv("CF_INSTANCE_PORTS")
	if !ok {
		return -1
	}

	var instancePorts []struct {
		InternalTLSProxy int `json:"internal_tls_proxy"`
		Internal         int `json:"internal"`
		// ExternalTlsProxy int `json:"external_tls_proxy"`
		// External         int `json:"external"`
	}
	err := json.Unmarshal([]byte(instancePortsJSON), &instancePorts)
	if err != nil {
		slog.Warn("failed to parse CF_INSTANCE_PORTS", "error", err)
		return -1
	}

	for _, p := range instancePorts {
		if p.Internal == 2222 {
			return p.InternalTLSProxy
		}
	}

	return -1
}
