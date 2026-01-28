package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"code.cloudfoundry.org/dockerapplifecycle"
	"code.cloudfoundry.org/dockerapplifecycle/protocol"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	DockerHubHostname    = "registry-1.docker.io"
	DockerHubLoginServer = "https://index.docker.io/v1/"
	MAX_DOCKER_RETRIES   = 4
)

// borrowed from docker/docker
func splitReposName(reposName string) (string, string) {
	nameParts := strings.SplitN(reposName, "/", 2)
	var hostname, repoName string
	if len(nameParts) == 1 || (!strings.Contains(nameParts[0], ".") &&
		!strings.Contains(nameParts[0], ":") && nameParts[0] != "localhost") {
		// This is a Docker Index repos (ex: samalba/hipache or ubuntu)
		// 'docker.io' in docker/docker codebase, but they use indices...
		hostname = DockerHubHostname
		repoName = reposName
	} else {
		hostname = nameParts[0]
		repoName = nameParts[1]
	}
	return hostname, repoName
}

// For standard docker image references expressed as a protocol-less string
// returns RegistryURL, repoName, tag|digest
func ParseDockerRef(dockerRef string) (string, string, string) {
	remote, tag := ParseRepositoryTag(dockerRef)
	hostname, repoName := splitReposName(remote)

	if hostname == DockerHubHostname && !strings.Contains(repoName, "/") {
		repoName = "library/" + repoName
	}

	if len(tag) == 0 {
		tag = "latest"
	}
	return hostname, repoName, tag
}

// stolen from docker/docker
// Get a repos name and returns the right reposName + tag
// The tag can be confusing because of a port in a repository name.
//
//	Ex: localhost.localdomain:5000/samalba/hipache:latest
func ParseRepositoryTag(repos string) (string, string) {
	n := strings.LastIndex(repos, ":")
	if n < 0 {
		return repos, ""
	}
	if tag := repos[n+1:]; !strings.Contains(tag, "/") {
		return repos[:n], tag
	}
	return repos, ""
}

func FetchMetadata(registryURL, repoName, tag string, ctx *types.SystemContext, stderr io.Writer) (*v1.ImageConfig, error) {
	dockerRef := fmt.Sprintf("//%s/%s", registryURL, repoName+":"+tag)
	ref, err := docker.ParseReference(dockerRef)
	if err != nil {
		return nil, err
	}

	manifest.DefaultRequestedManifestMIMETypes = []string{
		v1.MediaTypeImageIndex,
		v1.MediaTypeImageManifest,
		manifest.DockerV2Schema2MediaType,
		manifest.DockerV2Schema1SignedMediaType,
		manifest.DockerV2Schema1MediaType,
	}

	var imageConfig *v1.Image
	var imgSrc types.ImageSource
	for i := 0; i < MAX_DOCKER_RETRIES; i++ {
		imgSrc, err = ref.NewImageSource(context.Background(), ctx)
		if err != nil && i < MAX_DOCKER_RETRIES-1 {
			fmt.Fprintln(stderr, "Failed getting docker image manifest by tag:", err, " Going to retry attempt:", i+1)
			continue
		} else if err != nil {
			fmt.Fprintln(stderr, "Failed getting docker image manifest by tag:", err)
			continue
		}
		defer imgSrc.Close()
		break
	}

	if err != nil {
		return nil, err
	}

	img, err := image.FromUnparsedImage(context.Background(), ctx, image.UnparsedInstance(imgSrc, nil))
	if err != nil {
		return nil, err
	}

	for i := 0; i < MAX_DOCKER_RETRIES; i++ {
		imageConfig, err = img.OCIConfig(context.Background())
		if err != nil && i < MAX_DOCKER_RETRIES-1 {
			fmt.Fprintln(stderr, "Failed getting docker image config by tag:", err, " Going to retry attempt:", i+1)
			continue
		} else if err != nil {
			fmt.Fprintln(stderr, "Failed getting docker image config by tag:", err)
			continue
		}

		break
	}

	if err != nil {
		return nil, err
	}

	return &imageConfig.Config, nil
}

func SaveMetadata(filename string, metadata *protocol.DockerImageMetadata) error {
	err := os.MkdirAll(path.Dir(filename), 0755)
	if err != nil {
		return err
	}

	executionMetadataJSON, err := json.Marshal(metadata.ExecutionMetadata)
	if err != nil {
		return err
	}

	resultFile, err := os.Create(filename)
	if err != nil {
		return err
	}

	defer resultFile.Close()

	startCommand := strings.Join(metadata.ExecutionMetadata.Cmd, " ")
	if len(metadata.ExecutionMetadata.Entrypoint) > 0 {
		startCommand = strings.Join([]string{strings.Join(metadata.ExecutionMetadata.Entrypoint, " "), startCommand}, " ")
	}

	err = json.NewEncoder(resultFile).Encode(dockerapplifecycle.NewStagingResult(
		dockerapplifecycle.ProcessTypes{
			"web": startCommand,
		},
		dockerapplifecycle.LifecycleMetadata{
			DockerImage: metadata.DockerImage,
		},
		string(executionMetadataJSON),
	))
	if err != nil {
		return err
	}

	return nil
}
