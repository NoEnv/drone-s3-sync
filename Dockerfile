FROM golang:1.16 as builder

ARG VERSION=1.0.2

WORKDIR /go/src/github.com/noenv/drone-s3-sync
ADD . /go/src/github.com/noenv/drone-s3-sync

RUN go vet ./... \
  && go test -cover ./... \
  && CGO_ENABLED=0 GO111MODULE=on go build -v -ldflags "-X main.version=${VERSION}" -a -tags netgo -o release/linux/amd64/drone-s3-sync

FROM plugins/base:multiarch

LABEL maintainer="Lukas Prettenthaler <lukas@noenv.com>" \
  org.label-schema.name="Drone S3 Sync" \
  org.label-schema.vendor="NoEnv" \
  org.label-schema.schema-version="1.0"

COPY --from=builder /go/src/github.com/noenv/drone-s3-sync/release/linux/amd64/drone-s3-sync /bin/

ENTRYPOINT ["/bin/drone-s3-sync"]
