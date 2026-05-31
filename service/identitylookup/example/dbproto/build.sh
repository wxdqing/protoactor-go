protoc \
  --go_out=paths=source_relative:./tcaplus \
  -I ./tcaplus/proto \
  ./tcaplus/proto/*.proto