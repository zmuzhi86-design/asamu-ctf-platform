[CmdletBinding()]
param(
  [string]$OutputDirectory = "",
  [string]$Version = "0.3.1"
)

$ErrorActionPreference = 'Stop'
$project = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
if (-not $OutputDirectory) { $OutputDirectory = Join-Path $project 'vm-dist' }
$output = [System.IO.Path]::GetFullPath($OutputDirectory)
if (-not $output.StartsWith($project + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
  throw 'Output directory must stay inside the project workspace.'
}
if ($Version -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$' -or $Version -eq 'latest') {
  throw 'Version must be fixed and safe; latest is not allowed.'
}

$stage = Join-Path $output 'stage'
$bundleName = "asamu-vm-$Version"
$bundleRoot = Join-Path $stage $bundleName
$archive = Join-Path $output "$bundleName.tar.gz"

if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force }
New-Item -ItemType Directory -Path $bundleRoot -Force | Out-Null

$rootFiles = @(
  '.dockerignore', '.env.docker.example', 'docker-compose.yml',
  'docker-compose.build.yml', 'README.md', 'CHANGELOG.md', 'PORTFIX_NOTES.md',
  'RELEASE_NOTES_0.3.md'
)
foreach ($file in $rootFiles) {
  Copy-Item -LiteralPath (Join-Path $project $file) -Destination $bundleRoot
}

$directories = @('apps/api', 'apps/worker', 'apps/web', 'challenges', 'deploy', 'scripts', 'docs')
$excludedDirectoryNames = @('node_modules', 'dist', '.cache', '.gocache', '.gomodcache', 'coverage', 'backups', 'offline-dist', 'vm-dist', '__pycache__', '.git', '.github', '.agents', '.codex')
$excludedFileNames = @('.env', '.env.docker', 'deployment-credentials.txt', 'tsconfig.app.tsbuildinfo', 'tsconfig.node.tsbuildinfo')

foreach ($directory in $directories) {
  $source = Join-Path $project $directory
  if (-not (Test-Path -LiteralPath $source)) { continue }
  Get-ChildItem -LiteralPath $source -Recurse -Force -File | ForEach-Object {
    $relative = $_.FullName.Substring($project.Length).TrimStart([char[]]'\/')
    $segments = $relative -split '[\\/]'
    if ($segments | Where-Object { $_ -in $excludedDirectoryNames }) { return }
    if ($_.Name -in $excludedFileNames -or $_.Extension -in @('.log', '.pyc')) { return }
    $destination = Join-Path $bundleRoot $relative
    New-Item -ItemType Directory -Path (Split-Path $destination) -Force | Out-Null
    Copy-Item -LiteralPath $_.FullName -Destination $destination
  }
}

$manifestLines = Get-ChildItem -LiteralPath $bundleRoot -Recurse -Force -File |
  ForEach-Object {
    $relative = $_.FullName.Substring($bundleRoot.Length).TrimStart([char[]]'\/').Replace('\', '/')
    [PSCustomObject]@{
      Path = $relative
      Line = "$( (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant() )  $relative"
    }
  } |
  Sort-Object Path |
  ForEach-Object { $_.Line }
Set-Content -LiteralPath (Join-Path $bundleRoot 'SOURCE_MANIFEST.sha256') -Value $manifestLines -Encoding ascii

if (Test-Path -LiteralPath $archive) { Remove-Item -LiteralPath $archive -Force }
$tar = Get-Command 'tar.exe' -ErrorAction SilentlyContinue
if (-not $tar) { throw 'tar.exe is required to create an Ubuntu-compatible archive.' }
Push-Location $stage
try {
  & $tar.Source -czf $archive $bundleName
  if ($LASTEXITCODE -ne 0) { throw "tar.exe failed with exit code $LASTEXITCODE" }
} finally {
  Pop-Location
}
$hash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
Set-Content -LiteralPath "$archive.sha256" -Value "$hash  $([System.IO.Path]::GetFileName($archive))" -Encoding ascii
Remove-Item -LiteralPath $stage -Recurse -Force

$size = [math]::Round((Get-Item -LiteralPath $archive).Length / 1MB, 2)
Write-Host "VM bundle: $archive"
Write-Host "Size: $size MB"
Write-Host "SHA-256: $hash"
