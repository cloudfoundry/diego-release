package containerstore

import (
	"errors"
	"net"
	"strings"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
)

var ErrIPRangeConversionFailed = errors.New("failed to convert destination to ip range")

func newBindMount(src, dst string) garden.BindMount {
	return garden.BindMount{
		SrcPath: src,
		DstPath: dst,
		Mode:    garden.BindMountModeRO,
		Origin:  garden.BindMountOriginHost,
	}
}

func convertEnvVars(execEnv []executor.EnvironmentVariable) []string {
	env := make([]string, len(execEnv))
	for i := range execEnv {
		envVar := &execEnv[i]
		env[i] = envVar.Name + "=" + envVar.Value
	}
	return env
}

func convertEgressToNetOut(logger lager.Logger, egressRules []*models.SecurityGroupRule) ([]garden.NetOutRule, error) {
	netOutRules := make([]garden.NetOutRule, 0, len(egressRules))
	for _, rule := range egressRules {
		if err := rule.Validate(); err != nil {
			logger.Error("invalid-egress-rule", err)
			return nil, err
		}

		nextNetOutRuleSet, err := securityGroupRuleToNetOutRules(rule)
		if err != nil {
			logger.Error("failed-to-convert-to-net-out-rule", err)
			return nil, err
		}

		netOutRules = append(netOutRules, nextNetOutRuleSet...)
	}
	return netOutRules, nil
}

func securityGroupRuleToNetOutRules(securityRule *models.SecurityGroupRule) ([]garden.NetOutRule, error) {
	var protocol garden.Protocol
	var portRanges []garden.PortRange
	var icmp *garden.ICMPControl

	switch securityRule.Protocol {
	case models.TCPProtocol:
		protocol = garden.ProtocolTCP
	case models.UDPProtocol:
		protocol = garden.ProtocolUDP
	case models.ICMPProtocol:
		protocol = garden.ProtocolICMP
		icmp = &garden.ICMPControl{
			Type: garden.ICMPType(securityRule.IcmpInfo.Type),
			Code: garden.ICMPControlCode(uint8(securityRule.IcmpInfo.Code)),
		}
	case models.ICMPv6Protocol:
		protocol = garden.ProtocolICMPv6
		// Can reuse icmp because the fields are the same
		icmp = &garden.ICMPControl{
			Type: garden.ICMPType(securityRule.IcmpInfo.Type),
			Code: garden.ICMPControlCode(uint8(securityRule.IcmpInfo.Code)),
		}
	case models.AllProtocol:
		protocol = garden.ProtocolAll
	}

	if securityRule.PortRange != nil {
		portRanges = append(portRanges, garden.PortRange{Start: uint16(securityRule.PortRange.Start), End: uint16(securityRule.PortRange.End)})
	} else if securityRule.Ports != nil {
		for _, port := range securityRule.Ports {
			portRanges = append(portRanges, garden.PortRangeFromPort(uint16(port)))
		}
	}

	var destinations []string
	for _, d := range securityRule.Destinations {
		destinations = append(destinations, strings.Split(d, ",")...)
	}

	var netOutRules []garden.NetOutRule

	for _, dest := range destinations {
		ipRange, err := toIPRange(dest)
		if err != nil {
			return nil, err
		}

		var networks []garden.IPRange
		networks = append(networks, ipRange)

		newRule := garden.NetOutRule{
			Protocol: protocol,
			Networks: networks,
			Ports:    portRanges,
			ICMPs:    icmp,
			Log:      securityRule.Log,
		}

		netOutRules = append(netOutRules, newRule)
	}

	return netOutRules, nil
}

func toIPRange(dest string) (garden.IPRange, error) {
	idx := strings.IndexAny(dest, "-/")

	// Not a range or a CIDR
	if idx == -1 {
		ip := net.ParseIP(dest)
		if ip == nil {
			return garden.IPRange{}, ErrIPRangeConversionFailed
		}

		return garden.IPRangeFromIP(ip), nil
	}

	// We have a CIDR
	if dest[idx] == '/' {
		_, ipNet, err := net.ParseCIDR(dest)
		if err != nil {
			return garden.IPRange{}, ErrIPRangeConversionFailed
		}

		return garden.IPRangeFromIPNet(ipNet), nil
	}

	// We have an IP range
	firstIP := net.ParseIP(dest[:idx])
	secondIP := net.ParseIP(dest[idx+1:])
	if firstIP == nil || secondIP == nil {
		return garden.IPRange{}, ErrIPRangeConversionFailed
	}

	return garden.IPRange{Start: firstIP, End: secondIP}, nil
}
