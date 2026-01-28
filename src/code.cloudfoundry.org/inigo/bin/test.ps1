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
  ipconfig /flushdns
}

function Setup-TempDirContainerAccess() {
  Write-Host "Setup-TempDirContainerAccess"
    $rule = New-Object System.Security.AccessControl.FileSystemAccessRule("Users", "ReadAndExecute", "ContainerInherit, ObjectInherit", "None", "Allow")
    $acl = Get-Acl "$env:TEMP"
    $acl.AddAccessRule($rule)
    Set-Acl "$env:TEMP" $acl
}

function Setup-GardenRunc() {
$env:GARDEN_BINPATH="$env:GARDEN_RUNC_RELEASE_PATH/bin"
$env:GROOTFS_BINPATH="$env:GARDEN_BINPATH"
$env:GROOTFS_STORE_PATH="$env:GROOT_IMAGE_STORE"

mkdir -Force "$env:GARDEN_BINPATH"
cp "$env:WINIT_BINARY" "$env:GARDEN_BINPATH/init.exe"
cp "$env:NSTAR_BINARY" "$env:GARDEN_BINPATH/nstar.exe"
cp "$env:GROOT_BINARY" "$env:GARDEN_BINPATH/grootfs.exe"
cp "$env:GROOT_QUOTA_DLL" "$env:GARDEN_BINPATH/quota.dll"
cp "$env:WINC_BINARY" "$env:GARDEN_BINPATH/winc.exe"
cp "$env:WINC_NETWORK_BINARY" "$env:GARDEN_BINPATH/winc-network.exe"
$tarPath = (Get-Command tar).Source
cp "${tarPath}" "${env:GARDEN_BINPATH}/tar.exe"
}

function Setup-Database() {
  Write-Host "Setup-Database"

  $origCaFile="$env:DIEGO_RELEASE_PATH\src\code.cloudfoundry.org\inigo\fixtures\certs\sql-certs\server-ca.crt"
  $origCertFile="$env:DIEGO_RELEASE_PATH\src\code.cloudfoundry.org\inigo\fixtures\certs\sql-certs\server.crt"
  $origKeyFile="$env:DIEGO_RELEASE_PATH\src\code.cloudfoundry.org\inigo\fixtures\certs\sql-certs\server.key"

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
ssl-ca=$caFile
max_connections=1000"

  Restart-Service Mysql
}

$timestamp="$((get-date).ToUniversalTime().ToString('yyyy-MM-ddTHH-mm-ss'))"
$logsDir="$env:TMP/inigo-logs-$timestamp"
echo "Log Dir: $logsDir"
mkdir -Force "$logsDir"

Setup-GardenRunc
Configure-Groot "$env:GARDEN_TEST_ROOTFS"
Configure-Winc-Network "delete"
Configure-Winc-Network "create"
Set-NetFirewallProfile -All -DefaultInboundAction Block -DefaultOutboundAction Allow -Enabled True
Setup-Database
Setup-DnsNames
Setup-TempDirContainerAccess

$env:CODE_CLOUDFOUNDRY_ORG_MODULE="$env:DIEGO_RELEASE_PATH/src/code.cloudfoundry.org"
$env:GUARDIAN_MODULE="$env:DIEGO_RELEASE_PATH/src/guardian"
$env:ROUTER_GOPATH="$env:ROUTING_RELEASE_PATH\src\code.cloudfoundry.org"
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

# used for routing to apps; same logic that Garden uses.
$ipAddressObject = Find-NetRoute -RemoteIPAddress "8.8.8.8" | Select-Object IpAddress
$ipAddress = $ipAddressObject.IpAddress
$env:EXTERNAL_ADDRESS="$ipAddress".Trim()

Invoke-Expression "go run github.com/onsi/ginkgo/v2/ginkgo $args --output-dir $logsDir --json-report report.json"
if ($LastExitCode -ne 0) {
  throw "tests failed"
}
