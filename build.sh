#!/bin/sh
mkdir -p bin
go build $@ -o bin/mono-go
