#!/bin/bash

# Initialize error
ERROR=""

# Report suspicious constructs
godep go vet $(go list ./... | grep -v '/vendor/') || ERROR="Error with go vet"
# Test all packages except vendor
godep go test $(go list ./... | grep -v '/vendor/') || ERROR="Error testing all packages"

[ -d ./bin ] || mkdir bin

godep go build -o bin/workflow || ERROR="Build error"

for pkg in ./pkg/*
do
    CFILE=/tmp/cover.out
    godep go test -coverprofile=/tmp/cover.out $pkg || ERROR="Error testing $pkg"
    if [ -f $CFILE ]
    then
        godep go tool cover -func=/tmp/cover.out || ERROR="Unable to append coverage for $pkg"
        rm $CFILE
    else
        echo "No tests coverage for $pkg"
    fi
done

if [ ! -z "$ERROR" ]
then
    die "Encountered error, last error was: $ERROR"
fi
