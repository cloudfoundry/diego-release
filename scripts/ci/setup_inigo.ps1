$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

$env:GOROOT=(Get-ChildItem "C:\var\vcap\packages\golang-*-windows\go").FullName
$env:PATH= "$env:GOROOT\bin;$env:PATH"
$env:TMP = "C:\var\vcap\data\tmp"
$env:TEMP = "C:\var\vcap\data\tmp"

function Setup-DnsNames() {
  Write-Host "Setup-DnsNames"
  Add-Content -Path "C:\Windows\System32\Drivers\etc\hosts" -Encoding ASCII -Value "
127.0.0.1 the-cell-id-1-0.cell.service.cf.internal
127.0.0.1 the-cell-id-2-0.cell.service.cf.internal
127.0.0.1 the-cell-id-3-0.cell.service.cf.internal
127.0.0.1 the-cell-id-4-0.cell.service.cf.internal
"
}

function Setup-TempDirContainerAccess() {
  Write-Host "Setup-TempDirContainerAccess"
  $rule = New-Object System.Security.AccessControl.FileSystemAccessRule("Users", "ReadAndExecute", "ContainerInherit, ObjectInherit", "None", "Allow")
  $acl = Get-Acl "$env:TEMP"
  $acl.AddAccessRule($rule)
  Set-Acl "$env:TEMP" $acl
}

function Build-GardenRunc(){
  param([string] $gardenRuncDir, [string] $wincReleaseDir)

  Write-Host "Building garden-runc-release"
  $env:GARDEN_RUNC_PATH = $gardenRuncDir
  $env:WINC_RELEASE_PATH = $wincReleaseDir

  push-location $env:GARDEN_RUNC_PATH
    $env:PATH = "$PWD\bin;$env:PATH"
    $env:GARDEN_BINPATH = "$PWD\bin"

    mkdir -Force "$env:GARDEN_RUNC_PATH\bin"

    $tarPath = (Get-Command tar).source
    cp -ErrorAction SilentlyContinue -Force $tarPath "$env:GARDEN_BINPATH"

    push-location ".\src\guardian"
      go build -mod vendor -o "$env:GARDEN_BINPATH\init.exe" ".\cmd\winit"
      if ($LastExitCode -ne 0) {
	throw "Building init.exe process returned error code: $LastExitCode"
      }
    pop-location
  pop-location


  push-location $env:WINC_RELEASE_PATH
    Write-Host "Building winc-release"
    mkdir -Force "$env:WINC_RELEASE_PATH\bin"

    $env:GROOTFS_BINPATH = "$env:GARDEN_BINPATH"

    $env:PATH = "$env:PATH;C:\var\vcap\packages\mingw64\mingw64\bin"

    $env:GOPATH="$PWD"

    push-location "src/nstar"
    go build -o "$env:GARDEN_BINPATH\nstar.exe" .
    if ($LastExitCode -ne 0) {
      throw "Building nstar.exe process returned error code: $LastExitCode"
    }

    pop-location

    push-location "src/code.cloudfoundry.org/groot-windows"
    go build -o "$env:GROOTFS_BINPATH\grootfs.exe" .
    if ($LastExitCode -ne 0) {
      throw "Building grootfs.exe process returned error code: $LastExitCode"
    }
    pop-location

    gcc -c ".\src\code.cloudfoundry.org\groot-windows\volume\quota\quota.c" -o "$env:GROOTFS_BINPATH\quota.o"
    if ($LastExitCode -ne 0) {
      throw "Building quota.o process returned error code: $LastExitCode"
    }

    gcc -shared -o "$env:GROOTFS_BINPATH\quota.dll" "$env:GROOTFS_BINPATH\quota.o" -lole32 -loleaut32
    if ($LastExitCode -ne 0) {
      throw "Building quota.dll process returned error code: $LastExitCode"
    }

    push-location "src/code.cloudfoundry.org/winc"
    go build -o "$env:GARDEN_BINPATH\winc.exe" "./cmd/winc"
    if ($LastExitCode -ne 0) {
      throw "Building winc.exe process returned error code: $LastExitCode"
    }

    go build -o "$env:GARDEN_BINPATH\winc-network.exe" -tags "hnsAcls" "./cmd/winc-network"
    if ($LastExitCode -ne 0) {
      throw "Building winc-network.exe process returned error code: $LastExitCode"
    }
    pop-location

  pop-location
}

