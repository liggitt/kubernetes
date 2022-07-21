#!/bin/bash

set -xeu

# cleanup
rm -rf $HOME/.cache/golangci-lint/
rm -rf $GOPATH/bin/logcheck.so*
rm -rf $GOPATH/bin/golangci-lint*
rm -rf _output

# create directory for logcheck plugin
mkdir -p _output/local/bin/

go version

pushd hack/tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint
go build -o "../../_output/local/bin/logcheck.so" -buildmode=plugin sigs.k8s.io/logtools/logcheck/plugin
popd


export LOGCHECK_CONFIG="${HOME}/go/src/k8s.io/kubernetes/hack/logcheck.conf"

golangci-lint --path-prefix staging/src/k8s.io/api/apps -v run ./...
