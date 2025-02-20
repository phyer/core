#!/bin/sh
go build -ldflags "-s -w" -trimpath -o ./bin/core ./cmd/core
rm bin/core
