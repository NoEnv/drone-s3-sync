[![Docker Pulls](https://badgen.net/docker/pulls/noenv/s3-sync)](https://hub.docker.com/r/noenv/s3-sync)
[![Quay.io Enabled](https://badgen.net/badge/quay%20pulls/enabled/green)](https://quay.io/repository/noenv/s3-sync)
[![build](https://github.com/NoEnv/drone-s3-sync/actions/workflows/build.yml/badge.svg)](https://github.com/NoEnv/drone-s3-sync/actions/workflows/build.yml)

## drone-s3-sync

Drone plugin to synchronize a directory with an Amazon S3 Bucket.

This is a fork of the official [plugin](https://plugins.drone.io/plugins/s3-sync)

#### Local Build

Build the binary with the following command:

```console
export CGO_ENABLED=0
export GO111MODULE=on

go build -v -a -tags netgo -o release/drone-s3-sync
```

#### Container Build

Build the Docker image with the following command:

```console
docker build \
  --label org.label-schema.build-date=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  --label org.label-schema.vcs-ref=$(git rev-parse --short HEAD) \
  --file Dockerfile --tag noenv/s3-sync .
```

#### Container Usage

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

#### Drone Usage

```yaml
kind: pipeline
name: default

steps:
  - name: sync
    image: noenv/s3-sync
    settings:
      acl:
        "public/*": public-read
        "private/*": private
      access_key: AKIIZ3SQTZFDACSWPFSX
      secret_key:
        from_secret: aws_secret_access_key
      region: us-east-1
      bucket: my-bucket.s3-website-us-east-1.amazonaws.com
      cloudfront_distribution: E315A6KO9N36VD
      content_type:
        ".json": application/json
        ".svg": image/svg+xml
      cache_control:
        "*.json": "public, max-age=31536000"
      content_encoding:
        ".js": gzip
        ".css": gzip
      metadata:
        "*.png":
          CustomHeader: abc123
      redirects:
        "some/missing/file": /somewhere/that/actually/exists
      source: folder/to/archive
      target: target/location
      delete: true
      dry_run: false
```

#### Debug

```console
DEBUG=1 ./release/drone-s3-sync \
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
