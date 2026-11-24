#!/usr/bin/env bash
#
# 从 viva.toml 的 [app].version 生成 internal/app/version.go。
# build.sh / package-win.sh / package-mac.sh 在构建前都会调用，
# 避免设置抽屉里显示的版本号与打包清单不一致。
#
set -euo pipefail

# 切到项目根目录（脚本位于 scripts/ 下）。
cd "$(dirname "$0")/.."

version="$(sed -n 's/^version = "\(.*\)"/\1/p' viva.toml | head -n 1)"
if [ -z "$version" ]; then
	echo "❌ viva.toml 中缺少 [app].version" >&2
	exit 1
fi

cat > internal/app/version.go <<EOF
// Code generated from viva.toml by scripts/sync-version.sh. DO NOT EDIT.

package app

// appVersion 显示在设置抽屉底部，与 viva.toml 中的版本号保持一致。
const appVersion = "$version"
EOF
