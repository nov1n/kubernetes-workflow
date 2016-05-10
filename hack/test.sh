#!/bin/bash
go vet $(go list ./... | grep -v '/vendor/')
go test $(go list ./... | grep -v '/vendor/')

[ -d ./bin ] && mkdir bin

godep go build -o bin/workflow

for file in ./pkg/*
do
    go test -coverprofile=/tmp/cover.out $file
    go tool cover -func=/tmp/cover.out
done
