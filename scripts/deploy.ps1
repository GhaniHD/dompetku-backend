# Deploy backend MyFulus dari laptop ke VPS (pengganti GitHub Actions).
# Jalankan dari folder repo backend:
#   powershell -ExecutionPolicy Bypass -File scripts\deploy.ps1
param(
    [string]$Server = "43.173.39.224",
    [string]$Key = "$HOME\.ssh\deploy_myfulus",
    [string]$Branch = "master",
    [switch]$SkipTests
)

$ErrorActionPreference = "Stop"

function Step($text) { Write-Host "`n==> $text" -ForegroundColor Cyan }
function Fail($text) { Write-Host "`nGAGAL: $text" -ForegroundColor Red; exit 1 }
function Check($what) { if ($LASTEXITCODE -ne 0) { Fail $what } }

$repo = $null
try { $repo = (git rev-parse --show-toplevel 2>$null) } catch { }
if (-not $repo) { Fail "Jalankan skrip ini dari dalam folder repo backend." }
Set-Location $repo

if (-not (Test-Path $Key)) { Fail "Kunci deploy tidak ditemukan: $Key" }
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { Fail "Go belum terpasang (cek dengan: go version)." }

Step "Cek git"
$current = (git rev-parse --abbrev-ref HEAD).Trim()
if ($current -ne $Branch) { Fail "Sedang di branch '$current'. Pindah dulu: git checkout $Branch" }
if (git status --porcelain) { Fail "Masih ada perubahan yang belum di-commit. Commit atau batalkan dulu." }
git fetch -q origin $Branch
Check "git fetch"
$behind = (git rev-list --count "HEAD..origin/$Branch").Trim()
if ($behind -ne "0") { Fail "Branch lokal ketinggalan $behind commit dari GitHub. Jalankan: git pull" }
$sha = (git rev-parse --short=7 HEAD).Trim()
$msg = (git log -1 --pretty=%s).Trim()
Write-Host "Commit $sha  $msg"

if (-not $SkipTests) {
    Step "Go vet dan tes"
    go vet ./...
    Check "go vet menemukan masalah"
    go test ./...
    Check "ada tes yang gagal, deploy dibatalkan"
}

Step "Build untuk Linux"
$out = Join-Path $env:TEMP "myfulus-deploy"
New-Item -ItemType Directory -Force -Path $out | Out-Null
$bin = Join-Path $out "myfulus-api"
$saved = @{ CGO_ENABLED = $env:CGO_ENABLED; GOOS = $env:GOOS; GOARCH = $env:GOARCH }
$env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "amd64"
try {
    go build -trimpath "-ldflags=-s -w" -o $bin ./cmd/app
    Check "build gagal"
} finally {
    $env:CGO_ENABLED = $saved.CGO_ENABLED; $env:GOOS = $saved.GOOS; $env:GOARCH = $saved.GOARCH
}
Write-Host ("Ukuran {0:N1} MB" -f ((Get-Item $bin).Length / 1MB))

$rel = "/opt/myfulus/releases/" + (Get-Date).ToUniversalTime().ToString("yyyyMMddHHmmss") + "-" + $sha
$target = "deploy@$Server"
$opts = @("-i", $Key, "-o", "IdentitiesOnly=yes", "-o", "BatchMode=yes")

Step "Kirim ke server"
ssh @opts $target "mkdir -p $rel"
Check "tidak bisa masuk sebagai deploy (cek kunci dan alamat server)"
scp @opts $bin "${target}:$rel/myfulus-api"
Check "gagal mengirim program"

Step "Aktifkan versi baru (cek kesehatan maks. 30 detik)"
ssh @opts $target "chmod 755 $rel/myfulus-api && sudo /opt/myfulus/bin/activate $rel"
$ok = ($LASTEXITCODE -eq 0)
Remove-Item -Recurse -Force $out -ErrorAction SilentlyContinue

if ($ok) {
    Write-Host "`nBERHASIL: versi $sha aktif di server." -ForegroundColor Green
    Write-Host "Cek di MyFulus Ops: CI/CD > Versi di server."
    exit 0
}
$active = (ssh @opts $target "readlink -f /opt/myfulus/current")
if ($active -and $active.Trim() -eq $rel) {
    Write-Host "`nPERINGATAN: versi $sha aktif dan sehat, tapi skrip activate mengembalikan error. Kirim output ini ke developer." -ForegroundColor Yellow
    exit 1
}
Fail "versi baru tidak sehat. Server kembali ke $active. Cek Ops > Log > Backend."