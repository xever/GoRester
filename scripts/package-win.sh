#!/usr/bin/env bash
#
# 打包 Windows x64 版：编译 exe 并嵌入应用图标，输出 zip。
#
# 说明：
#   - 依赖 Viva 的打包命令，通过 go run 调用（首次会编译该命令，稍慢）。
#   - Windows 目标走 purego（无需 mingw-w64 交叉编译器）。
#   - 图标与版本信息由 viva.toml 的 [app] icon 与 addWindowsResources 写入 exe。
#
# 用法：
#   ./scripts/package-win.sh
#
set -euo pipefail

# 切到项目根目录。
cd "$(dirname "$0")/.."

go run github.com/vivaui/viva/cmd/viva@v0.6.6 package \
	--target windows --arch amd64 --format zip
