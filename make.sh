#/bin/bash

mkdir -p build

GOOS_ARG=linux
GOARCH_ARG=amd64

GOOS=$GOOS_ARG GOARCH=$GOARCH_ARG go build -o build/smdp-gateway

# 单测覆盖率
# go test -coverprofile=/tmp/size_coverage.out && go tool cover -html=/tmp/size_coverage.out -o /tmp/coverage.html && sz -bye /tmp/coverage.html
