# 按 scripts/icons.txt 从 Iconify Solar（@iconify-json/solar）同步图标到 components/v1/icons/assets，
# 并自动重新生成 components/v1/icons/icons_gen.go。
# 清单内缺失的会下载，清单外的 .svg 会被删除；追加 -force 可强制全量刷新。

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
Push-Location $Root
try {
    go run ./scripts/gen-icons -sync @args
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
} finally {
    Pop-Location
}