function Set-GardenRootfs() {
  Write-Host "Set-GardenRootfs"
  if (-not (Test-Path 'env:GARDEN_ROOTFS')) {
    throw "Please set GARDEN_ROOTFS environment variable"
  }
  $env:GROOTFS_STORE_PATH="C:\grootfs-store"
  & "$env:GROOTFS_BINPATH\grootfs.exe" --driver-store "$env:GROOTFS_STORE_PATH" pull "$env:GARDEN_ROOTFS"
  if ($LastExitCode -ne 0) {
    throw "Pulling $env:GARDEN_ROOTFS returned error code: $LastExitCode"
  }
}

function Setup-ContainerNetworking() {
  Write-Host "Setup-ContainerNetworking"
  Set-Content -Path "C:\winc-network.json" -Value '{
  "network_name": "winc-nat",
  "subnet_range": "172.30.0.0/22",
  "gateway_address": "172.30.0.1"
}'

  & "$env:GARDEN_BINPATH\winc-network.exe" --debug --log-format json --action delete --configFile "C:\winc-network.json"
  if ($LASTEXITCODE -ne 0) {
    throw "Deleting container network returned error code: $LastExitCode"
  }

  & "$env:GARDEN_BINPATH\winc-network.exe" --debug --log-format json --action create --configFile "C:\winc-network.json"
  if ($LASTEXITCODE -ne 0) {
    throw "Creating container network returned error code: $LastExitCode"
  }

  Set-NetFirewallProfile -All -DefaultInboundAction Block -DefaultOutboundAction Allow -Enabled True
}

function Setup-Gopath() {
  param([string] $dir)

  Write-Host "Setup-Gopath"
  Push-Location $dir
    $env:GOPATH_ROOT="$PWD"

    $env:CODE_CLOUDFOUNDRY_ORG_MODULE="$env:GOPATH_ROOT/src/code.cloudfoundry.org"
    $env:GUARDIAN_MODULE="$env:GOPATH_ROOT/src/guardian"

    $env:GOPATH="${env:GOPATH_ROOT}"
    $env:GOBIN="${env:GOPATH_ROOT}/bin"
    $env:PATH="${env:GOBIN};${env:PATH}"

    Push-Location "./src/code.cloudfoundry.org"
      echo "Installing nats-server ..."
      go build -o "$env:GOBIN/nats-server.exe" github.com/nats-io/nats-server/v2
      $env:NATS_DOCKERIZED = "1"
      echo "Installing ginkgo ..."
      go build -o "$env:GOBIN/ginkgo.exe" github.com/onsi/ginkgo/ginkgo
      echo "Installing consul ..."
      go build -o "$env:GOBIN/consul.exe" github.com/hashicorp/consul
    Pop-Location
  Pop-Location
}

function Setup-Envoy() {
 param([string] $envoyReleaseDir)

  Write-Host "Setup-Envoy"
  Push-Location $envoyReleaseDir
    bosh sync-blobs
    if ($LastExitCode -ne 0) {
      throw "Syncing envoy bosh blobs returned error code: $LastExitCode"
    }

    $env:ENVOY_PATH="$env:TEMP\envoy"
    mkdir -Force "$env:ENVOY_PATH"

    Expand-Archive -Force -Path "blobs\envoy-nginx\envoy-nginx*.zip" -DestinationPath "$env:ENVOY_PATH"

    push-location "src/code.cloudfoundry.org/envoy-nginx"
    go build -o "$env:ENVOY_PATH\envoy.exe" .
    if ($LastExitCode -ne 0) {
      throw "Building envoy.exe process returned error code: $LastExitCode"
    }
    pop-location

  Pop-Location
}

