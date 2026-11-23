#!/usr/bin/env bash
#
# 打包 macOS 版：编译 .app 并内嵌应用图标。
#
# 说明：
#   - macOS 打包用 .icns 图标，而 viva.toml 里 [app] icon 默认指向 Windows 的
#     .ico；这里临时替换成 .icns，打包结束（含出错）后自动还原。
#   - 依赖 Viva 的打包命令，通过 go run 调用（首次会编译该命令，稍慢）。
#   - 默认输出 .app（若需要 .zip / .dmg，把 --format app 改成对应格式）。
#
# 用法：
#   ./scripts/package-mac.sh
#
set -euo pipefail

# 切到项目根目录。
cd "$(dirname "$0")/.."

# 同步版本号：viva package 用自己的 ldflags，显示的版本来自生成的 Go 文件。
./scripts/sync-version.sh

manifest="viva.toml"
backup="$(mktemp)"

# 无论成功失败，退出前都还原 viva.toml。
restore() {
	if [ -f "$backup" ]; then
		mv -f "$backup" "$manifest"
	fi
}
trap restore EXIT

cp "$manifest" "$backup"

# 把 Windows 的 .ico 图标换成 macOS 的 .icns（用可移植的 sed 写法，避免 BSD/GNU 差异）。
sed 's#resource/icon\.ico#resource/icon.icns#' "$manifest" > "$manifest.tmp"
mv "$manifest.tmp" "$manifest"

go run github.com/vivaui/viva/cmd/viva@v0.6.6 package \
	--target darwin --format app
