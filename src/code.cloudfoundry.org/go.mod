module code.cloudfoundry.org

go 1.26.6

replace (
	github.com/cactus/go-statsd-client => github.com/cactus/go-statsd-client v2.0.2-0.20150911070441-6fa055a7b594+incompatible
	// pin ifrit until https://github.com/tedsuo/ifrit/pull/48 is merged
	github.com/tedsuo/ifrit => github.com/tedsuo/ifrit v0.0.0-20260418191334-846868129986
)

require (
	code.cloudfoundry.org/archiver v0.87.0
	code.cloudfoundry.org/bbs v1.16.0
	code.cloudfoundry.org/bbs/encryption v1.12.0
	code.cloudfoundry.org/bbs/format v1.11.0
	code.cloudfoundry.org/bbs/models v1.13.0
	code.cloudfoundry.org/bytefmt v0.89.0
	code.cloudfoundry.org/certsplitter v0.87.0
	code.cloudfoundry.org/cfhttp/v2 v2.94.0
	code.cloudfoundry.org/clock v1.87.0
	code.cloudfoundry.org/cnbapplifecycle v0.0.8
	code.cloudfoundry.org/credhub-cli v0.0.0-20260907130120-567c46b571cd
	code.cloudfoundry.org/debugserver v0.114.0
	code.cloudfoundry.org/diego-logging-client v0.124.0
	code.cloudfoundry.org/dockerdriver v0.106.0
	code.cloudfoundry.org/durationjson v0.89.0
	code.cloudfoundry.org/eventhub v0.89.0
	code.cloudfoundry.org/garden v0.2.0
	code.cloudfoundry.org/go-loggregator/v9 v9.2.1
	code.cloudfoundry.org/goshims v0.112.0
	code.cloudfoundry.org/guardian v0.0.0-20260902161120-8a4bd5473238
	code.cloudfoundry.org/k8s-garden-client v0.8.0
	code.cloudfoundry.org/lager/v3 v3.86.0
	code.cloudfoundry.org/localip v0.88.0
	code.cloudfoundry.org/locket v1.12.0
	code.cloudfoundry.org/routing-api v0.14.0
	code.cloudfoundry.org/routing-info v1.13.0
	code.cloudfoundry.org/tlsconfig v0.66.0
	github.com/GaryBoone/GoStats v0.0.0-20130122001700-1993eafbef57
	github.com/ajstarks/svgo v0.0.0-20211024235047-1546f124cd8b
	github.com/aws/aws-sdk-go-v2 v1.46.0
	github.com/aws/aws-sdk-go-v2/config v1.33.3
	github.com/aws/aws-sdk-go-v2/credentials v1.20.3
	github.com/aws/aws-sdk-go-v2/service/ecr v1.64.0
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.12.0
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/containerd/containerd/v2 v2.3.4
	github.com/containers/image/v5 v5.36.2
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7
	github.com/envoyproxy/go-control-plane/envoy v1.39.0
	github.com/fsnotify/fsnotify v1.10.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v1.4.4
	github.com/gogo/protobuf v1.3.2
	github.com/golang-jwt/jwt/v4 v4.5.2
	github.com/golang/protobuf v1.5.4
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/gopacket/gopacket v1.7.1
	github.com/hashicorp/errwrap v1.1.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/kr/pty v1.1.8
	github.com/mitchellh/hashstructure v1.1.0
	github.com/moby/term v0.5.2
	github.com/nats-io/nats-server/v2 v2.14.6
	github.com/nats-io/nats.go v1.53.1
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo/v2 v2.32.1
	github.com/onsi/gomega v1.43.0
	github.com/onsi/say v1.2.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.1
	github.com/openzipkin/zipkin-go v0.4.3
	github.com/pborman/getopt v1.1.0
	github.com/pkg/sftp v1.13.11
	github.com/spf13/cobra v1.10.2
	github.com/square/certstrap v1.3.0
	github.com/tedsuo/ifrit v0.0.0-20260813155221-94822c932811
	github.com/tedsuo/rata v1.0.0
	github.com/vito/go-sse v1.1.3
	golang.org/x/crypto v0.56.0
	golang.org/x/oauth2 v0.37.0
	golang.org/x/sys v0.48.0
	golang.org/x/time v0.16.0
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.12
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.36.3
	k8s.io/apimachinery v0.36.3
	k8s.io/client-go v0.36.3
	k8s.io/utils v0.0.0-20260707023825-cf1189d6abe3
	sigs.k8s.io/controller-runtime v0.24.1
)

