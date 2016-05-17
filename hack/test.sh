#!/bin/bash
godep go vet $(go list ./... | grep -v '/vendor/')
godep go test $(go list ./... | grep -v '/vendor/')

[ -d ./bin ] || mkdir bin

godep go build -o bin/workflow

for file in ./pkg/*
do
    CFILE=/tmp/cover.out
    godep go test -coverprofile=/tmp/cover.out $file
    if [ -f $CFILE ]
    then
        godep go tool cover -func=/tmp/cover.out
        rm $CFILE
    else
        echo "No tests coverage for $file"
    fi
done
