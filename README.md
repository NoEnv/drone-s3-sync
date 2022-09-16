[![Docker Pulls](https://badgen.net/docker/pulls/noenv/s3-sync)](https://hub.docker.com/r/noenv/s3-sync)
[![Quay.io Enabled](https://badgen.net/badge/quay%20pulls/enabled/green)](https://quay.io/repository/noenv/s3-sync)
[![build](https://github.com/NoEnv/drone-s3-sync/actions/workflows/build.yml/badge.svg)](https://github.com/NoEnv/drone-s3-sync/actions/workflows/build.yml)

## drone-s3-sync

Drone plugin to synchronize a directory with an Amazon S3 Bucket.

This is a fork of the official [plugin](http://plugins.drone.io/drone-plugins/drone-s3-sync/)

#### Build

Build the binary with the following command:

```console
export CGO_ENABLED=0
export GO111MODULE=on

go build -v -a -tags netgo -o release/drone-s3-sync
```

#### Docker

Build the Docker image with the following command:

```console
docker build \
  --label org.label-schema.build-date=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  --label org.label-schema.vcs-ref=$(git rev-parse --short HEAD) \
  --file Dockerfile --tag noenv/s3-sync .
```

#### Usage

```console
docker run --rm \
  -e PLUGIN_SOURCE=<source> \
  -e PLUGIN_TARGET=<target> \
  -e PLUGIN_BUCKET=<bucket> \
  -e AWS_ACCESS_KEY_ID=<access_key> \
  -e AWS_SECRET_ACCESS_KEY=<secret_key> \
  -v $(pwd):$(pwd) \
  -w $(pwd) \
  noenv/s3-sync
```

#### Debug

```console
DEBUG=1 ./release/linux/amd64/drone-s3-sync \
  --access-key <access_key> \
  --secret-key <secret_key> \
  --bucket <bucket> \
  --region <region> \
  --source . \
  --target / \
  --access public-read \
  --content-type '{".txt":"text/plain"}' \
  --cache-control '{"*.txt":"max-age=3600"}' \
  --delete \
  --dry-run
```

#### Source

https://github.com/noenv/drone-s3-sync
