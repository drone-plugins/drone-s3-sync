# drone-s3-sync

[![Build Status](http://beta.drone.io/api/badges/drone-plugins/drone-s3-sync/status.svg)](http://beta.drone.io/drone-plugins/drone-s3-sync)
[![Join the chat at https://gitter.im/drone/drone](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/drone/drone)
[![Go Doc](https://godoc.org/github.com/drone-plugins/drone-s3-sync?status.svg)](http://godoc.org/github.com/drone-plugins/drone-s3-sync)
[![Go Report](https://goreportcard.com/badge/github.com/drone-plugins/drone-s3-sync)](https://goreportcard.com/report/github.com/drone-plugins/drone-s3-sync)
[![](https://images.microbadger.com/badges/image/plugins/s3-sync.svg)](https://microbadger.com/images/plugins/s3-sync "Get your own image badge on microbadger.com")

Drone plugin to synchronize a directory with an Amazon S3 Bucket. For the usage information and a listing of the available options please take a look at [the docs](http://plugins.drone.io/drone-plugins/drone-s3-sync/).

## Build

Build the binary with the following commands:

```
go build
```

## Docker

Build the Docker image with the following commands:

```
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -tags netgo -o release/linux/amd64/drone-s3-sync
docker build --rm -t plugins/s3-sync .
```

## Usage

Execute from the working directory:

```sh
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
