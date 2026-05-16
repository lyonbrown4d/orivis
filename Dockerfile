# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG APP=orivis-server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/orivis ./cmd/${APP}

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S orivis \
    && adduser -S -G orivis orivis \
    && mkdir -p /data \
    && chown -R orivis:orivis /data

WORKDIR /app
COPY --from=build /out/orivis /usr/local/bin/orivis

USER orivis
ENTRYPOINT ["orivis"]
