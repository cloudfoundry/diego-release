$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

Write-Host "Downloading winpty DLL"
Add-Type -AssemblyName System.IO.Compression, System.IO.Compression.FileSystem
$WINPTY_DIR = "C:\winpty"
$env:WINPTY_DLL_DIR="$WINPTY_DIR\x64\bin"
if(!(Test-Path -Path $env:WINPTY_DLL_DIR )) {
    New-Item -ItemType directory -Path $WINPTY_DIR -Force
    (New-Object System.Net.WebClient).DownloadFile('https://github.com/rprichard/winpty/releases/download/0.4.3/winpty-0.4.3-msvc2015.zip', "$WINPTY_DIR\winpty.zip")
    [System.IO.Compression.ZipFile]::ExtractToDirectory("$WINPTY_DIR\winpty.zip", "$WINPTY_DIR")
}

Debug "$(gci env:* | sort-object name | Out-String)"

Invoke-Expression "go run github.com/onsi/ginkgo/v2/ginkgo $args"
if ($LastExitCode -ne 0) {
  throw "tests failed"
}
