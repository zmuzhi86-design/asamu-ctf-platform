# syntax=docker/dockerfile:1.7
ARG GO_BUILD_IMAGE=golang:1.25.12-alpine3.24
ARG ALPINE_BASE_IMAGE=alpine:3.24
ARG GOPROXY=https://goproxy.cn|https://proxy.golang.org|direct
ARG GOSUMDB=sum.golang.google.cn

FROM ${GO_BUILD_IMAGE} AS build
ARG GOPROXY
ARG GOSUMDB
ENV GOPROXY=${GOPROXY} GOSUMDB=${GOSUMDB}
WORKDIR /src/api
COPY apps/api/go.mod apps/api/go.sum ./
WORKDIR /src/worker
COPY apps/worker/go.mod apps/worker/go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
WORKDIR /src
COPY apps/api ./api
COPY apps/worker ./worker
WORKDIR /src/worker
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/asamu-worker ./cmd/asamu-worker

FROM ${ALPINE_BASE_IMAGE}
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/asamu-worker /usr/local/bin/asamu-worker
ENTRYPOINT ["asamu-worker"]
