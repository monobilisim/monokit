#!/bin/sh
export CGO_ENABLED=0
mkdir -p bin
go build $@ -o bin/monokit
