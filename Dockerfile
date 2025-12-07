# Simplified Railway-compatible Dockerfile for Navidrome
# Uses pre-built binary approach for Railway deployment

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
### Build Navidrome binary
FROM golang:1.25-alpine AS build

# Install build dependencies (including static libraries)
RUN apk add --no-cache git gcc g++ musl-dev pkgconfig taglib-dev zlib-dev zlib-static

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

# Build the binary with static linking
RUN CGO_ENABLED=1 go build -tags=netgo \
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
