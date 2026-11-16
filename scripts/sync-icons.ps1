# 按 scripts/icons.txt 同步 Lucide 图标到 components/v1/icons/assets。
# 清单内缺失的会下载，清单外的 .svg 会被删除，保证打包内容与清单一致。

$ErrorActionPreference = "Stop"

$Version = "1.72.0"
$Base = "https://unpkg.com/lucide-static@$Version/icons"
$Root = Split-Path -Parent $PSScriptRoot
$Dir = Join-Path $Root "components\v1\icons\assets"
$List = Join-Path $Root "scripts\icons.txt"
$Stamp = Join-Path $Dir ".lucide-version"

if (-not (Test-Path $List)) {
    Write-Error "缺少清单：$List"
}

if (-not (Test-Path $Dir)) {
    New-Item -ItemType Directory -Path $Dir -Force | Out-Null
}

$SameVersion = $false
if ((Test-Path $Stamp) -and ((Get-Content $Stamp -Raw).Trim() -eq $Version)) {
    $SameVersion = $true
}

$Names = Get-Content $List | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne "" -and -not $_.StartsWith("#") }

# 1. 下载清单里的图标
foreach ($name in $Names) {
    $target = Join-Path $Dir "$name.svg"
    if ($SameVersion -and (Test-Path $target)) {
        continue
    }
    $url = "$Base/$name.svg"
    curl.exe -k -fsSL $url -o $target
    Write-Host "已下载 $name.svg"
}

# 2. 删除清单外的图标
$svgFiles = Get-ChildItem -Path $Dir -Filter "*.svg" -File
foreach ($file in $svgFiles) {
    $baseName = $file.BaseName
    if ($Names -notcontains $baseName) {
        Remove-Item $file.FullName -Force
        Write-Host "已删除 $($file.Name)（不在清单中）"
    }
}

# 3. 许可证
$licensePath = Join-Path $Dir "LICENSE"
if (-not $SameVersion -or -not (Test-Path $licensePath)) {
    curl.exe -k -fsSL "https://unpkg.com/lucide-static@$Version/LICENSE" -o $licensePath
}

Set-Content -Path $Stamp -Value $Version -NoNewline
Write-Host "完成，图标目录与 $List 一致（Lucide $Version）。"
