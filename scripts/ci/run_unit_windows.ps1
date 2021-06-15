$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

$env:DIEGO_RELEASE_DIR = Resolve-Path -Path $pwd/diego-release/ | select -ExpandProperty Path
cd diego-release/

Add-Type -AssemblyName System.IO.Compression.FileSystem
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12


Push-Location "$env:DIEGO_RELEASE_DIR/src/code.cloudfoundry.org"
  $NATS_DIR = "C:\nats-server"
  Write-Host "Installing nats-server ..."
  go build -o "$NATS_DIR/nats-server.exe" github.com/nats-io/nats-server/v2
  $env:NATS_DOCKERIZED = "1"
  $env:NATS_DOCKERIZED = "1"
  $CONSUL_DIR = "C:\consul"
  Write-Host "Installing consul ..."
  go build -o "$CONSUL_DIR/consul.exe" github.com/hashicorp/consul
  $env:PATH += ";$NATS_DIR;$CONSUL_DIR"
Pop-Location

Write-Host "Downloading winpty DLL"
$WINPTY_DIR = "C:\winpty"
if(!(Test-Path -Path $WINPTY_DIR )) {
    New-Item -ItemType directory -Path $WINPTY_DIR
    (New-Object System.Net.WebClient).DownloadFile('https://github.com/rprichard/winpty/releases/download/0.4.3/winpty-0.4.3-msvc2015.zip', "$WINPTY_DIR\winpty.zip")
    [System.IO.Compression.ZipFile]::ExtractToDirectory("$WINPTY_DIR\winpty.zip", "$WINPTY_DIR")
}
$env:WINPTY_DLL_DIR="$WINPTY_DIR\x64\bin"


Write-Host "CONTAINER DEBUG INFO"
Write-Host "DIEGO_RELEASE_DIR = $env:DIEGO_RELEASE_DIR"
Write-Host "PATH = $env:PATH"



Write-Host "Running store-independent test suites against a MySQL database..."
$env:SQL_FLAVOR="mysql"

cd src/code.cloudfoundry.org/

go run github.com/onsi/ginkgo/ginkgo -r -keepGoing -trace -randomizeAllSpecs -progress -race `
  route-emitter `
  cfhttp `
  cacheddownloader `
  certsplitter `
  diego-logging-client `
  diego-ssh `
  executor `
  bytefmt `
  durationjson `
  eventhub `
  healthcheck `
  localip `
  operationq `
  rep `
  routing-info `
  workpool

if ($LastExitCode -ne 0) {
  Write-Host "Diego unit tests failed"
  exit 1
} else {
  Write-Host "Diego unit tests passed"
  exit 0
}
