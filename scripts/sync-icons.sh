#!/usr/bin/env sh
# 按 scripts/icons.txt 同步 Lucide 图标到 components/v1/icons。
# 清单内缺失的会下载，清单外的 .svg 会被删除，保证打包内容与清单一致。
set -eu

VERSION=1.72.0 # 固定版本；升级时改这里，然后重新运行本脚本
BASE="https://unpkg.com/lucide-static@${VERSION}/icons"
ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
DIR="$ROOT/components/v1/icons/assets"
LIST="$ROOT/scripts/icons.txt"

STAMP="$DIR/.lucide-version"

[ -f "$LIST" ] || { echo "缺少清单：$LIST" >&2; exit 1; }
mkdir -p "$DIR"

# 版本未变时跳过已存在的文件；改 VERSION 或删掉版本戳可强制全量刷新
SAME_VERSION=false
if [ -f "$STAMP" ] && [ "$(cat "$STAMP")" = "$VERSION" ]; then
	SAME_VERSION=true
fi

# 1. 下载清单里的图标
while read -r name; do
	[ -z "$name" ] && continue
	case "$name" in \#*) continue ;; esac
	if [ "$SAME_VERSION" = true ] && [ -f "$DIR/$name.svg" ]; then
		continue
	fi
	curl -fsSL "$BASE/$name.svg" -o "$DIR/$name.svg"
	echo "已下载 $name.svg"
done < "$LIST"

# 2. 删除清单外的图标，避免删除功能后仍被打包
for f in "$DIR"/*.svg; do
	[ -e "$f" ] || continue
	name=$(basename "$f" .svg)
	if ! grep -Fxq -- "$name" "$LIST"; then
		rm "$f"
		echo "已删除 $name.svg（不在清单中）"
	fi
done

# 3. 许可证随图标一起分发
if [ "$SAME_VERSION" = false ] || [ ! -f "$DIR/LICENSE" ]; then
	curl -fsSL "https://unpkg.com/lucide-static@${VERSION}/LICENSE" -o "$DIR/LICENSE"
fi

printf '%s\n' "$VERSION" > "$STAMP"
echo "完成，图标目录与 ${LIST} 一致（Lucide ${VERSION}）。"
