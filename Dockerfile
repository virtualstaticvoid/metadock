# ------------------------------------------------------------
FROM golang:1.22 AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-X go.virtualstaticvoid.com/metadock/Version=${VERSION}" -o /metadock .

# ------------------------------------------------------------
FROM alpine:3.20 AS base

RUN apk add --no-cache tini curl

# ------------------------------------------------------------
FROM base AS app

RUN mkdir -p /opt/metadock/bin/

COPY --from=builder --chmod=755 /metadock /opt/metadock/bin/

COPY README.md LICENSE /opt/metadock/

ENV PORT=80

HEALTHCHECK --interval=5s \
            --timeout=10s \
            --start-period=1s \
            --retries=5 \
  CMD curl -sSfL http://127.0.0.1:${PORT}/health/ > /dev/null

ENTRYPOINT ["/sbin/tini", "--", "/opt/metadock/bin/metadock"]
CMD ["default"]
