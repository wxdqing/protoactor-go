#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
GRAIN_ROOT="${PROTOC_GEN_GO_GRAIN_ROOT:-$ROOT/../../tools/source/protoc-gen-go-grain}"
MODULE="github.com/asynkron/protoactor-go"
PROTO="$ROOT/service/example/grain-oneway/proto/grain_oneway.proto"

resolve_plugin() {
  local plugin=""

  if [[ -d "$GRAIN_ROOT" ]]; then
    plugin="$GRAIN_ROOT/protoc-gen-go-grain.exe"
    if [[ ! -f "$plugin" ]]; then
      (cd "$GRAIN_ROOT" && GOWORK=off go build -o protoc-gen-go-grain.exe .)
    fi
    if [[ -f "$plugin" ]]; then
      printf '%s' "$plugin"
      return
    fi

    plugin="$GRAIN_ROOT/protoc-gen-go-grain.sh"
    if [[ -f "$plugin" ]]; then
      printf '%s' "$plugin"
      return
    fi
  fi

  if command -v protoc-gen-go-grain >/dev/null 2>&1; then
    command -v protoc-gen-go-grain
    return
  fi

  echo "protoc-gen-go-grain not found" >&2
  echo "Sync tools/source/protoc-gen-go-grain or set PROTOC_GEN_GO_GRAIN_ROOT" >&2
  exit 1
}

PLUGIN="$(resolve_plugin)"

if [[ ! -d "$GRAIN_ROOT" ]]; then
  GRAIN_ROOT="$(cd "$(dirname "$PLUGIN")/.." && pwd)"
fi

cd "$ROOT"
protoc \
  --go_out=. --go_opt=module="$MODULE" \
  --plugin=protoc-gen-go-grain="$PLUGIN" \
  --go-grain_out=. --go-grain_opt=module="$MODULE" \
  --go-grain_opt=cluster_import="$MODULE/service/cluster" \
  -I"$ROOT" -I"$GRAIN_ROOT" \
  "$PROTO"
