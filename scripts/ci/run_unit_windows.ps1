$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

cd diego-release/

Add-Type -AssemblyName System.IO.Compression.FileSystem
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$CONSUL_DIR = "C:/consul"
# Remove-Item $CONSUL_DIR -Force
if(!(Test-Path -Path $CONSUL_DIR )) {
    New-Item -ItemType directory -Path $CONSUL_DIR
    (New-Object System.Net.WebClient).DownloadFile('https://releases.hashicorp.com/consul/0.7.0/consul_0.7.0_windows_amd64.zip', "$CONSUL_DIR/consul.zip")
    [System.IO.Compression.ZipFile]::ExtractToDirectory("$CONSUL_DIR/consul.zip", "$CONSUL_DIR")
}

$env:GOPATH=($pwd).path
$env:PATH = $env:GOPATH + "/bin;C:/go/bin;" + $CONSUL_DIR + ";" + $env:PATH
# Write-Host "Gopath is " + $env:GOPATH
# Write-Host "PATH is " + $env:PATH

go install github.com/apcera/gnatsd
go install github.com/coreos/etcd

Write-Host "Installing Ginkgo"
go install github.com/onsi/ginkgo/ginkgo
if ($LastExitCode -ne 0) {
    throw "Ginkgo installation process returned error code: $LastExitCode"
} else {
  Write-Host "Ginkgo successfully installed"
}

Write-Host "Running store-independent test suites against a MySQL database..."
$env:SQL_FLAVOR="mysql"

cd src/code.cloudfoundry.org/

# $env:SKIP_PACKAGES=route-emitter/routingtable/benchmarks

ginkgo -p -r -skipPackage=$env:SKIP_PACKAGES -keepGoing -trace -randomizeAllSpecs -progress --race `
  cfhttp `
  executor `
  bytefmt `
  durationjson `
  eventhub `
  localip `
  operationq `
  rep `
  routing-info `
  workpool

# TODO: These suites do not work yet.
# ginkgo -r -skipPackage=$env:SKIP_PACKAGES -keepGoing -trace -randomizeAllSpecs -progress --race cacheddownloader
# ginkgo -r -skipPackage=$env:SKIP_PACKAGES -keepGoing -trace -randomizeAllSpecs -progress --race cfhttp
# ginkgo -r -skipPackage=$env:SKIP_PACKAGES -keepGoing -trace -randomizeAllSpecs -progress --race route-emitter

if ($LastExitCode -ne 0) {
  Write-Host "Diego unit tests failed"
  exit 1
} else {
  Write-Host "Diego unit tests passed"
  exit 0
}
