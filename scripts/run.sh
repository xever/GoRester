#!/usr/bin/env bash
#
# 直接以源码方式运行 GoRester（开发调试用，无需先构建）。
#
# 用法：
#   ./scripts/run.sh
#
set -euo pipefail

# 切到项目根目录（脚本位于 scripts/ 下）。
cd "$(dirname "$0")/.."

go run ./cmd/gorester
