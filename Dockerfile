# Docker image for the Drone build runner
#
#     CGO_ENABLED=0 go build -a -tags netgo
#     docker build --rm=true -t plugins/drone-s3-sync .

FROM gliderlabs/alpine:3.1
RUN apk add --update \
  ca-certificates
ADD drone-s3-sync /bin/
ADD mime.types /etc/
ENTRYPOINT ["/bin/drone-s3-sync"]