function Setup-Database() {
  Write-Host "Setup-Database"

  $origCaFile="$env:GOPATH_ROOT\src\code.cloudfoundry.org\inigo\fixtures\certs\sql-certs\server-ca.crt"
  $origCertFile="$env:GOPATH_ROOT\src\code.cloudfoundry.org\inigo\fixtures\certs\sql-certs\server.crt"
  $origKeyFile="$env:GOPATH_ROOT\src\code.cloudfoundry.org\inigo\fixtures\certs\sql-certs\server.key"

  $mysqlCertsDir = "$env:TEMP\mysql-certs" -replace '\\','\\'
  mkdir -Force $mysqlCertsDir

  $caFile="$mysqlCertsDir\\server-ca.crt"
  $certFile="$mysqlCertsDir\\server.crt"
  $keyFile="$mysqlCertsDir\\server.key"

  cp $origCaFile $caFile
  cp $origCertFile $certFile
  cp $origKeyFile $keyFile

  $mySqlBaseDir=(Get-ChildItem "C:\var\vcap\packages\mysql\mysql-*").FullName

  Set-Content -Path "$mySqlBaseDir\my.ini" -Encoding Ascii -Value "[mysqld]
basedir=$mySqlBaseDir
datadir=C:\\var\\vcap\\data\\mysql
ssl-cert=$certFile
ssl-key=$keyFile
ssl-ca=$caFile"

  Restart-Service Mysql
}

Remove-Item -Recurse -Force -ErrorAction Ignore $PWD/diego-release/src/code.cloudfoundry.org/guardian/vendor/github.com/onsi/ginkgo
Remove-Item -Recurse -Force -ErrorAction Ignore $PWD/diego-release/src/code.cloudfoundry.org/guardian/vendor/github.com/onsi/gomega

Build-GardenRunc "$PWD\garden-runc-release" "$PWD\winc-release"
Setup-Envoy "$PWD/envoy-nginx-release"
Setup-Gopath "$PWD/diego-release"
Set-GardenRootfs
Setup-ContainerNetworking
Setup-Database
Setup-DnsNames
Setup-TempDirContainerAccess

$env:ROUTER_GOPATH="$PWD\routing-release"
$env:ROUTING_API_GOPATH=$env:ROUTER_GOPATH
$env:APP_LIFECYCLE_GOPATH=${env:CODE_CLOUDFOUNDRY_ORG_MODULE}
$env:AUCTIONEER_GOPATH=${env:CODE_CLOUDFOUNDRY_ORG_MODULE}
$env:BBS_GOPATH=${env:CODE_CLOUDFOUNDRY_ORG_MODULE}
$env:FILE_SERVER_GOPATH=${env:CODE_CLOUDFOUNDRY_ORG_MODULE}
$env:HEALTHCHECK_GOPATH=${env:CODE_CLOUDFOUNDRY_ORG_MODULE}
$env:LOCKET_GOPATH=${env:CODE_CLOUDFOUNDRY_ORG_MODULE}
$env:REP_GOPATH=${env:CODE_CLOUDFOUNDRY_ORG_MODULE}
$env:ROUTE_EMITTER_GOPATH=${env:CODE_CLOUDFOUNDRY_ORG_MODULE}
$env:SSHD_GOPATH=${env:CODE_CLOUDFOUNDRY_ORG_MODULE}
$env:SSH_PROXY_GOPATH=${env:CODE_CLOUDFOUNDRY_ORG_MODULE}
$env:GARDEN_GOPATH=${env:GUARDIAN_MODULE}

$env:DIEGO_RELEASE_DIR=$env:GOPATH

# used for routing to apps; same logic that Garden uses.
$ipAddressObject = Find-NetRoute -RemoteIPAddress "8.8.8.8" | Select-Object IpAddress
$ipAddress = $ipAddressObject.IpAddress
$env:EXTERNAL_ADDRESS="$ipAddress".Trim()

