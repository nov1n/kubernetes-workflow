#!/bin/bash

# Define die function
die() { echo "$@" 1>&2 ; exit 1; }

# Initialize error
ERROR=""

# Report suspicious constructs
printf 'GO VET:\n'
godep go vet $(go list ./... | grep -v '/vendor/' | grep '/nov1n/') || ERROR="Error with go vet"

# Test all packages except vendor
printf "\n\nGO TEST:\n"
godep go test $(go list ./... | grep -v '/vendor/' | grep '/nov1n/') || ERROR="Error testing all packages"

# Create bin directory if non existent
[ -d ./bin ] || mkdir bin

# Build main binary
printf "\n\nBUILDING BINARY:\n"
godep go build -o bin/workflow || ERROR="Build error"

# Create coverage file
[ -f ./coverage.out ] && rm ./coverage.out
touch ./coverage.out
echo 'mode: count' > ./coverage.out

# Run coverage per package
printf "\n\nCODE COVERAGE:\n"
CFILE=/tmp/cover.out
for pkg in ./pkg/*
do
    printf "[$pkg]\n"
    godep go test -covermode=count -coverprofile=/tmp/cover.out $pkg > /dev/null || ERROR="Error testing $pkg"
    if [ -f $CFILE ]
    then
        tail -n +2 /tmp/cover.out >> ./coverage.out
        godep go tool cover -func=/tmp/cover.out || ERROR="Unable to append coverage for $pkg"
        rm $CFILE
    else
        echo "No test coverage for $pkg"
    fi
    printf "\n"
done

# Exit non zero in case of error
if [ ! -z "$ERROR" ]
then
    die "Encountered error, last error was: $ERROR"
fi
