#!/usr/bin/env bash
# 后台启动 Slate：等同于在本目录执行 ./slate -d。
set -euo pipefail
cd "$(dirname "$0")"
exec ./slate -d "$@"
