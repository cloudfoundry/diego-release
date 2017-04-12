$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

function absolute_path() {
  cd $1
  if ($?) {
    pwd
  }
}

cd diego-release/

$env:GOPATH=($pwd).path
$env:PATH = $env:GOPATH + "/bin;C:/go/bin;" + $env:PATH
Write-Host "Gopath is $GOPATH"
Write-Host "PATH is $PATH"

go install github.com/apcera/gnatsd
# go install github.com/coreos/etcd

Write-Host "Installing Ginkgo"
go install github.com/onsi/ginkgo/ginkgo
if ($LastExitCode -ne 0) {
    throw "Ginkgo installation process returned error code: $LastExitCode"
} else {
  Write-Host "Ginkgo successfully installed"
}

# scripts_path=$(absolute_path `dirname $0`)

# Write-Host "Setting environment variables..."
# $env:DIEGO_RELEASE_DIR=$(absolute_path $env:scripts_path/..)
# $env:CF_RELEASE_DIR=$(absolute_path $env:DIEGO_RELEASE_DIR/../cf-release)

# $env:CF_MANIFESTS_DIR=$env:CF_RELEASE_DIR + "/bosh-lite/deployments"
# $env:DIEGO_MANIFESTS_DIR=$env:DIEGO_RELEASE_DIR + "/bosh-lite/deployments"

# Write-Host $env:DIEGO_RELEASE_DIR=$env:DIEGO_RELEASE_DIR
# Write-Host $env:CF_RELEASE_DIR=$env:CF_RELEASE_DIR
# Write-Host $env:CF_MANIFESTS_DIR=$env:CF_MANIFESTS_DIR
# Write-Host $env:DIEGO_MANIFESTS_DIR=$env:DIEGO_MANIFESTS_DIR


Write-Host "Running store-dependent test suites against a MySQL database..."
$env:DB_UNITS="./bbs/db/sqldb"
$env:SQL_FLAVOR=mysql

cd src/code.cloudfoundry.org/

ginkgo -r -keepGoing -p -trace -randomizeAllSpecs -progress --race rep/cmd/rep rep/generator/internal route-emitter/cmd/route-emitter bbs/db/sqldb
    # rep/cmd/rep \
    # rep/generator/internal \
    # route-emitter/cmd/route-emitter \
    # ./bbs/db/sqldb



# $scripts_path/run-unit-tests-no-backing-store
# let ERROR_CODE+=$?

if ($LastExitCode -ne 0) {
    Write-Host "Diego unit tests failed"
    exit 1
} else {
  Write-Host "Diego unit tests passed"
  exit 0
}

