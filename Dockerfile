ARG GO_VERSION=1.26-alpine

FROM golang:${GO_VERSION} AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates git upx

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG APP=orivis-server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/orivis ./cmd/${APP} \
    && (upx --best --lzma /out/orivis || upx --best /out/orivis)

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
