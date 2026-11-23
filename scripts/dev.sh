#!/usr/bin/env bash
#
# 直接以源码方式运行 GoRester（开发调试用，无需先构建）。
#
# macOS / Linux
# brew install vivaui/tap/viva

# 检查环境是否就绪（会校验 Go 工具链、平台 SDK 等）
# viva doctor

# 用法：
#   ./scripts/dev.sh
#
set -euo pipefail

# 切到项目根目录（脚本位于 scripts/ 下）。
cd "$(dirname "$0")/.."

viva dev ./cmd/gorester
