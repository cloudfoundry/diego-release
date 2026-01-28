package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"code.cloudfoundry.org/dockerapplifecycle/docker/nat"
	"code.cloudfoundry.org/dockerapplifecycle/helpers"
	"code.cloudfoundry.org/dockerapplifecycle/protocol"
	"code.cloudfoundry.org/ecrhelper"
	"github.com/containers/image/v5/types"
)

const ECR_REPO_REGEX = `[a-zA-Z0-9][a-zA-Z0-9_-]*\.dkr\.ecr(-fips)?\.[a-zA-Z0-9][a-zA-Z0-9_-]*\.amazonaws\.com(\.cn)?[^ ]*`

type Builder struct {
	RegistryURL                string
	RepoName                   string
	Tag                        string
	InsecureDockerRegistries   []string
	OutputFilename             string
	DockerDaemonExecutablePath string
	DockerDaemonUnixSocket     string
	DockerDaemonTimeout        time.Duration
	CacheDockerImage           bool
	DockerRegistryIPs          []string
	DockerRegistryHost         string
	DockerRegistryPort         int
	DockerRegistryRequireTLS   bool
	DockerLoginServer          string
	DockerUser                 string
	DockerPassword             string
	DockerEmail                string
	ECRHelper                  ecrhelper.ECRHelper
}

func (builder *Builder) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	select {
	case err := <-builder.build():
		if err != nil {
			return err
		}
	case signal := <-signals:
		return errors.New(signal.String())
	}

	return nil
}

func (builder Builder) build() <-chan error {
	errorChan := make(chan error, 1)

	go func() {
		defer close(errorChan)

		username, password, err := builder.getCredentials()
		if err != nil {
			errorChan <- err
			return
		}

		ctx := &types.SystemContext{
			DockerAuthConfig: &types.DockerAuthConfig{
				Username: username,
				Password: password,
			},
		}
		for _, insecure := range builder.InsecureDockerRegistries {
			if builder.RegistryURL == insecure {
				ctx.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
			}
		}

		imgConfig, err := helpers.FetchMetadata(builder.RegistryURL, builder.RepoName, builder.Tag, ctx, os.Stderr)
		if err != nil {
			errorChan <- fmt.Errorf(
				"failed to fetch metadata from [%s] with tag [%s] and insecure registries %s due to %s",
				builder.RepoName,
				builder.Tag,
				builder.InsecureDockerRegistries,
				err.Error(),
			)
			return
		}

		info := protocol.DockerImageMetadata{}
		if imgConfig != nil {
			info.ExecutionMetadata.Cmd = imgConfig.Cmd
			info.ExecutionMetadata.Entrypoint = imgConfig.Entrypoint
			info.ExecutionMetadata.Workdir = imgConfig.WorkingDir
			info.ExecutionMetadata.User = imgConfig.User
			info.ExecutionMetadata.ExposedPorts, err = extractPorts(convertPortsToNatPorts(imgConfig.ExposedPorts))
			if err != nil {
				portDetails := fmt.Sprintf("%v", imgConfig.ExposedPorts)
				println("failed to parse image ports", portDetails, err.Error())
				errorChan <- err
				return
			}
		}

		dockerImageURL := builder.RepoName
		if builder.RegistryURL != helpers.DockerHubHostname {
			dockerImageURL = builder.RegistryURL + "/" + dockerImageURL
		}
		if len(builder.Tag) > 0 {
			dockerImageURL = dockerImageURL + ":" + builder.Tag
		}
		info.DockerImage = dockerImageURL

		if err := helpers.SaveMetadata(builder.OutputFilename, &info); err != nil {
			errorChan <- fmt.Errorf(
				"failed to save metadata to [%s] due to %s",
				builder.OutputFilename,
				err.Error(),
			)
			return
		}

		errorChan <- nil
	}()

	return errorChan
}

func (builder Builder) getCredentials() (string, string, error) {
	isECRRepo, err := builder.ECRHelper.IsECRRepo(builder.RegistryURL)
	if err != nil {
		return "", "", fmt.Errorf(
			"failed to check whether the registry URL is ECR repo: %s",
			err.Error(),
		)
	}

	if !isECRRepo {
		return builder.DockerUser, builder.DockerPassword, nil
	}

	username, password, err := builder.ECRHelper.GetECRCredentials(builder.RegistryURL, builder.DockerUser, builder.DockerPassword)
	if err != nil {
		return "", "", fmt.Errorf(
			"failed to get ECR credentials from [%s] due to %s",
			builder.RegistryURL,
			err.Error(),
		)
	}
	return username, password, nil
}

func convertPortsToNatPorts(ports map[string]struct{}) map[nat.Port]struct{} {
	natPorts := map[nat.Port]struct{}{}
	for portProto, v := range ports {
		proto, port := nat.SplitProtoPort(portProto)
		p := nat.NewPort(proto, port)
		natPorts[p] = v
	}
	return natPorts
}

func extractPorts(dockerPorts map[nat.Port]struct{}) (exposedPorts []protocol.Port, err error) {
	sortedPorts := sortPorts(dockerPorts)
	for _, port := range sortedPorts {
		exposedPort, err := strconv.ParseUint(port.Port(), 10, 16)
		if err != nil {
			return []protocol.Port{}, err
		}
		exposedPorts = append(exposedPorts, protocol.Port{Port: uint16(exposedPort), Protocol: port.Proto()})
	}
	return exposedPorts, nil
}

func sortPorts(dockerPorts map[nat.Port]struct{}) []nat.Port {
	var dockerPortsSlice []nat.Port
	for port := range dockerPorts {
		dockerPortsSlice = append(dockerPortsSlice, port)
	}
	nat.Sort(dockerPortsSlice, func(ip, jp nat.Port) bool {
		return ip.Int() < jp.Int() || (ip.Int() == jp.Int() && ip.Proto() == "tcp")
	})
	return dockerPortsSlice
}
