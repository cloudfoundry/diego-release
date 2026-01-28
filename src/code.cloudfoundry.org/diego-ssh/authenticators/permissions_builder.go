package authenticators

import (
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/diego-ssh/proxy"
	"code.cloudfoundry.org/diego-ssh/routes"
	"code.cloudfoundry.org/lager/v3"
	"golang.org/x/crypto/ssh"
)

type permissionsBuilder struct {
	bbsClient             bbs.InternalClient
	useDirectInstanceAddr bool
}

func NewPermissionsBuilder(bbsClient bbs.InternalClient, useDirectInstanceAddr bool) PermissionsBuilder {
	return &permissionsBuilder{
		bbsClient:             bbsClient,
		useDirectInstanceAddr: useDirectInstanceAddr,
	}
}

func (pb *permissionsBuilder) Build(logger lager.Logger, processGuid string, index int, metadata ssh.ConnMetadata) (*ssh.Permissions, error) {
	ind := int32(index)
	filter := models.ActualLRPFilter{
		ProcessGuid: processGuid,
		Index:       &ind,
	}
	actualLRPs, err := pb.bbsClient.ActualLRPs(logger, "", filter)
	if err != nil {
		return nil, err
	} else if len(actualLRPs) > 1 {
		return nil, fmt.Errorf("multiple matching ActualLRP for ProcessGuid: %s, Index: %d", processGuid, ind)
	} else if len(actualLRPs) == 0 {
		return nil, fmt.Errorf("no matching ActualLRP for ProcessGuid: %s, Index: %d", processGuid, ind)
	}

	desired, err := pb.bbsClient.DesiredLRPByProcessGuid(logger, "", processGuid)
	if err != nil {
		return nil, err
	}

	sshRoute, err := getRoutingInfo(desired)
	if err != nil {
		return nil, err
	}

	logMessage := fmt.Sprintf("Successful remote access by %s", metadata.RemoteAddr().String())

	return pb.createPermissions(sshRoute, actualLRPs[0], desired, logMessage)
}

func (pb *permissionsBuilder) createPermissions(
	sshRoute *routes.SSHRoute,
	actual *models.ActualLRP,
	desired *models.DesiredLRP,
	logMessage string,
) (*ssh.Permissions, error) {
	var targetConfig *proxy.TargetConfig

	for _, mapping := range actual.ActualLrpNetInfo.Ports {
		if mapping.ContainerPort == sshRoute.ContainerPort {
			address := actual.ActualLrpNetInfo.Address
			port := mapping.HostPort
			var useInstanceAddr bool
			switch actual.ActualLrpNetInfo.PreferredAddress {
			case models.ActualLRPNetInfo_PreferredAddressInstance:
				useInstanceAddr = true
			case models.ActualLRPNetInfo_PreferredAddressHost:
				useInstanceAddr = false
			case models.ActualLRPNetInfo_PreferredAddressUnknown:
				useInstanceAddr = pb.useDirectInstanceAddr
			}
			if useInstanceAddr {
				address = actual.ActualLrpNetInfo.InstanceAddress
				port = mapping.ContainerPort
			}

			tlsAddress := ""
			if mapping.HostTlsProxyPort > 0 {
				tlsAddress = fmt.Sprintf("%s:%d", actual.ActualLrpNetInfo.Address, mapping.HostTlsProxyPort)
			}

			if useInstanceAddr && mapping.ContainerTlsProxyPort > 0 {
				tlsAddress = fmt.Sprintf("%s:%d", actual.ActualLrpNetInfo.InstanceAddress, mapping.ContainerTlsProxyPort)
			}

			targetConfig = &proxy.TargetConfig{
				Address:             fmt.Sprintf("%s:%d", address, port),
				TLSAddress:          tlsAddress,
				ServerCertDomainSAN: actual.ActualLrpInstanceKey.InstanceGuid,
				HostFingerprint:     sshRoute.HostFingerprint,
				User:                sshRoute.User,
				Password:            sshRoute.Password,
				PrivateKey:          sshRoute.PrivateKey,
			}
			break
		}
	}

	if targetConfig == nil {
		return &ssh.Permissions{}, nil
	}

	targetConfigJson, err := json.Marshal(targetConfig)
	if err != nil {
		return nil, err
	}

	if len(desired.MetricTags) == 0 {
		desired.MetricTags = map[string]*models.MetricTagValue{}
	}
	if _, ok := desired.MetricTags["source_id"]; !ok {
		desired.MetricTags["source_id"] = &models.MetricTagValue{Static: desired.LogGuid}
	}
	if _, ok := desired.MetricTags["instance_id"]; !ok {
		desired.MetricTags["instance_id"] = &models.MetricTagValue{Dynamic: models.MetricTagValue_MetricTagDynamicValueIndex}
	}

	tags, err := models.ConvertMetricTags(desired.MetricTags, map[models.MetricTagValue_DynamicValue]interface{}{
		models.MetricTagValue_MetricTagDynamicValueIndex:        int32(actual.ActualLrpKey.Index),
		models.MetricTagValue_MetricTagDynamicValueInstanceGuid: actual.ActualLrpInstanceKey.InstanceGuid,
	})
	if err != nil {
		return nil, err
	}

	logMessageJson, err := json.Marshal(proxy.LogMessage{
		Message: logMessage,
		Tags:    tags,
	})
	if err != nil {
		return nil, err
	}

	return &ssh.Permissions{
		CriticalOptions: map[string]string{
			"proxy-target-config": string(targetConfigJson),
			"log-message":         string(logMessageJson),
		},
	}, nil
}

func getRoutingInfo(desired *models.DesiredLRP) (*routes.SSHRoute, error) {
	if desired.Routes == nil {
		return nil, RouteNotFoundErr
	}

	rawMessage := (*desired.Routes)[routes.DIEGO_SSH]
	if rawMessage == nil {
		return nil, RouteNotFoundErr
	}

	var sshRoute routes.SSHRoute
	err := json.Unmarshal(*rawMessage, &sshRoute)
	if err != nil {
		return nil, err
	}

	return &sshRoute, nil
}
