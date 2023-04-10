module code.cloudfoundry.org

go 1.19

replace (
	code.cloudfoundry.org/garden => ../garden
	code.cloudfoundry.org/grootfs => ../grootfs
	code.cloudfoundry.org/guardian => ../guardian
	code.cloudfoundry.org/idmapper => ../idmapper
	github.com/docker/docker => github.com/docker/docker v20.10.13+incompatible
	github.com/envoyproxy/go-control-plane => github.com/envoyproxy/go-control-plane v0.9.5
	github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf => github.com/golang/protobuf v1.3.2
	github.com/hashicorp/consul => github.com/hashicorp/consul v0.7.0
	github.com/nats-io/nats.go => github.com/nats-io/nats.go v1.16.1-0.20220906180156-a1017eec10b0
	github.com/onsi/gomega => github.com/onsi/gomega v1.27.1
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.11.1
	github.com/prometheus/common => github.com/prometheus/common v0.30.0
	github.com/spf13/cobra => github.com/spf13/cobra v0.0.0-20160722081547-f62e98d28ab7
	github.com/zorkian/go-datadog-api => github.com/zorkian/go-datadog-api v0.0.0-20150915071709-8f1192dcd661
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20180817151627-c66870c02cf8
	google.golang.org/grpc => google.golang.org/grpc v1.29.0-dev.0.20200306163155-d179e8f5cd96
)

require (
	code.cloudfoundry.org/archiver v0.0.0-20230406165347-48bf3e51e0c6
	code.cloudfoundry.org/bytefmt v0.0.0-20230406175530-3579ae0680eb
	code.cloudfoundry.org/certsplitter v0.0.0-20230406165926-271f22a66575
	code.cloudfoundry.org/cf-routing-test-helpers v0.0.0-20200827173955-6ac4653025b4
	code.cloudfoundry.org/cf-tcp-router v0.0.0-20230405170811-6b149c7f7ef9
	code.cloudfoundry.org/cfhttp v1.0.1-0.20210513172332-4c5ee488a657
	code.cloudfoundry.org/cfhttp/v2 v2.0.1-0.20210513172332-4c5ee488a657
	code.cloudfoundry.org/clock v1.1.0
	code.cloudfoundry.org/credhub-cli v0.0.0-20220228140414-459eb2d27a1c
	code.cloudfoundry.org/debugserver v0.0.0-20230329140605-8c21649a9a42
	code.cloudfoundry.org/diego-logging-client v0.0.0-20230406170846-5a035e7a0fe0
	code.cloudfoundry.org/durationjson v0.0.0-20230406152829-ab56af44497d
	code.cloudfoundry.org/eventhub v0.0.0-20230406153306-c90e7c7a29a1
	code.cloudfoundry.org/garden v0.0.0-20230109170928-83a99bb745e3
	code.cloudfoundry.org/go-loggregator/v8 v8.0.5
	code.cloudfoundry.org/goshims v0.13.0
	code.cloudfoundry.org/guardian v0.0.0-00010101000000-000000000000
	code.cloudfoundry.org/lager/v3 v3.0.1
	code.cloudfoundry.org/localip v0.0.0-20230406154046-f137f65d303d
	code.cloudfoundry.org/tlsconfig v0.0.0-20230320190829-8f91c367795b
	github.com/GaryBoone/GoStats v0.0.0-20130122001700-1993eafbef57
	github.com/ajstarks/svgo v0.0.0-20210406150507-75cfd577ce75
	github.com/aws/aws-sdk-go v1.44.235
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20210324191134-efd1603705e9
	github.com/cactus/go-statsd-client v3.1.1-0.20161031215955-d8eabe07bc70+incompatible
	github.com/cloudfoundry-community/go-uaa v0.3.2-0.20221011190625-aaeaae3ce7c2
	github.com/cloudfoundry/dropsonde v1.0.1-0.20220627224009-409b7756c19d
	github.com/containers/image v3.0.2+incompatible
	github.com/docker/docker v23.0.1+incompatible
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7
	github.com/envoyproxy/go-control-plane v0.11.0
	github.com/fsnotify/fsnotify v1.6.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-sql-driver/mysql v1.7.0
	github.com/go-test/deep v1.0.8
	github.com/gogo/protobuf v1.3.2
	github.com/golang-jwt/jwt/v4 v4.5.0
	github.com/golang/protobuf v1.5.3
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/hashicorp/errwrap v1.1.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jackc/pgx v3.6.2+incompatible
	github.com/jinzhu/gorm v1.9.16
	github.com/kr/pty v1.1.8
	github.com/lib/pq v1.10.7
	github.com/mitchellh/hashstructure v1.1.0
	github.com/nats-io/nats-server/v2 v2.9.0
	github.com/nats-io/nats.go v1.16.1-0.20220906180156-a1017eec10b0
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo/v2 v2.9.2
	github.com/onsi/gomega v1.27.6
	github.com/onsi/say v1.0.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.0-rc2.0.20221005185240-3a7f492d3f1b
	github.com/pborman/getopt v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/pkg/sftp v1.13.5
	github.com/spf13/cobra v1.4.0
	github.com/square/certstrap v1.3.0
	github.com/tedsuo/ifrit v0.0.0-20230330192023-5cba443a66c4
	github.com/tedsuo/rata v1.0.0
	github.com/vito/go-sse v1.0.0
	golang.org/x/crypto v0.8.0
	golang.org/x/net v0.9.0
	golang.org/x/oauth2 v0.7.0
	golang.org/x/sys v0.7.0
	golang.org/x/time v0.0.0-20220722155302-e5dcc9cfc0b9
	google.golang.org/grpc v1.54.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	code.cloudfoundry.org/commandrunner v0.0.0-20230406174834-03aed0535eb9 // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20230406180255-f141159491b0 // indirect
	filippo.io/edwards25519 v1.0.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/BurntSushi/toml v1.2.1 // indirect
	github.com/Microsoft/go-winio v0.6.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bmizerany/pat v0.0.0-20210406213842-e4b6760bdd6f // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cloudfoundry/go-socks5 v0.0.0-20180221174514-54f73bdb8a8e // indirect
	github.com/cloudfoundry/socks5-proxy v0.2.44 // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20230323202738-86a2a74b11b0 // indirect
	github.com/cncf/udpa/go v0.0.0-20200313221541-5f7e5dd04533 // indirect
	github.com/cockroachdb/apd v1.1.0 // indirect
	github.com/containers/storage v1.45.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/creack/pty v1.1.18 // indirect
	github.com/cyphar/filepath-securejoin v0.2.3 // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.6.3 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/envoyproxy/protoc-gen-validate v0.9.1 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gofrs/uuid v4.0.0+incompatible // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/pprof v0.0.0-20230406165453-00490a63f317 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/hashicorp/go-version v1.4.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jackc/fake v0.0.0-20150926172116-812a484cc733 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/klauspost/compress v1.16.0 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/minio/highwayhash v1.0.2 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/moby/sys/mountinfo v0.6.2 // indirect
	github.com/moby/term v0.0.0-20221205130635-1aeaba878587 // indirect
	github.com/nats-io/jwt/v2 v2.3.0 // indirect
	github.com/nats-io/nkeys v0.3.0 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/opencontainers/runc v1.1.4 // indirect
	github.com/opencontainers/runtime-spec v1.1.0-rc.1 // indirect
	github.com/openzipkin/zipkin-go v0.4.1 // indirect
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.step.sm/crypto v0.29.0 // indirect
	go.uber.org/automaxprocs v1.5.1 // indirect
	golang.org/x/mod v0.10.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/tools v0.8.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
