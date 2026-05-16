ARG GO_VERSION=1.26-alpine
ARG NODE_VERSION=22-alpine
ARG PNPM_VERSION=10.29.2

FROM node:${NODE_VERSION} AS web-build
ARG PNPM_VERSION
WORKDIR /src/web
RUN corepack enable && corepack prepare pnpm@${PNPM_VERSION} --activate
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM golang:${GO_VERSION} AS go-build
WORKDIR /src
RUN apk add --no-cache ca-certificates git upx

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/orivis-server ./cmd/orivis-server \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/orivis-agent ./cmd/orivis-agent \
    && (upx --best --lzma /out/orivis-server || upx --best /out/orivis-server) \
    && (upx --best --lzma /out/orivis-agent || upx --best /out/orivis-agent)

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S orivis \
    && adduser -S -G orivis orivis \
    && mkdir -p /data \
    && chown -R orivis:orivis /data

WORKDIR /app

FROM runtime AS agent
COPY --from=go-build /out/orivis-agent /usr/local/bin/orivis-agent
USER orivis
ENTRYPOINT ["orivis-agent"]

FROM runtime AS server
COPY --from=go-build /out/orivis-server /usr/local/bin/orivis-server
COPY --from=web-build /src/web/dist /app/web
ENV ORIVIS_WEB__ENABLED=true
ENV ORIVIS_WEB__ROOT=/app/web
USER orivis
ENTRYPOINT ["orivis-server"]
