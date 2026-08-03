package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/dockerapplifecycle/helpers"
	"code.cloudfoundry.org/ecrhelper"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

type registries []string
type ips []string

func main() {
	var insecureDockerRegistries registries
	var dockerRegistryIPs ips

	flagSet := flag.NewFlagSet("builder", flag.ExitOnError)

	dockerRef := flagSet.String(
		"dockerRef",
		"",
		"docker image reference in standard docker string format",
	)

	outputFilename := flagSet.String(
		"outputMetadataJSONFilename",
		"/tmp/result/result.json",
		"filename in which to write the app metadata",
	)

	flagSet.Var(
		&insecureDockerRegistries,
		"insecureDockerRegistries",
		"insecure Docker Registry addresses (host:port)",
	)

	dockerDaemonExecutablePath := flagSet.String(
		"dockerDaemonExecutablePath",
		"/tmp/docker_app_lifecycle/docker",
		"path to the 'docker' executable",
	)

	dockerDaemonUnixSocket := flagSet.String(
		"dockerDaemonUnixSocket",
		"/var/run/docker.sock/info",
		"path to the unix socket the docker daemon is listening to",
	)

	cacheDockerImage := flagSet.Bool(
		"cacheDockerImage",
		false,
		"Caches Docker images to private docker registry",
	)

	flagSet.Var(
		&dockerRegistryIPs,
		"dockerRegistryIPs",
		"Docker Registry IPs",
	)

	dockerRegistryHost := flagSet.String(
		"dockerRegistryHost",
		"",
		"Docker Registry host",
	)

	dockerRegistryPort := flagSet.Int(
		"dockerRegistryPort",
		8080,
		"Docker Registry port",
	)

	dockerRegistryRequireTLS := flagSet.Bool(
		"dockerRegistryRequireTLS",
		false,
		"Use HTTPS when contacting the Docker Registry",
	)

	dockerLoginServer := flagSet.String(
		"dockerLoginServer",
		helpers.DockerHubLoginServer,
		"Docker Login server address",
	)

	dockerUser := flagSet.String(
		"dockerUser",
		"",
		"User for pulling from docker registry",
	)

	dockerPassword := flagSet.String(
		"dockerPassword",
		"",
		"Password for pulling from docker registry",
	)

	dockerEmail := flagSet.String(
		"dockerEmail",
		"",
		"Email for pulling from docker registry",
	)

	if err := flagSet.Parse(os.Args[1:len(os.Args)]); err != nil {
		println(err.Error())
		os.Exit(1)
	}

	var registryURL, repoName, tag string
	if len(*dockerRef) > 0 {
		registryURL, repoName, tag = helpers.ParseDockerRef(*dockerRef)
	} else {
		println("missing flag: dockerRef required")
		flagSet.PrintDefaults()
		os.Exit(1)
	}

	builder := Builder{
		RegistryURL:                registryURL,
		RepoName:                   repoName,
		Tag:                        tag,
		OutputFilename:             *outputFilename,
		DockerDaemonExecutablePath: *dockerDaemonExecutablePath,
		InsecureDockerRegistries:   insecureDockerRegistries,
		DockerDaemonTimeout:        10 * time.Second,
		DockerDaemonUnixSocket:     *dockerDaemonUnixSocket,
		CacheDockerImage:           *cacheDockerImage,
		DockerRegistryIPs:          dockerRegistryIPs,
		DockerRegistryHost:         *dockerRegistryHost,
		DockerRegistryPort:         *dockerRegistryPort,
		DockerRegistryRequireTLS:   *dockerRegistryRequireTLS,
		DockerLoginServer:          *dockerLoginServer,
		DockerUser:                 *dockerUser,
		DockerPassword:             *dockerPassword,
		DockerEmail:                *dockerEmail,
		ECRHelper:                  ecrhelper.NewECRHelper(),
	}

	members := grouper.Members{
		{Name: "builder", Runner: ifrit.RunFunc(builder.Run)},
	}

	group := grouper.NewParallel(os.Interrupt, members)
	process := ifrit.Invoke(sigmon.New(group))

	fmt.Println("Staging process started ...")

	err := <-process.Wait()
	if err != nil {
		println("Staging process failed:", err.Error())
		os.Exit(2)
	}

	fmt.Println("Staging process finished")
}

func (r *registries) String() string {
	return fmt.Sprint(*r)
}

func (r *registries) Set(value string) error {
	if len(*r) > 0 {
		return errors.New("Insecure Docker Registries flag already set")
	}
	for _, reg := range strings.Split(value, ",") {
		registry := strings.TrimSpace(reg)
		if strings.Contains(registry, "://") {
			return errors.New("no scheme allowed for Docker Registry [" + registry + "]")
		}
		if !strings.Contains(registry, ":") {
			return errors.New("ip:port expected for Docker Registry [" + registry + "]")
		}
		*r = append(*r, registry)
	}
	return nil
}

func (r *ips) String() string {
	return fmt.Sprint(*r)
}

func (r *ips) Set(value string) error {
	if len(*r) > 0 {
		return errors.New("Docker Registry IPs flag already set")
	}
	for _, el := range strings.Split(value, ",") {
		element := strings.TrimSpace(el)
		if strings.Contains(element, ":") {
			return errors.New("unexpected format for [" + element + "]")
		}
		*r = append(*r, element)
	}
	return nil
}
