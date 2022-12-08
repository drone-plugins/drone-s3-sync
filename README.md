# drone-s3-sync

[![Build Status](http://cloud.drone.io/api/badges/drone-plugins/drone-s3-sync/status.svg)](http://cloud.drone.io/drone-plugins/drone-s3-sync)
[![Gitter chat](https://badges.gitter.im/drone/drone.png)](https://gitter.im/drone/drone)
[![Join the discussion at https://discourse.drone.io](https://img.shields.io/badge/discourse-forum-orange.svg)](https://discourse.drone.io)
[![Drone questions at https://stackoverflow.com](https://img.shields.io/badge/drone-stackoverflow-orange.svg)](https://stackoverflow.com/questions/tagged/drone.io)
[![](https://images.microbadger.com/badges/image/plugins/s3-sync.svg)](https://microbadger.com/images/plugins/s3-sync "Get your own image badge on microbadger.com")
[![Go Doc](https://godoc.org/github.com/drone-plugins/drone-s3-sync?status.svg)](http://godoc.org/github.com/drone-plugins/drone-s3-sync)
[![Go Report](https://goreportcard.com/badge/github.com/drone-plugins/drone-s3-sync)](https://goreportcard.com/report/github.com/drone-plugins/drone-s3-sync)

Drone plugin to synchronize a directory with an Amazon S3 Bucket. For the usage information and a listing of the available options please take a look at [the docs](http://plugins.drone.io/drone-plugins/drone-s3-sync/).

## Build

Build the binary with the following command:

```bash
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0
export GO111MODULE=on

go build -v -a -tags netgo -o release/linux/amd64/drone-s3-sync
```

## Docker

Build the Docker image with the following command:

```bash
docker build \
  --label org.label-schema.build-date=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  --label org.label-schema.vcs-ref=$(git rev-parse --short HEAD) \
  --file docker/Dockerfile.linux.amd64 --tag plugins/s3-sync .
```

## Usage

```bash
docker run --rm \
  -e PLUGIN_SOURCE=<source> \
  -e PLUGIN_TARGET=<target> \
  -e PLUGIN_BUCKET=<bucket> \
  -e AWS_ACCESS_KEY_ID=<access_key> \
  -e AWS_SECRET_ACCESS_KEY=<secret_key> \
  -v $(pwd):$(pwd) \
  -w $(pwd) \
  plugins/s3-sync
```
