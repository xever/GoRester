#!/usr/bin/env bash
#
# 构建 GoRester，把可执行文件输出到 dist/ 目录。
#
# 用法：
#   ./scripts/build.sh
#
set -euo pipefail

# 切到项目根目录（脚本位于 scripts/ 下）。
cd "$(dirname "$0")/.."

# 创建成品目录（若不存在）。
mkdir -p dist

# 编译 cmd/gorester 入口，输出到 dist/gorester。
# -trimpath 让二进制里的路径更干净，-ldflags "-s -w" 去掉调试信息减小体积。
go build -trimpath -ldflags "-s -w" -o dist/gorester ./cmd/gorester

echo "✅ 构建完成：dist/gorester"
ls -lh dist/gorester
