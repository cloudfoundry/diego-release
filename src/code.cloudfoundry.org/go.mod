module code.cloudfoundry.org

go 1.21.6

replace (
	code.cloudfoundry.org/garden => ../garden
	code.cloudfoundry.org/grootfs => ../grootfs
	code.cloudfoundry.org/guardian => ../guardian
	code.cloudfoundry.org/idmapper => ../idmapper

	github.com/nats-io/nats.go => github.com/nats-io/nats.go v1.16.1-0.20220906180156-a1017eec10b0
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.11.1
	github.com/prometheus/common => github.com/prometheus/common v0.30.0
	github.com/spf13/cobra => github.com/spf13/cobra v0.0.0-20160722081547-f62e98d28ab7
	github.com/zorkian/go-datadog-api => github.com/zorkian/go-datadog-api v0.0.0-20150915071709-8f1192dcd661
)

require (
	code.cloudfoundry.org/archiver v0.0.0-20240705175209-d851fc3a04ea
	code.cloudfoundry.org/bytefmt v0.0.0-20240705175204-221292338951
	code.cloudfoundry.org/cacheddownloader v0.0.0-20240408163934-09b8631e33d0
	code.cloudfoundry.org/certsplitter v0.0.0-20240705175203-764523d29b39
	code.cloudfoundry.org/cf-routing-test-helpers v0.0.0-20240304203209-3404f81a986b
	code.cloudfoundry.org/cf-tcp-router v0.0.0-20240611155319-8cae5c93a2bd
	code.cloudfoundry.org/cfhttp/v2 v2.1.0
	code.cloudfoundry.org/clock v1.2.0
	code.cloudfoundry.org/credhub-cli v0.0.0-20240701130506-350489345474
	code.cloudfoundry.org/debugserver v0.0.0-20240705175209-1b863bb63b67
	code.cloudfoundry.org/diego-logging-client v0.0.0-20240705175210-f205aacaea3c
	code.cloudfoundry.org/dockerdriver v0.0.0-20240620154825-441e44b5dbb3
	code.cloudfoundry.org/durationjson v0.0.0-20240705175208-6880c3129de7
	code.cloudfoundry.org/eventhub v0.0.0-20240705175213-d572d3ae11a4
	code.cloudfoundry.org/garden v0.0.0-20240705200726-e508adf1e306
	code.cloudfoundry.org/go-loggregator/v9 v9.2.1
	code.cloudfoundry.org/goshims v0.37.0
	code.cloudfoundry.org/guardian v0.0.0-20240701200955-01b74f60037d
	code.cloudfoundry.org/lager/v3 v3.0.3
	code.cloudfoundry.org/localip v0.0.0-20240705175206-721695fced14
	code.cloudfoundry.org/tlsconfig v0.0.0-20240705175211-7a5a6eee6ef2
	github.com/GaryBoone/GoStats v0.0.0-20130122001700-1993eafbef57
	github.com/ajstarks/svgo v0.0.0-20211024235047-1546f124cd8b
	github.com/aws/aws-sdk-go v1.54.15
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20240701140119-53ce41df9490
	github.com/cactus/go-statsd-client v3.1.1-0.20161031215955-d8eabe07bc70+incompatible
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/cloudfoundry-community/go-uaa v0.3.2
	github.com/cloudfoundry/dropsonde v1.1.0
	github.com/containers/image/v5 v5.31.1
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7
	github.com/envoyproxy/go-control-plane v0.12.0
	github.com/fsnotify/fsnotify v1.7.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-sql-driver/mysql v1.8.1
	github.com/go-test/deep v1.1.1
	github.com/gogo/protobuf v1.3.2
	github.com/golang-jwt/jwt/v4 v4.5.0
	github.com/golang/protobuf v1.5.4
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/hashicorp/errwrap v1.1.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jackc/pgx/v5 v5.6.0
	github.com/jinzhu/gorm v1.9.16
	github.com/kr/pty v1.1.8
	github.com/lib/pq v1.10.9
	github.com/mitchellh/hashstructure v1.1.0
	github.com/moby/term v0.5.0
	github.com/nats-io/nats-server/v2 v2.10.17
	github.com/nats-io/nats.go v1.36.0
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo/v2 v2.19.0
	github.com/onsi/gomega v1.33.1
	github.com/onsi/say v1.1.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.0
	github.com/openzipkin/zipkin-go v0.4.3
	github.com/pborman/getopt v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/pkg/sftp v1.13.6
	github.com/spf13/cobra v1.8.1
	github.com/square/certstrap v1.3.0
	github.com/tedsuo/ifrit v0.0.0-20230516164442-7862c310ad26
	github.com/tedsuo/rata v1.0.0
	github.com/vito/go-sse v1.0.0
	golang.org/x/crypto v0.25.0
	golang.org/x/net v0.27.0
	golang.org/x/oauth2 v0.21.0
	golang.org/x/sys v0.22.0
	golang.org/x/time v0.5.0
	google.golang.org/grpc v1.65.0
	google.golang.org/protobuf v1.34.2
	gopkg.in/yaml.v2 v2.4.0
)

require (
	cel.dev/expr v0.15.0 // indirect
	code.cloudfoundry.org/commandrunner v0.0.0-20240705155009-6aef00a37535 // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20240604201846-c756bfed2ed3 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/BurntSushi/toml v1.4.0 // indirect
	github.com/aws/aws-sdk-go-v2 v1.30.1 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.27.24 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.24 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.9 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.13 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.13 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecr v1.30.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.25.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.11.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.11.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.22.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.26.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.30.1 // indirect
	github.com/aws/smithy-go v1.20.3 // indirect
	github.com/bmizerany/pat v0.0.0-20210406213842-e4b6760bdd6f // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cloudfoundry/go-socks5 v0.0.0-20180221174514-54f73bdb8a8e // indirect
	github.com/cloudfoundry/socks5-proxy v0.2.119 // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20240620221854-09ef53324489 // indirect
	github.com/cncf/xds/go v0.0.0-20240423153145-555b57ec207b // indirect
	github.com/containers/libtrust v0.0.0-20230121012942-c1716e8a8d01 // indirect
	github.com/containers/ocicrypt v1.2.0 // indirect
	github.com/containers/storage v1.54.0 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/creack/pty v1.1.21 // indirect
	github.com/cyphar/filepath-securejoin v0.2.5 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker v27.0.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.8.2 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.4 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/pprof v0.0.0-20240625030939-27f56978b8b0 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/minio/highwayhash v1.0.3 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/moby/sys/mountinfo v0.7.1 // indirect
	github.com/moby/sys/user v0.1.0 // indirect
	github.com/nats-io/jwt/v2 v2.5.8 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/opencontainers/runc v1.1.13 // indirect
	github.com/opencontainers/runtime-spec v1.2.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	github.com/vishvananda/netlink v1.2.1-beta.2 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	go.step.sm/crypto v0.48.1 // indirect
	go.uber.org/automaxprocs v1.5.3 // indirect
	golang.org/x/exp v0.0.0-20240613232115-7f521ea00fb8 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	golang.org/x/tools v0.22.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240701130421-f6361c86f094 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240701130421-f6361c86f094 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
