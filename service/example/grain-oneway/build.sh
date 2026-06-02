#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
PLUGIN="$ROOT/service/protobuf/protoc-gen-go-grain/protoc-gen-go-grain.sh"
MODULE="github.com/asynkron/protoactor-go"
PROTO="$ROOT/service/example/grain-oneway/proto/grain_oneway.proto"

cd "$ROOT"
protoc \
  --go_out=. --go_opt=module="$MODULE" \
  --plugin=protoc-gen-go-grain="$PLUGIN" \
  --go-grain_out=. --go-grain_opt=module="$MODULE" \
  -I"$ROOT" -I"$ROOT/service/protobuf/protoc-gen-go-grain" \
  "$PROTO"
