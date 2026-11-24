#!/usr/bin/env sh
# 按 scripts/icons.txt 从 Iconify Solar（@iconify-json/solar）同步图标到 components/v1/icons/assets，
# 并自动重新生成 components/v1/icons/icons_gen.go。
# 清单内缺失的会下载，清单外的 .svg 会被删除；追加 -force 可强制全量刷新。
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$ROOT"

exec go run ./scripts/gen-icons -sync "$@"