require (
	cel.dev/expr v0.25.3 // indirect
	code.cloudfoundry.org/commandrunner v0.76.0 // indirect
	code.cloudfoundry.org/diego-db-helpers v0.17.0 // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20260831145205-e8366a756183 // indirect
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/Azure/azure-sdk-for-go v68.0.0+incompatible // indirect
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.30 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.24 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.13 // indirect
	github.com/Azure/go-autorest/autorest/azure/cli v0.4.7 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.1 // indirect
	github.com/Azure/go-autorest/logger v0.2.2 // indirect
	github.com/Azure/go-autorest/tracing v0.6.1 // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/Masterminds/semver/v3 v3.5.0 // indirect
	github.com/Microsoft/go-winio v0.6.3-0.20251027160822-ad3df93bed29 // indirect
	github.com/Microsoft/hcsshim v0.15.0-rc.4 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/antithesishq/antithesis-sdk-go v0.8.0 // indirect
	github.com/apex/log v1.9.0 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.19.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.46.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.9.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.37.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.42.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.49.0 // indirect
	github.com/aws/smithy-go v1.28.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bmizerany/pat v0.0.0-20210406213842-e4b6760bdd6f // indirect
	github.com/buildpacks/imgutil v0.0.0-20260824214648-e09626c50080 // indirect
	github.com/buildpacks/lifecycle v0.21.18 // indirect
	github.com/buildpacks/pack v0.40.9 // indirect
	github.com/cactus/go-statsd-client v3.2.1+incompatible // indirect
	github.com/chrismellard/docker-credential-acr-env v0.0.0-20230304212654-82a0ddb27589 // indirect
	github.com/cloudfoundry-community/go-uaa v0.5.0 // indirect
	github.com/cloudfoundry/dropsonde v1.1.0 // indirect
	github.com/cloudfoundry/go-socks5 v0.0.0-20250423223041-4ad5fea42851 // indirect
	github.com/cloudfoundry/socks5-proxy v0.2.187 // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20260818080958-d46298cd8513 // indirect
	github.com/cncf/xds/go v0.0.0-20260202195803-dba9d589def2 // indirect
	github.com/containerd/cgroups/v3 v3.1.3 // indirect
	github.com/containerd/containerd/api v1.11.1 // indirect
	github.com/containerd/continuity v0.5.0 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/fifo v1.1.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v1.0.0-rc.5 // indirect
	github.com/containerd/plugin v1.1.0 // indirect
	github.com/containerd/ttrpc v1.2.9 // indirect
	github.com/containerd/typeurl/v2 v2.3.0 // indirect
	github.com/containers/libtrust v0.0.0-20230121012942-c1716e8a8d01 // indirect
	github.com/containers/ocicrypt v1.3.2 // indirect
	github.com/containers/storage v1.59.1 // indirect
	github.com/coreos/go-systemd/v22 v22.7.0 // indirect
	github.com/creack/pty v1.1.24 // indirect
	github.com/cyphar/filepath-securejoin v0.7.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/cli v29.8.0+incompatible // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker v28.5.2+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.9.9 // indirect
	github.com/docker/go-connections v0.8.1 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.3 // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v1.0.0 // indirect
	github.com/go-openapi/jsonreference v1.0.0 // indirect
	github.com/go-openapi/swag v0.27.1 // indirect
	github.com/go-openapi/swag/cmdutils v0.27.1 // indirect
	github.com/go-openapi/swag/conv v0.27.1 // indirect
	github.com/go-openapi/swag/fileutils v0.27.1 // indirect
	github.com/go-openapi/swag/jsonutils v0.27.1 // indirect
	github.com/go-openapi/swag/loading v0.27.1 // indirect
	github.com/go-openapi/swag/mangling v0.27.1 // indirect
	github.com/go-openapi/swag/netutils v0.27.1 // indirect
	github.com/go-openapi/swag/pools v0.27.1 // indirect
	github.com/go-openapi/swag/stringutils v0.27.1 // indirect
	github.com/go-openapi/swag/typeutils v0.27.1 // indirect
	github.com/go-openapi/swag/yamlutils v0.27.1 // indirect
	github.com/go-sql-driver/mysql v1.10.1 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/go-test/deep v1.1.1 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-containerregistry v0.22.1 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/pprof v0.0.0-20260906184651-6331bc6350fe // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/heroku/color v0.0.6 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.11.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.20.0 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/minio/highwayhash v1.0.4 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/ioprogress v0.0.0-20180201004757-6a23b12fa88e // indirect
	github.com/moby/buildkit v0.33.0 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/moby/api v1.56.0 // indirect
	github.com/moby/moby/client v0.6.0 // indirect
	github.com/moby/sys/capability v0.4.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/reexec v0.1.0 // indirect
	github.com/moby/sys/sequential v0.7.0 // indirect
	github.com/moby/sys/signal v0.7.1 // indirect
	github.com/moby/sys/user v0.4.1 // indirect
	github.com/moby/sys/userns v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nats-io/jwt/v2 v2.8.2 // indirect
	github.com/nats-io/nkeys v0.4.16 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/opencontainers/cgroups v0.1.0 // indirect
	github.com/opencontainers/runtime-spec v1.3.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.24.1 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.70.1 // indirect
	github.com/prometheus/procfs v0.21.1 // indirect
	github.com/sirupsen/logrus v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/tonistiigi/go-csvvalue v0.0.0-20240814133006-030d3b2625d0 // indirect
	github.com/vishvananda/netlink v1.3.1 // indirect
	github.com/vishvananda/netns v0.0.5 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.71.0 // indirect
	go.opentelemetry.io/otel v1.46.0 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/sdk v1.46.0 // indirect
	go.opentelemetry.io/otel/trace v1.46.0 // indirect
	go.step.sm/crypto v0.90.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/term v0.46.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	golang.org/x/tools v0.49.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260908043556-f8649ddbbfe6 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260908043556-f8649ddbbfe6 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gorm.io/driver/mysql v1.6.0 // indirect
	gorm.io/driver/postgres v1.6.2 // indirect
	gorm.io/gorm v1.31.2 // indirect
	k8s.io/apiextensions-apiserver v0.36.0 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/kube-openapi v0.0.0-20260721132016-d427ff9ee9ad // indirect
	k8s.io/kubelet v0.36.3 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.4.2 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)
