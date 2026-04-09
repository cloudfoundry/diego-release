module code.cloudfoundry.org

go 1.25.7

replace (
	code.cloudfoundry.org/garden => ../garden
	code.cloudfoundry.org/grootfs => ../grootfs
	code.cloudfoundry.org/guardian => ../guardian
	code.cloudfoundry.org/idmapper => ../idmapper
)

require (
	code.cloudfoundry.org/archiver v0.66.0
	code.cloudfoundry.org/bytefmt v0.68.0
	code.cloudfoundry.org/cacheddownloader v0.0.0-20250312193827-23c030d5e4f3
	code.cloudfoundry.org/certsplitter v0.70.0
	code.cloudfoundry.org/cf-routing-test-helpers v0.0.0-20250228152907-f87c227cf5e7
	code.cloudfoundry.org/cf-tcp-router v0.0.0-20250228153011-2de6b290768e
	code.cloudfoundry.org/cfhttp/v2 v2.73.0
	code.cloudfoundry.org/clock v1.65.0
	code.cloudfoundry.org/credhub-cli v0.0.0-20260406130247-f0f6a4624b0e
	code.cloudfoundry.org/debugserver v0.91.0
	code.cloudfoundry.org/diego-logging-client v0.100.0
	code.cloudfoundry.org/dockerdriver v0.83.0
	code.cloudfoundry.org/durationjson v0.68.0
	code.cloudfoundry.org/eventhub v0.68.0
	code.cloudfoundry.org/garden v0.0.0-20260408024713-8ea40dc96c05
	code.cloudfoundry.org/go-loggregator/v9 v9.2.1
	code.cloudfoundry.org/goshims v0.96.0
	code.cloudfoundry.org/guardian v0.0.0-20260401025327-b8e60c988757
	code.cloudfoundry.org/lager/v3 v3.65.0
	code.cloudfoundry.org/localip v0.67.0
	code.cloudfoundry.org/tlsconfig v0.50.0
	github.com/GaryBoone/GoStats v0.0.0-20130122001700-1993eafbef57
	github.com/ajstarks/svgo v0.0.0-20211024235047-1546f124cd8b
	github.com/aws/aws-sdk-go-v2 v1.41.5
	github.com/aws/aws-sdk-go-v2/config v1.32.14
	github.com/aws/aws-sdk-go-v2/credentials v1.19.14
	github.com/aws/aws-sdk-go-v2/service/ecr v1.57.0
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.12.0
	github.com/cactus/go-statsd-client v3.1.1-0.20161031215955-d8eabe07bc70+incompatible
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/cloudfoundry-community/go-uaa v0.3.6
	github.com/cloudfoundry/dropsonde v1.1.0
	github.com/containers/image/v5 v5.36.2
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7
	github.com/envoyproxy/go-control-plane/envoy v1.37.0
	github.com/fsnotify/fsnotify v1.9.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-sql-driver/mysql v1.9.3
	github.com/go-test/deep v1.1.1
	github.com/gogo/protobuf v1.3.2
	github.com/golang-jwt/jwt/v4 v4.5.2
	github.com/golang/protobuf v1.5.4
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/gopacket/gopacket v1.5.0
	github.com/hashicorp/errwrap v1.1.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jackc/pgx/v5 v5.9.1
	github.com/jinzhu/gorm v1.9.16
	github.com/kr/pty v1.1.8
	github.com/lib/pq v1.12.3
	github.com/mitchellh/hashstructure v1.1.0
	github.com/moby/term v0.5.2
	github.com/nats-io/nats-server/v2 v2.12.6
	github.com/nats-io/nats.go v1.50.0
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo/v2 v2.28.1
	github.com/onsi/gomega v1.39.1
	github.com/onsi/say v1.2.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.1
	github.com/openzipkin/zipkin-go v0.4.3
	github.com/pborman/getopt v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/pkg/sftp v1.13.10
	github.com/spf13/cobra v1.10.2
	github.com/square/certstrap v1.3.0
	github.com/tedsuo/ifrit v0.0.0-20230516164442-7862c310ad26
	github.com/tedsuo/rata v1.0.0
	github.com/vito/go-sse v1.1.3
	golang.org/x/crypto v0.50.0
	golang.org/x/oauth2 v0.36.0
	golang.org/x/sys v0.43.0
	golang.org/x/time v0.15.0
	google.golang.org/grpc v1.80.0
	google.golang.org/protobuf v1.36.11
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	cel.dev/expr v0.25.1 // indirect
	code.cloudfoundry.org/commandrunner v0.59.0 // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20260209061029-a81ffbc46978 // indirect
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/antithesishq/antithesis-sdk-go v0.7.0 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.38.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.10 // indirect
	github.com/aws/smithy-go v1.24.3 // indirect
	github.com/bmizerany/pat v0.0.0-20210406213842-e4b6760bdd6f // indirect
	github.com/cloudfoundry/go-socks5 v0.0.0-20250423223041-4ad5fea42851 // indirect
	github.com/cloudfoundry/socks5-proxy v0.2.172 // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20251217143644-d1670a435f2a // indirect
	github.com/cncf/xds/go v0.0.0-20260202195803-dba9d589def2 // indirect
	github.com/containers/libtrust v0.0.0-20230121012942-c1716e8a8d01 // indirect
	github.com/containers/ocicrypt v1.2.1 // indirect
	github.com/containers/storage v1.59.1 // indirect
	github.com/coreos/go-systemd/v22 v22.7.0 // indirect
	github.com/creack/pty v1.1.24 // indirect
	github.com/cyphar/filepath-securejoin v0.6.1 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker v28.5.2+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.9.5 // indirect
	github.com/docker/go-connections v0.6.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.3 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/pprof v0.0.0-20260402051712-545e8a4df936 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/minio/highwayhash v1.0.4 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/moby/sys/capability v0.4.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/reexec v0.1.0 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/nats-io/jwt/v2 v2.8.1 // indirect
	github.com/nats-io/nkeys v0.4.15 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/opencontainers/cgroups v0.0.6 // indirect
	github.com/opencontainers/runtime-spec v1.3.0 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/prometheus/common v0.63.0 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/vishvananda/netlink v1.3.1 // indirect
	github.com/vishvananda/netns v0.0.5 // indirect
	go.step.sm/crypto v0.77.2 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.35.0 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	golang.org/x/tools v0.43.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260406210006-6f92a3bedf2d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260406210006-6f92a3bedf2d // indirect
)
