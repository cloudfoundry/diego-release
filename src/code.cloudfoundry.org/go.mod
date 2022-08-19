module code.cloudfoundry.org

go 1.16

replace (
	code.cloudfoundry.org/garden => ../garden
	code.cloudfoundry.org/grootfs => ../grootfs
	code.cloudfoundry.org/guardian => ../guardian
	code.cloudfoundry.org/idmapper => ../idmapper

	code.cloudfoundry.org/lager => code.cloudfoundry.org/lager v1.1.1-0.20210513163233-569157d2803b
	github.com/docker/docker => github.com/docker/docker v20.10.13+incompatible
	github.com/envoyproxy/go-control-plane => github.com/envoyproxy/go-control-plane v0.9.5
	github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf => github.com/golang/protobuf v1.3.2
	github.com/hashicorp/consul => github.com/hashicorp/consul v0.7.0
	github.com/nats-io/nats.go => github.com/nats-io/nats.go v1.9.1
	github.com/onsi/gomega => github.com/onsi/gomega v1.7.0
	github.com/spf13/cobra => github.com/spf13/cobra v0.0.0-20160722081547-f62e98d28ab7
	github.com/zorkian/go-datadog-api => github.com/zorkian/go-datadog-api v0.0.0-20150915071709-8f1192dcd661
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20180817151627-c66870c02cf8
	google.golang.org/grpc => google.golang.org/grpc v1.29.0-dev.0.20200306163155-d179e8f5cd96
)

require (
	code.cloudfoundry.org/archiver v0.0.0-20210609160716-67523bd33dbf
	code.cloudfoundry.org/certsplitter v0.0.0-20210609184434-2de18d24e399
	code.cloudfoundry.org/cf-routing-test-helpers v0.0.0-20200827173955-6ac4653025b4
	code.cloudfoundry.org/cf-tcp-router v0.0.0-20210401153245-254a1ad317ab
	code.cloudfoundry.org/cfhttp v1.0.1-0.20210513172332-4c5ee488a657
	code.cloudfoundry.org/cfhttp/v2 v2.0.1-0.20210513172332-4c5ee488a657
	code.cloudfoundry.org/clock v1.0.1-0.20210513171101-3765e64694c4
	code.cloudfoundry.org/consuladapter v0.0.0-20211122211027-9dbbfa656ee0
	code.cloudfoundry.org/credhub-cli v0.0.0-20220228140414-459eb2d27a1c
	code.cloudfoundry.org/debugserver v0.0.0-20210608171006-d7658ce493f4
	code.cloudfoundry.org/diego-logging-client v0.0.0-20220819172429-0486fc549e79
	code.cloudfoundry.org/durationjson v0.0.0-20210615172401-3a89d41c90da
	code.cloudfoundry.org/eventhub v0.0.0-20210615172938-0b896ce72257
	code.cloudfoundry.org/garden v0.0.0-20210608104724-fa3a10d59c82
	code.cloudfoundry.org/go-diodes v0.0.0-20220725190411-383eb6634c40 // indirect
	code.cloudfoundry.org/go-loggregator/v8 v8.0.5
	code.cloudfoundry.org/guardian v0.0.0-00010101000000-000000000000
	code.cloudfoundry.org/lager v2.0.0+incompatible
	code.cloudfoundry.org/localip v0.0.0-20210608161955-43c3ec713c20
	code.cloudfoundry.org/tlsconfig v0.0.0-20220621140725-0e6fbd869921
	github.com/GaryBoone/GoStats v0.0.0-20130122001700-1993eafbef57
	github.com/ajstarks/svgo v0.0.0-20210406150507-75cfd577ce75
	github.com/armon/go-metrics v0.3.10 // indirect
	github.com/aws/aws-sdk-go v1.43.11
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20210324191134-efd1603705e9
	github.com/cactus/go-statsd-client v3.1.1-0.20161031215955-d8eabe07bc70+incompatible
	github.com/cloudfoundry/dropsonde v1.0.0
	github.com/cockroachdb/apd v1.1.0 // indirect
	github.com/containers/image v3.0.2+incompatible
	github.com/docker/docker v20.10.17+incompatible
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7
	github.com/envoyproxy/go-control-plane v0.9.4
	github.com/fortytw2/leaktest v1.3.0
	github.com/fsnotify/fsnotify v1.5.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/go-test/deep v1.0.7
	github.com/gofrs/uuid v4.0.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.2
	github.com/golang-jwt/jwt/v4 v4.1.0
	github.com/golang/protobuf v1.5.2
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/hashicorp/consul v1.11.4
	github.com/hashicorp/errwrap v1.1.0
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/serf v0.9.7 // indirect
	github.com/jackc/fake v0.0.0-20150926172116-812a484cc733 // indirect
	github.com/jackc/pgx v3.6.2+incompatible
	github.com/jinzhu/gorm v1.9.16
	github.com/kr/pty v1.1.8
	github.com/lib/pq v1.10.1
	github.com/mitchellh/cli v1.1.2 // indirect
	github.com/mitchellh/hashstructure v1.1.0
	github.com/nats-io/nats-server/v2 v2.2.6
	github.com/nats-io/nats.go v1.11.0
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.20.0
	github.com/onsi/say v1.0.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.3-0.20211202193544-a5463b7f9c84
	github.com/pborman/getopt v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/pkg/sftp v1.13.0
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/spf13/cobra v1.3.0
	github.com/square/certstrap v1.2.0
	github.com/tedsuo/ifrit v0.0.0-20191009134036-9a97d0632f00
	github.com/tedsuo/rata v1.0.0
	github.com/vito/go-sse v1.0.0
	github.com/zorkian/go-datadog-api v2.30.0+incompatible
	golang.org/x/crypto v0.0.0-20220214200702-86341886e292
	golang.org/x/net v0.0.0-20220812174116-3211cb980234
	golang.org/x/sys v0.0.0-20220818161305-2296e01440c6
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac
	google.golang.org/genproto v0.0.0-20220819153447-c7cd466b0e09 // indirect
	google.golang.org/grpc v1.48.0
	gopkg.in/asn1-ber.v1 v1.0.0-20181015200546-f715ec2f112d // indirect
	gopkg.in/ldap.v2 v2.5.1
	gopkg.in/yaml.v2 v2.4.0
)
