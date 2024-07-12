module code.cloudfoundry.org

<<<<<<< HEAD
go 1.23.0

toolchain go1.23.4
=======
go 1.22.0

<<<<<<< HEAD
toolchain go1.22.7
>>>>>>> 5b6155251 (Update go.mod dependencies)

=======
>>>>>>> 40bc964ac (Remove toolchain for compatibility)
replace (
	code.cloudfoundry.org/garden => ../garden
	code.cloudfoundry.org/grootfs => ../grootfs
	code.cloudfoundry.org/guardian => ../guardian
	code.cloudfoundry.org/idmapper => ../idmapper
	github.com/cyphar/filepath-securejoin => github.com/cyphar/filepath-securejoin v0.3.6

<<<<<<< HEAD
	// We have to pin this dep to v0.12.0 to remain compatible with Enovy 1.28 until Xenial is out of support
	// https://www.pivotaltracker.com/n/projects/2477027/stories/186946795
	github.com/envoyproxy/go-control-plane => github.com/envoyproxy/go-control-plane v0.12.0
=======
	github.com/nats-io/nats.go => github.com/nats-io/nats.go v1.16.1-0.20220906180156-a1017eec10b0
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.11.1
	github.com/prometheus/common => github.com/prometheus/common v0.30.0
	github.com/spf13/cobra => github.com/spf13/cobra v0.0.0-20160722081547-f62e98d28ab7
	github.com/zorkian/go-datadog-api => github.com/zorkian/go-datadog-api v0.0.0-20150915071709-8f1192dcd661
>>>>>>> 4bbab6a12 (WIP: protobuf updates)
)

