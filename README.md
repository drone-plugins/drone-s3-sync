# drone-s3-sync

[![Build Status](http://beta.drone.io/api/badges/drone-plugins/drone-s3-sync/status.svg)](http://beta.drone.io/drone-plugins/drone-s3-sync)

Drone plugin to synchronize a directory with an Amazon S3 Bucket. For the
usage information and a listing of the available options please take a look at
[the docs](DOCS.md).

## Build

Build the binary with the following commands:

```
go build
go test
```

## Docker

Build the Docker image with the following commands:

```
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -tags netgo
docker build --rm=true -t plugins/s3-sync .
```

Please note incorrectly building the image for the correct x64 linux and with
CGO disabled will result in an error when running the Docker image:

```
docker: Error response from daemon: Container command
'/bin/drone-s3-sync' not found or does not exist..
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
