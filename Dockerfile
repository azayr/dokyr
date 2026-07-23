# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS api
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=0.1.0-dev
ARG REVISION=unknown
ARG BUILD_DATE=unknown
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
  -ldflags="-s -w -X github.com/azayr/selfhost/internal/version.Version=${VERSION} -X github.com/azayr/selfhost/internal/version.Revision=${REVISION} -X github.com/azayr/selfhost/internal/version.BuildDate=${BUILD_DATE}" \
  -o /selfhost ./cmd/server

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS railpack
RUN go install github.com/railwayapp/railpack/cmd/cli@latest \
  && mv /go/bin/cli /go/bin/railpack

FROM --platform=$BUILDPLATFORM rust:1.85-alpine AS nixpacks
RUN apk add --no-cache build-base openssl-dev pkgconfig \
  && cargo install nixpacks --locked

FROM --platform=$BUILDPLATFORM alpine:3.22 AS runtime-deps
RUN apk add --no-cache ca-certificates tzdata

FROM alpine:3.22
ARG VERSION=0.1.0-dev
ARG REVISION=unknown
ARG BUILD_DATE=unknown
WORKDIR /app
RUN apk add --no-cache git docker-cli gcompat
COPY --from=runtime-deps /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=runtime-deps /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=api /selfhost /usr/local/bin/selfhost
COPY --from=railpack /go/bin/railpack /usr/local/bin/railpack
COPY --from=nixpacks /usr/local/cargo/bin/nixpacks /usr/local/bin/nixpacks
COPY --from=web /src/web/build ./web/build
ENV SELFHOST_ADDRESS=:8080 SELFHOST_FRONTEND_DIR=/app/web/build
LABEL org.opencontainers.image.title="Dokyr" \
  org.opencontainers.image.description="A lightweight self-hosted deployment control plane" \
  org.opencontainers.image.source="https://github.com/azayr/dokyr" \
  org.opencontainers.image.version="${VERSION}" \
  org.opencontainers.image.revision="${REVISION}" \
  org.opencontainers.image.created="${BUILD_DATE}"
EXPOSE 8080
ENTRYPOINT ["selfhost"]