require (
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
	code.cloudfoundry.org/archiver v0.26.0
	code.cloudfoundry.org/bytefmt v0.28.0
	code.cloudfoundry.org/cacheddownloader v0.0.0-20241210011823-7ae5910b9f48
	code.cloudfoundry.org/certsplitter v0.38.0
	code.cloudfoundry.org/cf-routing-test-helpers v0.0.0-20241025163157-ce30ff0fff6d
	code.cloudfoundry.org/cf-tcp-router v0.0.0-20241025163552-3216bbbc1656
	code.cloudfoundry.org/cfhttp/v2 v2.33.0
	code.cloudfoundry.org/clock v1.28.0
	code.cloudfoundry.org/cnbapplifecycle v0.0.5
<<<<<<< HEAD
<<<<<<< HEAD
	code.cloudfoundry.org/credhub-cli v0.0.0-20250203140928-329c0d63b022
	code.cloudfoundry.org/debugserver v0.36.0
	code.cloudfoundry.org/diego-logging-client v0.41.0
	code.cloudfoundry.org/dockerdriver v0.36.0
	code.cloudfoundry.org/durationjson v0.29.0
	code.cloudfoundry.org/eventhub v0.28.0
	code.cloudfoundry.org/garden v0.0.0-20250203211612-6a899d598abc
=======
	code.cloudfoundry.org/credhub-cli v0.0.0-20250106140724-966bccf2e72f
	code.cloudfoundry.org/debugserver v0.33.0
	code.cloudfoundry.org/diego-logging-client v0.37.0
	code.cloudfoundry.org/dockerdriver v0.32.0
	code.cloudfoundry.org/durationjson v0.26.0
	code.cloudfoundry.org/eventhub v0.25.0
<<<<<<< HEAD
	code.cloudfoundry.org/garden v0.0.0-20250108022507-4d85c9b08b69
>>>>>>> 2d8490891 (go mod tidy && go mod vendor)
=======
	code.cloudfoundry.org/garden v0.0.0-20250115235658-0e958c0cf159
>>>>>>> 6b5bfaa10 (go mod tidy && go mod vendor)
	code.cloudfoundry.org/go-loggregator/v9 v9.2.1
	code.cloudfoundry.org/goshims v0.59.0
	code.cloudfoundry.org/guardian v0.0.0-20250203212249-209fc11dcd6b
	code.cloudfoundry.org/lager/v3 v3.25.0
	code.cloudfoundry.org/localip v0.29.0
	code.cloudfoundry.org/tlsconfig v0.17.0
=======
	code.cloudfoundry.org/credhub-cli v0.0.0-20250120140441-b4f533288159
	code.cloudfoundry.org/debugserver v0.36.0
	code.cloudfoundry.org/diego-logging-client v0.39.0
	code.cloudfoundry.org/dockerdriver v0.34.0
	code.cloudfoundry.org/durationjson v0.27.0
	code.cloudfoundry.org/eventhub v0.26.0
	code.cloudfoundry.org/garden v0.0.0-20250204164024-2360009b9575
	code.cloudfoundry.org/go-loggregator/v9 v9.2.1
	code.cloudfoundry.org/goshims v0.57.0
	code.cloudfoundry.org/guardian v0.0.0-20250116175404-1165bf1e3152
	code.cloudfoundry.org/lager/v3 v3.25.0
	code.cloudfoundry.org/localip v0.29.0
	code.cloudfoundry.org/tlsconfig v0.16.0
>>>>>>> c6d5c71f1 (go mod tidy && go mod vendor)
	github.com/GaryBoone/GoStats v0.0.0-20130122001700-1993eafbef57
	github.com/ajstarks/svgo v0.0.0-20211024235047-1546f124cd8b
	github.com/aws/aws-sdk-go v1.55.6
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20250127194757-14dfc70fe71c
=======
	code.cloudfoundry.org/archiver v0.0.0-20240730181455-02f310de40a8
	code.cloudfoundry.org/bytefmt v0.0.0-20240730181512-d61d30bca0a4
=======
	code.cloudfoundry.org/archiver v0.0.0-20240804182113-00880bdce86d
	code.cloudfoundry.org/bytefmt v0.0.0-20240804182054-0a63f33a903d
>>>>>>> cf305e78a (Update go.mod dependencies)
=======
	code.cloudfoundry.org/archiver v0.0.0-20240806182217-21e72e4eb2fc
=======
	code.cloudfoundry.org/archiver v0.0.0-20240807182314-711f0e448f09
>>>>>>> 2e2edb3bb (Update go.mod dependencies)
	code.cloudfoundry.org/bytefmt v0.0.0-20240806182212-6cf545ebdd6b
>>>>>>> f1e791a7d (Update go.mod dependencies)
=======
	code.cloudfoundry.org/archiver v0.0.0-20240808182454-16f3064ad053
	code.cloudfoundry.org/bytefmt v0.0.0-20240808182453-a379845013d9
>>>>>>> eca10b02a (Update go.mod dependencies)
=======
	code.cloudfoundry.org/archiver v0.1.0
	code.cloudfoundry.org/bytefmt v0.1.0
>>>>>>> 12f5cfffc (Update go.mod dependencies)
=======
	code.cloudfoundry.org/archiver v0.2.0
	code.cloudfoundry.org/bytefmt v0.2.0
>>>>>>> bf1357502 (Update go.mod dependencies)
=======
	code.cloudfoundry.org/archiver v0.3.0
	code.cloudfoundry.org/bytefmt v0.3.0
>>>>>>> f9a0b31c2 (Update go.mod dependencies)
=======
	code.cloudfoundry.org/archiver v0.4.0
=======
	code.cloudfoundry.org/archiver v0.5.0
<<<<<<< HEAD
>>>>>>> c45717251 (Update go.mod dependencies)
	code.cloudfoundry.org/bytefmt v0.4.0
>>>>>>> d1f566753 (Update go.mod dependencies)
=======
	code.cloudfoundry.org/bytefmt v0.5.0
>>>>>>> ae4bc5334 (Update go.mod dependencies)
=======
	code.cloudfoundry.org/archiver v0.6.0
	code.cloudfoundry.org/bytefmt v0.6.0
>>>>>>> b93b7e30f (Update go.mod dependencies)
=======
	code.cloudfoundry.org/archiver v0.7.0
	code.cloudfoundry.org/bytefmt v0.7.0
>>>>>>> 51f3ccb88 (Update go.mod dependencies)
=======
	code.cloudfoundry.org/archiver v0.8.0
	code.cloudfoundry.org/bytefmt v0.8.0
>>>>>>> 9db0ff976 (Update go.mod dependencies)
	code.cloudfoundry.org/cacheddownloader v0.0.0-20240408163934-09b8631e33d0
	code.cloudfoundry.org/certsplitter v0.10.0
	code.cloudfoundry.org/cf-routing-test-helpers v0.0.0-20240821054706-b28ee5fb37eb
	code.cloudfoundry.org/cf-tcp-router v0.0.0-20240906112606-41037f0a7a20
	code.cloudfoundry.org/cfhttp/v2 v2.10.0
	code.cloudfoundry.org/clock v1.11.0
	code.cloudfoundry.org/cnbapplifecycle v0.0.2
	code.cloudfoundry.org/credhub-cli v0.0.0-20240906130858-907ed5401334
	code.cloudfoundry.org/debugserver v0.11.0
	code.cloudfoundry.org/diego-logging-client v0.16.0
	code.cloudfoundry.org/dockerdriver v0.9.0
	code.cloudfoundry.org/durationjson v0.9.0
	code.cloudfoundry.org/eventhub v0.8.0
	code.cloudfoundry.org/garden v0.0.0-20240906210158-d3ba7afc2097
	code.cloudfoundry.org/go-loggregator/v9 v9.2.1
	code.cloudfoundry.org/goshims v0.39.0
	code.cloudfoundry.org/guardian v0.0.0-20240906222318-4127d0dffd4d
	code.cloudfoundry.org/lager/v3 v3.3.0
	code.cloudfoundry.org/localip v0.9.0
	code.cloudfoundry.org/tlsconfig v0.4.0
=======
	code.cloudfoundry.org/archiver v0.10.0
=======
	code.cloudfoundry.org/archiver v0.11.0
>>>>>>> 59f9170a4 (Update go.mod dependencies)
	code.cloudfoundry.org/bytefmt v0.10.0
	code.cloudfoundry.org/cacheddownloader v0.0.0-20240916163925-a52cc2a85daa
	code.cloudfoundry.org/certsplitter v0.12.0
	code.cloudfoundry.org/cf-routing-test-helpers v0.0.0-20240913174203-3d9b0221d609
	code.cloudfoundry.org/cf-tcp-router v0.0.0-20240906112606-41037f0a7a20
	code.cloudfoundry.org/cfhttp/v2 v2.12.0
	code.cloudfoundry.org/clock v1.13.0
	code.cloudfoundry.org/cnbapplifecycle v0.0.2
	code.cloudfoundry.org/credhub-cli v0.0.0-20240916130534-17db04838e07
	code.cloudfoundry.org/debugserver v0.14.0
	code.cloudfoundry.org/diego-logging-client v0.20.0
	code.cloudfoundry.org/dockerdriver v0.14.0
	code.cloudfoundry.org/durationjson v0.11.0
	code.cloudfoundry.org/eventhub v0.10.0
	code.cloudfoundry.org/garden v0.0.0-20240916141541-4995881dc952
	code.cloudfoundry.org/go-loggregator/v9 v9.2.1
	code.cloudfoundry.org/goshims v0.39.0
	code.cloudfoundry.org/guardian v0.0.0-20240916141817-cd421f9135d0
	code.cloudfoundry.org/lager/v3 v3.6.0
	code.cloudfoundry.org/localip v0.11.0
	code.cloudfoundry.org/tlsconfig v0.5.0
>>>>>>> 1eda7d3ea (Update go.mod dependencies)
=======
	code.cloudfoundry.org/archiver v0.17.0
	code.cloudfoundry.org/bytefmt v0.14.0
	code.cloudfoundry.org/cacheddownloader v0.0.0-20241001173837-fdbfab074c13
	code.cloudfoundry.org/certsplitter v0.19.0
	code.cloudfoundry.org/cf-routing-test-helpers v0.0.0-20240920121531-99a9a4bd418e
	code.cloudfoundry.org/cf-tcp-router v0.0.0-20240920121448-5431d45dbc75
	code.cloudfoundry.org/cfhttp/v2 v2.17.0
	code.cloudfoundry.org/clock v1.19.0
	code.cloudfoundry.org/cnbapplifecycle v0.0.2
	code.cloudfoundry.org/credhub-cli v0.0.0-20241022172641-e8450bcc2ac7
	code.cloudfoundry.org/debugserver v0.22.0
	code.cloudfoundry.org/diego-logging-client v0.25.0
	code.cloudfoundry.org/dockerdriver v0.20.0
	code.cloudfoundry.org/durationjson v0.15.0
	code.cloudfoundry.org/eventhub v0.14.0
	code.cloudfoundry.org/garden v0.0.0-20241029172354-7b65841cf35a
	code.cloudfoundry.org/go-loggregator/v9 v9.2.1
	code.cloudfoundry.org/goshims v0.45.0
	code.cloudfoundry.org/guardian v0.0.0-20241018174554-d21cb5a146e1
	code.cloudfoundry.org/lager/v3 v3.13.0
	code.cloudfoundry.org/localip v0.17.0
	code.cloudfoundry.org/tlsconfig v0.7.0
>>>>>>> 15f76c672 (go mod tidy && go mod vendor)
	github.com/GaryBoone/GoStats v0.0.0-20130122001700-1993eafbef57
	github.com/ajstarks/svgo v0.0.0-20211024235047-1546f124cd8b
	github.com/aws/aws-sdk-go v1.55.5
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20240730143543-a8d7d3c42ca1
>>>>>>> 499451692 (Update go.mod dependencies)
=======
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20240807144652-27a1eeba0782
>>>>>>> 2e2edb3bb (Update go.mod dependencies)
=======
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20240808162302-0ed0161ffc3a
>>>>>>> eca10b02a (Update go.mod dependencies)
=======
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20240809155957-ac94a3401898
>>>>>>> 9cd9aa6e7 (Update go.mod dependencies)
=======
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20240814030307-d4ae6cf26e8b
>>>>>>> 4feb34b65 (Update go.mod dependencies)
=======
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20240823171036-ae5ff3e791a3
>>>>>>> f9a0b31c2 (Update go.mod dependencies)
=======
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20240826150212-5dc58b6e29f8
>>>>>>> 15c176a21 (Update go.mod dependencies)
=======
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20240905205714-e112906c5a2b
>>>>>>> 9db0ff976 (Update go.mod dependencies)
=======
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20240918142057-e21b7a4e92d1
>>>>>>> 59f9170a4 (Update go.mod dependencies)
	github.com/cactus/go-statsd-client v3.1.1-0.20161031215955-d8eabe07bc70+incompatible
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/cloudfoundry-community/go-uaa v0.3.3
	github.com/cloudfoundry/dropsonde v1.1.0
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/containers/image/v5 v5.34.0
=======
	github.com/containers/image/v5 v5.32.1
>>>>>>> 9cd9aa6e7 (Update go.mod dependencies)
=======
	github.com/containers/image/v5 v5.32.2
>>>>>>> a29b1afa1 (Update go.mod dependencies)
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7
<<<<<<< HEAD
	github.com/envoyproxy/go-control-plane v0.13.4
	github.com/fsnotify/fsnotify v1.8.0
=======
	github.com/envoyproxy/go-control-plane v0.13.0
	github.com/fsnotify/fsnotify v1.7.0
>>>>>>> 8ce727573 (Update go.mod dependencies)
	github.com/ghodss/yaml v1.0.0
	github.com/go-sql-driver/mysql v1.8.1
	github.com/go-test/deep v1.1.1
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/gogo/protobuf v1.3.2
	github.com/golang-jwt/jwt/v4 v4.5.1
=======
=======
	github.com/gogo/protobuf v1.3.2
>>>>>>> 59f9170a4 (Update go.mod dependencies)
	github.com/golang-jwt/jwt/v4 v4.5.0
>>>>>>> 1dfea0dee (Remove direct references to gogo/protobuf)
	github.com/golang/protobuf v1.5.4
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/hashicorp/errwrap v1.1.0
	github.com/hashicorp/go-multierror v1.1.1
<<<<<<< HEAD
	github.com/jackc/pgx/v5 v5.7.2
=======
	github.com/jackc/pgx/v5 v5.7.0
>>>>>>> f7c23ee10 (Update go.mod dependencies)
	github.com/jinzhu/gorm v1.9.16
	github.com/kr/pty v1.1.8
	github.com/lib/pq v1.10.9
	github.com/mitchellh/hashstructure v1.1.0
<<<<<<< HEAD
	github.com/moby/term v0.5.2
	github.com/nats-io/nats-server/v2 v2.10.25
	github.com/nats-io/nats.go v1.38.0
=======
	github.com/moby/term v0.5.0
	github.com/nats-io/nats-server/v2 v2.10.20
	github.com/nats-io/nats.go v1.37.0
>>>>>>> 12f5cfffc (Update go.mod dependencies)
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/onsi/ginkgo/v2 v2.22.2
	github.com/onsi/gomega v1.36.2
=======
	github.com/onsi/ginkgo/v2 v2.19.1
<<<<<<< HEAD
	github.com/onsi/gomega v1.34.0
>>>>>>> 0854e8485 (go mod tidy && go mod vendor)
=======
=======
	github.com/onsi/ginkgo/v2 v2.20.0
>>>>>>> 2e2edb3bb (Update go.mod dependencies)
=======
	github.com/onsi/ginkgo/v2 v2.20.1
>>>>>>> c4973fb9c (Update go.mod dependencies)
	github.com/onsi/gomega v1.34.1
>>>>>>> 499451692 (Update go.mod dependencies)
=======
	github.com/onsi/ginkgo/v2 v2.20.2
	github.com/onsi/gomega v1.34.2
>>>>>>> c45717251 (Update go.mod dependencies)
=======
	github.com/onsi/ginkgo/v2 v2.21.0
	github.com/onsi/gomega v1.35.0
>>>>>>> 15f76c672 (go mod tidy && go mod vendor)
	github.com/onsi/say v1.1.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.0
	github.com/openzipkin/zipkin-go v0.4.3
	github.com/pborman/getopt v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/pkg/sftp v1.13.7
	github.com/spf13/cobra v1.8.1
	github.com/square/certstrap v1.3.0
	github.com/tedsuo/ifrit v0.0.0-20230516164442-7862c310ad26
	github.com/tedsuo/rata v1.0.0
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/vito/go-sse v1.1.2
<<<<<<< HEAD
	golang.org/x/crypto v0.32.0
	golang.org/x/net v0.34.0
	golang.org/x/oauth2 v0.25.0
	golang.org/x/sys v0.30.0
	golang.org/x/time v0.9.0
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
	google.golang.org/grpc v1.70.0
	google.golang.org/protobuf v1.36.4
=======
	github.com/vito/go-sse v1.1.1
=======
	github.com/vito/go-sse v1.1.2
>>>>>>> a29b1afa1 (Update go.mod dependencies)
	golang.org/x/crypto v0.26.0
	golang.org/x/net v0.28.0
=======
	golang.org/x/crypto v0.27.0
	golang.org/x/net v0.29.0
>>>>>>> 9db0ff976 (Update go.mod dependencies)
	golang.org/x/oauth2 v0.23.0
	golang.org/x/sys v0.25.0
	golang.org/x/time v0.6.0
<<<<<<< HEAD
	google.golang.org/grpc v1.66.0
=======
	google.golang.org/grpc v1.66.2
>>>>>>> 58a961646 (Update go.mod dependencies)
	google.golang.org/protobuf v1.34.2
>>>>>>> cf305e78a (Update go.mod dependencies)
=======
	google.golang.org/grpc v1.69.2
	google.golang.org/protobuf v1.36.2
>>>>>>> 2d8490891 (go mod tidy && go mod vendor)
=======
	google.golang.org/grpc v1.69.4
	google.golang.org/protobuf v1.36.3
>>>>>>> 6b5bfaa10 (go mod tidy && go mod vendor)
=======
	google.golang.org/grpc v1.70.0
	google.golang.org/protobuf v1.36.4
>>>>>>> c6d5c71f1 (go mod tidy && go mod vendor)
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
	cel.dev/expr v0.19.2 // indirect
	code.cloudfoundry.org/commandrunner v0.26.0 // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20250120125122-6d632ec80998 // indirect
=======
	cel.dev/expr v0.15.0 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
	code.cloudfoundry.org/commandrunner v0.0.0-20240807162722-0c25ae4d419e // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20240730232652-ce6331b0e7c0 // indirect
>>>>>>> 499451692 (Update go.mod dependencies)
=======
	code.cloudfoundry.org/commandrunner v0.0.0-20240808162750-d70e1b3eb38a // indirect
=======
	code.cloudfoundry.org/commandrunner v0.0.0-20240809162919-eea6c4f85f53 // indirect
>>>>>>> 9cd9aa6e7 (Update go.mod dependencies)
	code.cloudfoundry.org/go-diodes v0.0.0-20240807231455-f9cf434a8c3e // indirect
>>>>>>> eca10b02a (Update go.mod dependencies)
=======
	cel.dev/expr v0.16.0 // indirect
	code.cloudfoundry.org/commandrunner v0.3.0 // indirect
=======
	cel.dev/expr v0.16.1 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
	code.cloudfoundry.org/commandrunner v0.4.0 // indirect
>>>>>>> c45717251 (Update go.mod dependencies)
=======
	code.cloudfoundry.org/commandrunner v0.5.0 // indirect
>>>>>>> ae4bc5334 (Update go.mod dependencies)
=======
	code.cloudfoundry.org/commandrunner v0.6.0 // indirect
>>>>>>> b93b7e30f (Update go.mod dependencies)
=======
	code.cloudfoundry.org/commandrunner v0.7.0 // indirect
>>>>>>> 51f3ccb88 (Update go.mod dependencies)
=======
	code.cloudfoundry.org/commandrunner v0.8.0 // indirect
>>>>>>> 5b6155251 (Update go.mod dependencies)
=======
=======
	cel.dev/expr v0.16.2 // indirect
>>>>>>> 59f9170a4 (Update go.mod dependencies)
	code.cloudfoundry.org/commandrunner v0.10.0 // indirect
<<<<<<< HEAD
>>>>>>> 1eda7d3ea (Update go.mod dependencies)
	code.cloudfoundry.org/go-diodes v0.0.0-20240813203737-5032edb05ceb // indirect
>>>>>>> 12f5cfffc (Update go.mod dependencies)
=======
	code.cloudfoundry.org/go-diodes v0.0.0-20240911205836-e7f77fdf9650 // indirect
>>>>>>> 58a961646 (Update go.mod dependencies)
=======
	cel.dev/expr v0.18.0 // indirect
	code.cloudfoundry.org/commandrunner v0.16.0 // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20241007161556-ec30366c7912 // indirect
>>>>>>> 15f76c672 (go mod tidy && go mod vendor)
=======
	cel.dev/expr v0.19.1 // indirect
<<<<<<< HEAD
	code.cloudfoundry.org/commandrunner v0.24.0 // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20241223074059-7f8c1f03edeb // indirect
>>>>>>> 2d8490891 (go mod tidy && go mod vendor)
=======
	code.cloudfoundry.org/commandrunner v0.27.0 // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20250120125122-6d632ec80998 // indirect
>>>>>>> c6d5c71f1 (go mod tidy && go mod vendor)
	filippo.io/edwards25519 v1.1.0 // indirect
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
	github.com/BurntSushi/toml v1.4.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/apex/log v1.9.0 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/aws/aws-sdk-go-v2 v1.36.0 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.29.4 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.57 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.27 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.31 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.31 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecr v1.40.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.31.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.24.14 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.28.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.12 // indirect
	github.com/aws/smithy-go v1.22.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bmizerany/pat v0.0.0-20210406213842-e4b6760bdd6f // indirect
	github.com/buildpacks/imgutil v0.0.0-20240605145725-186f89b2d168 // indirect
	github.com/buildpacks/lifecycle v0.20.5 // indirect
	github.com/buildpacks/pack v0.36.4 // indirect
=======
	github.com/aws/aws-sdk-go-v2 v1.30.3 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.27.27 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.27 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.15 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.15 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecr v1.32.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.25.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.11.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.11.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.22.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.26.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.30.3 // indirect
=======
	github.com/aws/aws-sdk-go-v2 v1.30.4 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.27.31 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.30 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.16 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.16 // indirect
=======
	github.com/aws/aws-sdk-go-v2 v1.30.5 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.27.35 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.33 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.13 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.17 // indirect
>>>>>>> b93b7e30f (Update go.mod dependencies)
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecr v1.34.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.25.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.11.4 // indirect
<<<<<<< HEAD
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.11.18 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.22.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.26.5 // indirect
<<<<<<< HEAD
	github.com/aws/aws-sdk-go-v2/service/sts v1.30.4 // indirect
>>>>>>> 8ce727573 (Update go.mod dependencies)
=======
	github.com/aws/aws-sdk-go-v2/service/sts v1.30.5 // indirect
>>>>>>> bf1357502 (Update go.mod dependencies)
=======
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.11.19 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/aws/aws-sdk-go-v2/service/sso v1.22.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.26.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.30.6 // indirect
>>>>>>> b93b7e30f (Update go.mod dependencies)
=======
	github.com/aws/aws-sdk-go-v2/service/sso v1.22.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.26.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.30.7 // indirect
>>>>>>> 51f3ccb88 (Update go.mod dependencies)
=======
	github.com/aws/aws-sdk-go-v2/service/sso v1.22.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.26.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.30.8 // indirect
>>>>>>> 59f9170a4 (Update go.mod dependencies)
	github.com/aws/smithy-go v1.20.4 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bmizerany/pat v0.0.0-20210406213842-e4b6760bdd6f // indirect
	github.com/buildpacks/imgutil v0.0.0-20240605145725-186f89b2d168 // indirect
	github.com/buildpacks/lifecycle v0.20.1 // indirect
	github.com/buildpacks/pack v0.35.1 // indirect
>>>>>>> be6fdb996 (Update go.mod dependencies)
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/chrismellard/docker-credential-acr-env v0.0.0-20230304212654-82a0ddb27589 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/cloudfoundry/go-socks5 v0.0.0-20240831012420-2590b55236ee // indirect
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/cloudfoundry/socks5-proxy v0.2.140 // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20250127102140-78b0e7da13b3 // indirect
	github.com/cncf/xds/go v0.0.0-20250121191232-2f005788dc42 // indirect
=======
	github.com/cloudfoundry/go-socks5 v0.0.0-20180221174514-54f73bdb8a8e // indirect
	github.com/cloudfoundry/socks5-proxy v0.2.121 // indirect
=======
	github.com/cloudfoundry/go-socks5 v0.0.0-20240831012420-2590b55236ee // indirect
<<<<<<< HEAD
	github.com/cloudfoundry/socks5-proxy v0.2.122 // indirect
>>>>>>> e495ba225 (Update go.mod dependencies)
=======
	github.com/cloudfoundry/socks5-proxy v0.2.123 // indirect
>>>>>>> f7c23ee10 (Update go.mod dependencies)
=======
	github.com/cloudfoundry/socks5-proxy v0.2.124 // indirect
>>>>>>> 59f9170a4 (Update go.mod dependencies)
	github.com/cloudfoundry/sonde-go v0.0.0-20240807231527-361c7ad33dc7 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/cncf/xds/go v0.0.0-20240723142845-024c85f92f20 // indirect
>>>>>>> eca10b02a (Update go.mod dependencies)
=======
	github.com/cncf/xds/go v0.0.0-20240822171458-6449f94b4d59 // indirect
>>>>>>> bf1357502 (Update go.mod dependencies)
=======
	github.com/cncf/xds/go v0.0.0-20240830210341-88aa3b3c978a // indirect
>>>>>>> 189ceb64d (Update go.mod dependencies)
=======
	github.com/cncf/xds/go v0.0.0-20240905190251-b4127c9b8d78 // indirect
>>>>>>> 9db0ff976 (Update go.mod dependencies)
=======
	github.com/cloudfoundry/socks5-proxy v0.2.137 // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20250113140334-595e96981704 // indirect
=======
	github.com/cloudfoundry/socks5-proxy v0.2.138 // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20250127102140-78b0e7da13b3 // indirect
>>>>>>> c6d5c71f1 (go mod tidy && go mod vendor)
	github.com/cncf/xds/go v0.0.0-20241223141626-cff3c89139a3 // indirect
>>>>>>> 2d8490891 (go mod tidy && go mod vendor)
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.16.3 // indirect
	github.com/containerd/typeurl/v2 v2.2.3 // indirect
	github.com/containers/libtrust v0.0.0-20230121012942-c1716e8a8d01 // indirect
	github.com/containers/ocicrypt v1.2.1 // indirect
	github.com/containers/storage v1.57.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
<<<<<<< HEAD
	github.com/creack/pty v1.1.24 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/cyphar/filepath-securejoin v0.4.1 // indirect
=======
	github.com/cyphar/filepath-securejoin v0.4.0 // indirect
>>>>>>> 6b5bfaa10 (go mod tidy && go mod vendor)
=======
	github.com/cyphar/filepath-securejoin v0.4.1 // indirect
>>>>>>> c6d5c71f1 (go mod tidy && go mod vendor)
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/cli v27.5.1+incompatible // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker v27.5.1+incompatible // indirect
=======
	github.com/creack/pty v1.1.23 // indirect
	github.com/cyphar/filepath-securejoin v0.3.2 // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/cli v27.2.0+incompatible // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
<<<<<<< HEAD
	github.com/docker/docker v27.1.2+incompatible // indirect
>>>>>>> 12f5cfffc (Update go.mod dependencies)
=======
	github.com/docker/docker v27.2.0+incompatible // indirect
>>>>>>> d1f566753 (Update go.mod dependencies)
	github.com/docker/docker-credential-helpers v0.8.2 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/go-units v0.5.0 // indirect
<<<<<<< HEAD
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
=======
	github.com/envoyproxy/protoc-gen-validate v1.1.0 // indirect
>>>>>>> be6fdb996 (Update go.mod dependencies)
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/google/go-containerregistry v0.20.3 // indirect
	github.com/google/pprof v0.0.0-20250202011525-fc3143867406 // indirect
<<<<<<< HEAD
=======
	github.com/google/go-containerregistry v0.20.2 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/google/pprof v0.0.0-20240727154555-813a5fbdbec8 // indirect
>>>>>>> f1e791a7d (Update go.mod dependencies)
=======
	github.com/google/pprof v0.0.0-20240827171923-fa2c70bbbfe5 // indirect
>>>>>>> d1f566753 (Update go.mod dependencies)
=======
	github.com/google/pprof v0.0.0-20240829160300-da1f7e9f2b25 // indirect
>>>>>>> ae4bc5334 (Update go.mod dependencies)
=======
	github.com/google/pprof v0.0.0-20240903155634-a8630aee4ab9 // indirect
>>>>>>> b93b7e30f (Update go.mod dependencies)
=======
	github.com/google/pprof v0.0.0-20241029153458-d1b30febd7db // indirect
>>>>>>> 15f76c672 (go mod tidy && go mod vendor)
=======
>>>>>>> c6d5c71f1 (go mod tidy && go mod vendor)
	github.com/google/uuid v1.6.0 // indirect
=======
	github.com/google/pprof v0.0.0-20240727154555-813a5fbdbec8 // indirect
>>>>>>> 0854e8485 (go mod tidy && go mod vendor)
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/heroku/color v0.0.6 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/minio/highwayhash v1.0.3 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/ioprogress v0.0.0-20180201004757-6a23b12fa88e // indirect
<<<<<<< HEAD
	github.com/moby/buildkit v0.19.0 // indirect
=======
	github.com/moby/buildkit v0.15.2 // indirect
>>>>>>> 8ce727573 (Update go.mod dependencies)
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/sys/capability v0.4.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
=======
>>>>>>> 6b5bfaa10 (go mod tidy && go mod vendor)
	github.com/moby/sys/reexec v0.1.0 // indirect
	github.com/moby/sys/user v0.3.0 // indirect
<<<<<<< HEAD
	github.com/moby/sys/userns v0.1.0 // indirect
<<<<<<< HEAD
=======
	github.com/moby/sys/user v0.3.0 // indirect
>>>>>>> eca10b02a (Update go.mod dependencies)
=======
>>>>>>> 12f5cfffc (Update go.mod dependencies)
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nats-io/jwt/v2 v2.7.3 // indirect
	github.com/nats-io/nkeys v0.4.9 // indirect
=======
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nats-io/jwt/v2 v2.7.0 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
>>>>>>> 1eda7d3ea (Update go.mod dependencies)
	github.com/nats-io/nuid v1.0.1 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/opencontainers/runc v1.2.4 // indirect
=======
	github.com/opencontainers/runc v1.1.14 // indirect
>>>>>>> b93b7e30f (Update go.mod dependencies)
=======
	github.com/opencontainers/runc v1.2.4 // indirect
>>>>>>> 2d8490891 (go mod tidy && go mod vendor)
	github.com/opencontainers/runtime-spec v1.2.0 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/prometheus/client_golang v1.20.5 // indirect
=======
=======
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
>>>>>>> 8ce727573 (Update go.mod dependencies)
	github.com/prometheus/client_golang v1.20.0 // indirect
>>>>>>> 4feb34b65 (Update go.mod dependencies)
=======
	github.com/prometheus/client_golang v1.20.1 // indirect
>>>>>>> a29b1afa1 (Update go.mod dependencies)
=======
	github.com/prometheus/client_golang v1.20.2 // indirect
>>>>>>> f9a0b31c2 (Update go.mod dependencies)
	github.com/prometheus/client_model v0.6.1 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/prometheus/common v0.62.0 // indirect
=======
	github.com/prometheus/common v0.57.0 // indirect
>>>>>>> c45717251 (Update go.mod dependencies)
=======
	github.com/prometheus/common v0.58.0 // indirect
>>>>>>> b93b7e30f (Update go.mod dependencies)
=======
	github.com/prometheus/client_golang v1.20.3 // indirect
=======
	github.com/prometheus/client_golang v1.20.4 // indirect
>>>>>>> 59f9170a4 (Update go.mod dependencies)
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.59.1 // indirect
>>>>>>> 9db0ff976 (Update go.mod dependencies)
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
=======
>>>>>>> c6d5c71f1 (go mod tidy && go mod vendor)
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/tonistiigi/go-csvvalue v0.0.0-20240814133006-030d3b2625d0 // indirect
	github.com/vbatts/tar-split v0.12.1 // indirect
	github.com/vishvananda/netlink v1.3.1-0.20240922070040-084abd93d350 // indirect
	github.com/vishvananda/netns v0.0.5 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.59.0 // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.34.0 // indirect
	go.step.sm/crypto v0.57.1 // indirect
	go.uber.org/automaxprocs v1.6.0 // indirect
<<<<<<< HEAD
	golang.org/x/exp v0.0.0-20250128182459-e0ece0dbea4c // indirect
	golang.org/x/sync v0.10.0 // indirect
=======
	golang.org/x/exp v0.0.0-20250106191152-7588d65b2ba8 // indirect
	golang.org/x/sync v0.11.0 // indirect
>>>>>>> c6d5c71f1 (go mod tidy && go mod vendor)
	golang.org/x/term v0.28.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	golang.org/x/tools v0.29.0 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
	google.golang.org/genproto/googleapis/api v0.0.0-20250127172529-29210b9bc287 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250127172529-29210b9bc287 // indirect
=======
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
<<<<<<< HEAD
	github.com/vishvananda/netlink v1.2.1-beta.2 // indirect
=======
	github.com/vbatts/tar-split v0.11.5 // indirect
<<<<<<< HEAD
	github.com/vishvananda/netlink v1.2.1 // indirect
>>>>>>> bf1357502 (Update go.mod dependencies)
=======
	github.com/vishvananda/netlink v1.3.0 // indirect
>>>>>>> f9a0b31c2 (Update go.mod dependencies)
	github.com/vishvananda/netns v0.0.4 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
	go.step.sm/crypto v0.48.1 // indirect
=======
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.53.0 // indirect
=======
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.54.0 // indirect
<<<<<<< HEAD
>>>>>>> 25a4f88bf (Update go.mod dependencies)
	go.opentelemetry.io/otel v1.29.0 // indirect
	go.opentelemetry.io/otel/metric v1.29.0 // indirect
	go.opentelemetry.io/otel/trace v1.29.0 // indirect
<<<<<<< HEAD
	go.step.sm/crypto v0.51.1 // indirect
>>>>>>> 499451692 (Update go.mod dependencies)
=======
	go.step.sm/crypto v0.51.2 // indirect
>>>>>>> 189ceb64d (Update go.mod dependencies)
=======
=======
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.55.0 // indirect
>>>>>>> 58a961646 (Update go.mod dependencies)
	go.opentelemetry.io/otel v1.30.0 // indirect
	go.opentelemetry.io/otel/metric v1.30.0 // indirect
	go.opentelemetry.io/otel/trace v1.30.0 // indirect
	go.step.sm/crypto v0.52.0 // indirect
>>>>>>> 1eda7d3ea (Update go.mod dependencies)
	go.uber.org/automaxprocs v1.5.3 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
	golang.org/x/exp v0.0.0-20240613232115-7f521ea00fb8 // indirect
	golang.org/x/sync v0.7.0 // indirect
=======
	golang.org/x/exp v0.0.0-20240719175910-8a7402abbf56 // indirect
=======
	golang.org/x/exp v0.0.0-20240808152545-0cdaa3abc0fa // indirect
>>>>>>> eca10b02a (Update go.mod dependencies)
=======
	golang.org/x/exp v0.0.0-20240822175202-778ce7bba035 // indirect
>>>>>>> bf1357502 (Update go.mod dependencies)
=======
	golang.org/x/exp v0.0.0-20240823005443-9b4947da3948 // indirect
>>>>>>> f9a0b31c2 (Update go.mod dependencies)
=======
	golang.org/x/exp v0.0.0-20240904232852-e7e105dedf7e // indirect
>>>>>>> 9db0ff976 (Update go.mod dependencies)
	golang.org/x/sync v0.8.0 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
	golang.org/x/term v0.22.0 // indirect
>>>>>>> cf305e78a (Update go.mod dependencies)
	golang.org/x/text v0.16.0 // indirect
	golang.org/x/tools v0.23.0 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
	google.golang.org/genproto/googleapis/api v0.0.0-20240701130421-f6361c86f094 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240701130421-f6361c86f094 // indirect
>>>>>>> 0854e8485 (go mod tidy && go mod vendor)
=======
	google.golang.org/genproto/googleapis/api v0.0.0-20240730163845-b1a4ccb954bf // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240730163845-b1a4ccb954bf // indirect
>>>>>>> 499451692 (Update go.mod dependencies)
=======
=======
	golang.org/x/term v0.23.0 // indirect
	golang.org/x/text v0.17.0 // indirect
=======
	golang.org/x/term v0.24.0 // indirect
	golang.org/x/text v0.18.0 // indirect
>>>>>>> 51f3ccb88 (Update go.mod dependencies)
	golang.org/x/tools v0.24.0 // indirect
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
>>>>>>> f1e791a7d (Update go.mod dependencies)
	google.golang.org/genproto/googleapis/api v0.0.0-20240805194559-2c9e96a0b5d4 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240805194559-2c9e96a0b5d4 // indirect
>>>>>>> be6fdb996 (Update go.mod dependencies)
=======
	google.golang.org/genproto/googleapis/api v0.0.0-20240808171019-573a1156607a // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240808171019-573a1156607a // indirect
>>>>>>> eca10b02a (Update go.mod dependencies)
=======
	google.golang.org/genproto/googleapis/api v0.0.0-20240812133136-8ffd90a71988 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240812133136-8ffd90a71988 // indirect
>>>>>>> 1761f51a6 (Update go.mod dependencies)
=======
	google.golang.org/genproto/googleapis/api v0.0.0-20240814211410-ddb44dafa142 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240814211410-ddb44dafa142 // indirect
>>>>>>> 8ce727573 (Update go.mod dependencies)
=======
	google.golang.org/genproto/googleapis/api v0.0.0-20240820151423-278611b39280 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240820151423-278611b39280 // indirect
>>>>>>> a29b1afa1 (Update go.mod dependencies)
=======
	google.golang.org/genproto/googleapis/api v0.0.0-20240822170219-fc7c04adadcd // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240822170219-fc7c04adadcd // indirect
>>>>>>> bf1357502 (Update go.mod dependencies)
=======
	google.golang.org/genproto/googleapis/api v0.0.0-20240823204242-4ba0660f739c // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240823204242-4ba0660f739c // indirect
>>>>>>> f9a0b31c2 (Update go.mod dependencies)
=======
	google.golang.org/genproto/googleapis/api v0.0.0-20240826202546-f6391c0de4c7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240826202546-f6391c0de4c7 // indirect
>>>>>>> 15c176a21 (Update go.mod dependencies)
=======
	google.golang.org/genproto/googleapis/api v0.0.0-20240827150818-7e3bb234dfed // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240827150818-7e3bb234dfed // indirect
>>>>>>> d1f566753 (Update go.mod dependencies)
=======
	google.golang.org/genproto/googleapis/api v0.0.0-20240903143218-8af14fe29dc1 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240903143218-8af14fe29dc1 // indirect
>>>>>>> b93b7e30f (Update go.mod dependencies)
=======
	google.golang.org/genproto/googleapis/api v0.0.0-20250106144421-5f5ef82da422 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
>>>>>>> 6b5bfaa10 (go mod tidy && go mod vendor)
=======
	google.golang.org/genproto/googleapis/api v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250204164813-702378808489 // indirect
>>>>>>> c6d5c71f1 (go mod tidy && go mod vendor)
)
