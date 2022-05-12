# == Build environment. ======================================================
FROM golang:alpine AS build-env

ENV GO111MODULE=on

WORKDIR /app
ADD . /app
RUN cd /app && go mod download && go build -o pubsub example/*
# ============================================================================



# == Run environment. ========================================
FROM alpine

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

WORKDIR /app
COPY --from=build-env /app/pubsub /app

ENTRYPOINT ./pubsub
# ============================================================================
