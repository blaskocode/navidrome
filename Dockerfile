# Railway-compatible Dockerfile for Navidrome
# Uses Debian-based build for glibc compatibility with pre-built TagLib

########################################################################################################################
### Get pre-built static TagLib
FROM debian:bookworm-slim AS taglib-build
ARG CROSS_TAGLIB_VERSION=2.1.1-1
ENV CROSS_TAGLIB_RELEASES_URL=https://github.com/navidrome/cross-taglib/releases/download/v${CROSS_TAGLIB_VERSION}/

RUN apt-get update && apt-get install -y --no-install-recommends wget ca-certificates && \
    wget ${CROSS_TAGLIB_RELEASES_URL}taglib-linux-amd64.tar.gz && \
    mkdir /taglib && \
    tar -xzf taglib-linux-amd64.tar.gz -C /taglib && \
    rm -rf /var/lib/apt/lists/*

########################################################################################################################
### Build Navidrome UI
FROM node:lts-alpine AS ui
WORKDIR /app

# Install node dependencies
COPY ui/package.json ui/package-lock.json ./
COPY ui/bin/ ./bin/
RUN npm ci

# Build bundle
COPY ui/ ./
RUN npm run build -- --outDir=/build

########################################################################################################################
### Build Navidrome binary (using Debian for glibc compatibility)
FROM golang:1.25-bookworm AS build

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git gcc g++ pkg-config zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

# Copy pre-built static TagLib
COPY --from=taglib-build /taglib /taglib

WORKDIR /src

# Copy go mod files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy UI build
COPY --from=ui /build ./ui/build

ARG GIT_SHA
ARG GIT_TAG

# Build the binary with static linking using pre-built TagLib
RUN CGO_ENABLED=1 PKG_CONFIG_PATH=/taglib/lib/pkgconfig go build -tags=netgo \
    -ldflags="-w -s -extldflags '-static -latomic' \
        -X github.com/navidrome/navidrome/consts.gitSha=${GIT_SHA} \
        -X github.com/navidrome/navidrome/consts.gitTag=${GIT_TAG}" \
    -o /navidrome .

########################################################################################################################
### Build Final Image
FROM alpine:3.19
LABEL maintainer="deluan@navidrome.org"
LABEL org.opencontainers.image.source="https://github.com/navidrome/navidrome"

# Install runtime dependencies
RUN apk add --no-cache ffmpeg mpv sqlite ca-certificates tzdata

# Copy navidrome binary
COPY --from=build /navidrome /app/navidrome

# Railway manages volumes externally - no VOLUME directive
ENV ND_MUSICFOLDER=/data/music
ENV ND_DATAFOLDER=/data
ENV ND_CONFIGFILE=/data/navidrome.toml
ENV ND_PORT=4533
RUN mkdir -p /data/music && touch /.nddockerenv

EXPOSE 4533
WORKDIR /app

ENTRYPOINT ["/app/navidrome"]
