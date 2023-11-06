module code.cloudfoundry.org

go 1.21.0

toolchain go1.21.3

replace (
	code.cloudfoundry.org/garden => ../garden
	code.cloudfoundry.org/grootfs => ../grootfs
	code.cloudfoundry.org/guardian => ../guardian
	code.cloudfoundry.org/idmapper => ../idmapper

	github.com/docker/docker => github.com/docker/docker v20.10.25+incompatible
	github.com/nats-io/nats.go => github.com/nats-io/nats.go v1.16.1-0.20220906180156-a1017eec10b0
	github.com/onsi/gomega => github.com/onsi/gomega v1.27.1
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.11.1
	github.com/prometheus/common => github.com/prometheus/common v0.30.0
	github.com/spf13/cobra => github.com/spf13/cobra v0.0.0-20160722081547-f62e98d28ab7
	github.com/zorkian/go-datadog-api => github.com/zorkian/go-datadog-api v0.0.0-20150915071709-8f1192dcd661
)

require (
	code.cloudfoundry.org/archiver v0.0.0-20231101144724-e7fc2b4c7367
	code.cloudfoundry.org/bytefmt v0.0.0-20231017140541-3b893ed0421b
	code.cloudfoundry.org/certsplitter v0.0.0-20231030182437-e70b934b241d
	code.cloudfoundry.org/cf-routing-test-helpers v0.0.0-20230612154734-4f65ecb98d93
	code.cloudfoundry.org/cf-tcp-router v0.0.0-20230911184912-89625e5d5967
	code.cloudfoundry.org/cfhttp v2.0.0+incompatible
	code.cloudfoundry.org/cfhttp/v2 v2.0.1-0.20210513172332-4c5ee488a657
	code.cloudfoundry.org/clock v1.1.0
	code.cloudfoundry.org/credhub-cli v0.0.0-20231030130452-94343e94cef6
	code.cloudfoundry.org/debugserver v0.0.0-20231101144826-61d4b1f2e7b6
	code.cloudfoundry.org/diego-logging-client v0.0.0-20231101144625-b1a01cfc966d
	code.cloudfoundry.org/dockerdriver v0.0.0-20231025145110-1eefbc443913
	code.cloudfoundry.org/durationjson v0.0.0-20231101144611-73752e36c7eb
	code.cloudfoundry.org/eventhub v0.0.0-20231101144817-e1b5e1cd15a3
	code.cloudfoundry.org/garden v0.0.0-20231031181541-0ff6ac1ac49c
	code.cloudfoundry.org/go-loggregator/v8 v8.0.5
	code.cloudfoundry.org/goshims v0.26.0
	code.cloudfoundry.org/guardian v0.0.0-20231031182052-8789f016ea2d
	code.cloudfoundry.org/lager/v3 v3.0.2
	code.cloudfoundry.org/localip v0.0.0-20231101144618-f2950077affe
	code.cloudfoundry.org/tlsconfig v0.0.0-20231017135636-f0e44068c22f
	github.com/GaryBoone/GoStats v0.0.0-20130122001700-1993eafbef57
	github.com/ajstarks/svgo v0.0.0-20211024235047-1546f124cd8b
	github.com/aws/aws-sdk-go v1.47.3
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20231024185945-8841054dbdb8
	github.com/cactus/go-statsd-client v3.1.1-0.20161031215955-d8eabe07bc70+incompatible
	github.com/cloudfoundry-community/go-uaa v0.3.2
	github.com/cloudfoundry/dropsonde v1.1.0
	github.com/containers/image v3.0.2+incompatible
	github.com/docker/docker v24.0.7+incompatible
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7
	github.com/envoyproxy/go-control-plane v0.11.1
	github.com/fsnotify/fsnotify v1.7.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-sql-driver/mysql v1.7.1
	github.com/go-test/deep v1.1.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang-jwt/jwt/v4 v4.5.0
	github.com/golang/protobuf v1.5.3
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/hashicorp/errwrap v1.1.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jackc/pgx v3.6.2+incompatible
	github.com/jinzhu/gorm v1.9.16
	github.com/kr/pty v1.1.8
	github.com/lib/pq v1.10.9
	github.com/mitchellh/hashstructure v1.1.0
	github.com/nats-io/nats-server/v2 v2.10.4
	github.com/nats-io/nats.go v1.31.0
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo/v2 v2.13.0
	github.com/onsi/gomega v1.29.0
	github.com/onsi/say v1.1.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.0-rc5
	github.com/openzipkin/zipkin-go v0.4.2
	github.com/pborman/getopt v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/pkg/sftp v1.13.6
	github.com/spf13/cobra v1.7.0
	github.com/square/certstrap v1.3.0
	github.com/tedsuo/ifrit v0.0.0-20230516164442-7862c310ad26
	github.com/tedsuo/rata v1.0.0
	github.com/vito/go-sse v1.0.0
	golang.org/x/crypto v0.14.0
	golang.org/x/net v0.17.0
	golang.org/x/oauth2 v0.13.0
	golang.org/x/sys v0.13.0
	golang.org/x/time v0.3.0
	google.golang.org/grpc v1.59.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	code.cloudfoundry.org/commandrunner v0.0.0-20230612151827-2b11a2b4e9b8 // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20231023230358-aea71c8ef532 // indirect
	filippo.io/edwards25519 v1.0.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/BurntSushi/toml v1.3.2 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/aws/aws-sdk-go-v2 v1.22.1 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.21.0 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.15.0 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.14.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.2.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.5.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.5.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecr v1.22.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.20.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.10.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.17.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.19.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.25.0 // indirect
	github.com/aws/smithy-go v1.16.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bmizerany/pat v0.0.0-20210406213842-e4b6760bdd6f // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cloudfoundry/go-socks5 v0.0.0-20180221174514-54f73bdb8a8e // indirect
	github.com/cloudfoundry/socks5-proxy v0.2.102 // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20230911203642-fa89d986ae20 // indirect
	github.com/cncf/xds/go v0.0.0-20231016030527-8bd2eac9fb4a // indirect
	github.com/cockroachdb/apd v1.1.0 // indirect
	github.com/containers/storage v1.45.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/creack/pty v1.1.20 // indirect
	github.com/cyphar/filepath-securejoin v0.2.4 // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.8.0 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.2 // indirect
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gofrs/uuid v4.0.0+incompatible // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/pprof v0.0.0-20231023181126-ff6d637d2a7b // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/hashicorp/go-version v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/fake v0.0.0-20150926172116-812a484cc733 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/klauspost/compress v1.17.2 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/minio/highwayhash v1.0.2 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/moby/sys/mountinfo v0.6.2 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/nats-io/jwt/v2 v2.5.3 // indirect
	github.com/nats-io/nkeys v0.4.6 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/opencontainers/runc v1.1.10 // indirect
	github.com/opencontainers/runtime-spec v1.1.0 // indirect
	github.com/prometheus/client_golang v1.17.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.45.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.step.sm/crypto v0.36.1 // indirect
	go.uber.org/automaxprocs v1.5.3 // indirect
	golang.org/x/mod v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	golang.org/x/tools v0.14.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20231030173426-d783a09b4405 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231030173426-d783a09b4405 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
