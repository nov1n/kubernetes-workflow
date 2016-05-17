#!/bin/bash
godep go vet $(go list ./... | grep -v '/vendor/')
godep go test $(go list ./... | grep -v '/vendor/')

[ -d ./bin ] || mkdir bin

godep go build -o bin/workflow

for file in ./pkg/*
do
    godep go test -coverprofile=/tmp/cover.out $file
    godep go tool cover -func=/tmp/cover.out
done
