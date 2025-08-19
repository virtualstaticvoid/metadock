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

# ------------------------------------------------------------
FROM ubuntu:24.04 as test

ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get update -qq \
 && apt-get install -qy --no-install-recommends \
      ca-certificates \
      curl \
      unzip \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-$(uname -m).zip" -o awscli-exe-linux.zip \
 && tmpdir=$(mktemp -d) \
 && unzip awscli-exe-linux.zip -d "${tmpdir}" \
 && "${tmpdir}/aws/install" \
 && rm -rf awscli-exe-linux.zip "${tmpdir}"

COPY --chmod=755 test.sh /test.sh

CMD ["sleep", "infinity"]
