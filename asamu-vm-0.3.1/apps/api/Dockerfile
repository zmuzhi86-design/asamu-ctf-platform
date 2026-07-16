# syntax=docker/dockerfile:1.7
ARG GO_BUILD_IMAGE=golang:1.25.12-alpine3.24
ARG ALPINE_BASE_IMAGE=alpine:3.24
ARG GOPROXY=https://goproxy.cn|https://proxy.golang.org|direct
ARG GOSUMDB=sum.golang.google.cn

FROM ${GO_BUILD_IMAGE} AS build
ARG GOPROXY
ARG GOSUMDB
ENV GOPROXY=${GOPROXY} GOSUMDB=${GOSUMDB}
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY apps/api/go.mod apps/api/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY apps/api/. .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/asamu ./cmd/asamu

FROM ${ALPINE_BASE_IMAGE}
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 10001 asamu && \
    adduser -D -u 10001 -G asamu asamu
WORKDIR /app
COPY --from=build /out/* /usr/local/bin/
COPY apps/api/openapi ./openapi
COPY apps/web/public/assets/default/manifest.v3.json /app/assets/default/manifest.v3.json
ENV ASSET_MANIFEST_PATH=/app/assets/default/manifest.v3.json
RUN mkdir -p /app/var/storage && chown -R asamu:asamu /app
USER 10001:10001
EXPOSE 8787
ENTRYPOINT ["asamu"]
