$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

$dir=[System.IO.Path]::GetDirectoryName($PSScriptRoot)
. $dir/ci/setup_inigo.ps1

 $nodes_flag=""
 if ( "$env:GINKGO_PARALLEL" -eq "true" ) {
   $nodes_flag="-nodes=4"
 }

 Push-Location "${env:GOPATH_ROOT}\src\code.cloudfoundry.org\inigo"
   $PACKAGES_TO_SKIP="docker"

   if ( "$env:SKIP_PACKAGES" -ne "" ) {
     $PACKAGES_TO_SKIP=$PACKAGES_TO_SKIP + "," + $env:SKIP_PACKAGES
   }

   ginkgo $nodes_flag -r -skipPackage="${PACKAGES_TO_SKIP}" -skip="${env:SKIP_REGEX}" -failOnPending -randomizeAllSpecs -trace -race -slowSpecThreshold=60 -keepGoing

   if ($LASTEXITCODE -ne 0) {
      Write-Host "Failed to run inigo"
      exit 1
   }

 Pop-Location
