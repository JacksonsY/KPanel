# syntax=docker/dockerfile:1.27.1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e

FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine@sha256:f2c872bcee949b7be9c490efd02b48a8a2fbf947cdf533f1c58525ec9d8170fd AS build
ARG TARGETOS=linux
ARG TARGETARCH
ENV GOMAXPROCS=2
WORKDIR /src
COPY go.mod go.sum ./
COPY third_party/torrent/ ./third_party/torrent/
COPY internal/browserprotocol/ ./internal/browserprotocol/
COPY internal/netpolicy/ ./internal/netpolicy/
COPY internal/webbrowser/ ./internal/webbrowser/
COPY cmd/kpanel-browser/ ./cmd/kpanel-browser/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=secret,id=https_proxy,required=false \
    sh -eu -c 'if [ -f /run/secrets/https_proxy ]; then export HTTPS_PROXY="$(cat /run/secrets/https_proxy)"; fi; \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -p 2 -trimpath -ldflags="-s -w" -o /out/kpanel-browser ./cmd/kpanel-browser \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go test -p 2 -c -o /out/browser-tests ./internal/webbrowser'

FROM debian:trixie-slim@sha256:62bd423aeee912639ca8ee681c2b762cb3e25137e0a92b20be455898a9f034ad AS browser-base
ARG DEBIAN_MIRROR=https://deb.debian.org/debian
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
RUN --mount=type=secret,id=https_proxy,required=false \
    --mount=type=secret,id=proxy_ca,required=false,mode=0444 \
    --mount=type=cache,target=/var/cache/apt,sharing=locked \
    sh -eu -c 'if [ -f /run/secrets/https_proxy ]; then \
      export http_proxy="$(cat /run/secrets/https_proxy)" https_proxy="$(cat /run/secrets/https_proxy)"; fi; \
    case "$DEBIAN_MIRROR" in https://deb.debian.org/debian|https://mirrors.ustc.edu.cn/debian|https://mirrors.tuna.tsinghua.edu.cn/debian) ;; *) exit 2;; esac; \
    sed -i "s|http://deb.debian.org/debian-security|https://security.debian.org/debian-security|; s|http://deb.debian.org/debian$|$DEBIAN_MIRROR|" /etc/apt/sources.list.d/debian.sources; \
    rm -f /etc/apt/apt.conf.d/docker-clean; \
    if [ -f /run/secrets/proxy_ca ]; then export SSL_CERT_FILE=/run/secrets/proxy_ca; fi; \
    apt-get -o Acquire::https::CaInfo="${SSL_CERT_FILE:-/etc/ssl/certs/ca-certificates.crt}" update -o APT::Update::Error-Mode=any \
    && apt-get -o Acquire::https::CaInfo="${SSL_CERT_FILE:-/etc/ssl/certs/ca-certificates.crt}" install -y --no-install-recommends \
       ca-certificates chromium chromium-sandbox fonts-noto-cjk fonts-liberation tini \
    && dpkg-query -W > /usr/share/kpanel-browser-packages.txt \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 65532 browser \
    && useradd --uid 65532 --gid 65532 --home-dir /tmp/browser --no-create-home browser'
ENV HOME=/tmp/browser LANG=C.UTF-8
USER 65532:65532
WORKDIR /tmp

FROM browser-base AS test
COPY --from=build --chmod=0555 /out/browser-tests /usr/local/bin/browser-tests
ENV KPANEL_TEST_CHROME=/usr/bin/chromium KPANEL_TEST_SANDBOX=1
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/browser-tests"]
CMD ["-test.v", "-test.timeout=180s"]

FROM browser-base AS runtime
ARG VERSION=dev
ARG REVISION=unknown
LABEL org.opencontainers.image.title="KPanel Browser" \
      org.opencontainers.image.description="Optional sandboxed Chromium service for KPanel" \
      org.opencontainers.image.source="https://github.com/kejilion/KPanel" \
      org.opencontainers.image.url="https://hub.docker.com/r/kjlion/kpanel-browser" \
      org.opencontainers.image.licenses="AGPL-3.0-only" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}"
COPY --from=build --chmod=0555 /out/kpanel-browser /usr/local/bin/kpanel-browser
COPY LICENSE NOTICE THIRD_PARTY_NOTICES.md TRADEMARKS.md /licenses/
COPY LICENSES/ /licenses/third-party/
EXPOSE 8092
HEALTHCHECK --interval=30s --timeout=5s --start-period=40s --retries=3 \
    CMD ["/usr/local/bin/kpanel-browser", "-browser=/usr/bin/chromium", "-token-file=/run/secrets/browser-token", "-healthcheck"]
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/kpanel-browser"]
CMD ["-listen=0.0.0.0:8092", "-browser=/usr/bin/chromium", "-token-file=/run/secrets/browser-token", "-check-runtime"]
