#!/usr/bin/env bash

export GOBIN=$GOPATH/bin
export PATH=$PATH:$GOBIN

make deps build
make deps docker
docker build --rm=true -t ribase/drone-s3-sync .
docker push ribase/drone-s3-sync