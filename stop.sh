#!/usr/bin/env bash
# 停止 Slate 并删除服务文件：等同于在本目录执行 ./slate -stop。
set -euo pipefail
cd "$(dirname "$0")"
exec ./slate -stop "$@"
