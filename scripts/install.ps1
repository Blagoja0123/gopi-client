# Install Gopi Client on Windows (PowerShell):
#
#   irm https://raw.githubusercontent.com/Blagoja0123/gopi-client/master/scripts/install.ps1 | iex
#
# Set $env:VERSION = "v1.2.3" first to install a specific release.
# Downloads the installer, verifies its checksum and runs it silently. It
# installs per user (no administrator prompt) into
# %LOCALAPPDATA%\Programs\Gopi Client, installs the WebView2 runtime if
# missing, and adds Start Menu and desktop shortcuts.
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue' # Invoke-WebRequest is very slow with the progress bar

$repo = if ($env:GOPI_REPO) { $env:GOPI_REPO } else { 'Blagoja0123/gopi-client' }
$version = if ($env:VERSION) { $env:VERSION } else { 'latest' }
$base = if ($version -eq 'latest') { "https://github.com/$repo/releases/latest/download" }
        else { "https://github.com/$repo/releases/download/$version" }
$asset = 'gopi-client_windows_amd64_setup.exe'

$tmp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid())
New-Item -ItemType Directory $tmp | Out-Null
try {
    $exe = Join-Path $tmp $asset
    Write-Host "==> Downloading $asset"
    Invoke-WebRequest "$base/$asset" -OutFile $exe -UseBasicParsing
    $sums = (Invoke-WebRequest "$base/SHA256SUMS" -UseBasicParsing).Content
    if ($sums -is [byte[]]) { $sums = [Text.Encoding]::UTF8.GetString($sums) }
    $line = ($sums -split "`n") | Where-Object { $_ -match "\s\*?$([regex]::Escape($asset))\s*$" } | Select-Object -First 1
    if (-not $line) { throw "$asset is not listed in SHA256SUMS" }
    $want = ($line -split '\s+')[0].ToLower()
    $got = (Get-FileHash $exe -Algorithm SHA256).Hash.ToLower()
    if ($want -ne $got) { throw "checksum mismatch for $asset" }

    Write-Host '==> Installing'
    $p = Start-Process -FilePath $exe -ArgumentList '/S' -Wait -PassThru
    if ($p.ExitCode -ne 0) { throw "installer exited with code $($p.ExitCode)" }
    Write-Host '==> Done. Launch "Gopi Client" from the Start Menu.'
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
