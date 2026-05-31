#!/bin/bash
protoc \
  --go_out=./proto --go_opt=paths=source_relative \
  --go-grain_out=./proto --go-grain_opt=paths=source_relative \
  -I ./proto ./proto/*.proto