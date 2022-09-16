FROM golang:1.19 as builder

ARG VERSION=1.0.6

WORKDIR /go/src/github.com/noenv/drone-s3-sync
ADD . /go/src/github.com/noenv/drone-s3-sync

RUN go vet ./... \
  && go test -cover ./... \
  && CGO_ENABLED=0 go build -v -ldflags "-X main.version=${VERSION}" -a -tags netgo -o release/drone-s3-sync

FROM plugins/base:multiarch

LABEL maintainer="Lukas Prettenthaler <lukas@noenv.com>" \
  org.label-schema.name="Drone S3 Sync" \
  org.label-schema.vendor="NoEnv" \
  org.label-schema.schema-version="1.0"

COPY --from=builder /go/src/github.com/noenv/drone-s3-sync/release/drone-s3-sync /bin/

ENTRYPOINT ["/bin/drone-s3-sync"]
