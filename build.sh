#!/bin/sh
[ "$1" = "docs" ] && swag init --parseDependency --parseInternal -g common/api/server.go

export CGO_ENABLED=0
mkdir -p bin
go build $@ -o bin/monokit
strip bin/monokit
