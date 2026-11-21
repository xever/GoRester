#!/usr/bin/env bash
#
# 运行测试与静态检查（gofmt / go vet / go test）。
#
# 用法：
#   ./scripts/test.sh
#
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> gofmt 检查（应无输出）"
gofmt -l .

echo "==> go vet"
go vet ./...

echo "==> go test"
go test ./... -count=1
