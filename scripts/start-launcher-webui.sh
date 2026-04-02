#!/usr/bin/env bash
# Web UI Launcher：固定 Dashboard token，监听所有网卡（便于局域网访问）
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export PICOCLAW_LAUNCHER_TOKEN="${PICOCLAW_LAUNCHER_TOKEN:-TsingPaw-Dev-2026}"
mkdir -p "$HOME/.picoclaw/logs"
exec "$ROOT/build/picoclaw-launcher" -no-browser -public "$@"
