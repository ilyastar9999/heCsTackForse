param(
    [string]$ChallengesDir = ".\challenges",
    [string]$TagPrefix = "hecstack/",
    [switch]$NoCache,
    [string]$Challenge
)

$ErrorActionPreference = "Stop"

function Get-ImageTag {
    param(
        [string]$Prefix,
        [string]$FolderName
    )

    $safeName = $FolderName.ToLower() -replace '[^a-z0-9._/-]', '-'
    $safeName = $safeName.Trim('-','.')
    if ([string]::IsNullOrWhiteSpace($safeName)) {
        throw "Cannot derive image tag from folder name '$FolderName'"
    }

    if (-not $Prefix.EndsWith("/") -and -not $Prefix.EndsWith(":")) {
        $Prefix = "$Prefix/"
    }
    return "$Prefix$safeName`:latest"
}

$root = Resolve-Path $ChallengesDir
$dockerfiles = Get-ChildItem -Path $root -Recurse -Filter Dockerfile | Sort-Object FullName

if ($Challenge) {
    $dockerfiles = $dockerfiles | Where-Object {
        $_.DirectoryName -like "*$Challenge*"
    }
}

if (-not $dockerfiles) {
    Write-Host "No Dockerfile found under $root"
    exit 0
}

foreach ($dockerfile in $dockerfiles) {
    $context = $dockerfile.Directory.FullName
    $tag = Get-ImageTag -Prefix $TagPrefix -FolderName $dockerfile.Directory.Name

    $args = @("build", "-t", $tag, "-f", $dockerfile.FullName)
    if ($NoCache) {
        $args += "--no-cache"
    }
    $args += $context

    Write-Host "Building $tag from $context"
    & docker @args
    if ($LASTEXITCODE -ne 0) {
        throw "docker build failed for $context"
    }
}

Write-Host "Build complete."
