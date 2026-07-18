# Leba load balancer — multi-stage build.
# Build context should include a prebuilt `leba` binary or a Mako toolchain.
#
# Default: copy a locally-built binary (fast CI/dev).
# For from-source builds, set MAKO_IMAGE and use the mako stage.

FROM debian:bookworm-slim AS runtime
ARG LEGO_VERSION=4.17.4
ARG TARGETARCH=amd64
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl openssl \
    && rm -rf /var/lib/apt/lists/* \
    && ARCH="$TARGETARCH" \
    && if [ "$ARCH" = "amd64" ] || [ "$ARCH" = "x86_64" ] || [ -z "$ARCH" ]; then ARCH=amd64; fi \
    && if [ "$ARCH" = "arm64" ] || [ "$ARCH" = "aarch64" ]; then ARCH=arm64; fi \
    && curl -fsSL "https://github.com/go-acme/lego/releases/download/v${LEGO_VERSION}/lego_v${LEGO_VERSION}_linux_${ARCH}.tar.gz" \
       | tar -xz -C /usr/local/bin lego \
    && chmod +x /usr/local/bin/lego
RUN useradd -r -u 10001 -d /var/lib/leba -s /usr/sbin/nologin leba \
    && mkdir -p /etc/leba /var/lib/leba /var/lib/leba/acme /var/lib/leba/lego /var/log/leba /opt/leba/admin_ui \
    && chown -R leba:leba /var/lib/leba /var/log/leba

COPY leba /usr/local/bin/leba
COPY deploy/docker/leba.conf /etc/leba/leba.conf
COPY deploy/docker/leba.demo.conf /etc/leba/leba.demo.conf
COPY deploy/docker/admin-users.conf /etc/leba/admin-users.conf
COPY admin_ui/index.html /opt/leba/admin_ui/index.html
RUN chmod +x /usr/local/bin/leba

USER leba
EXPOSE 8080 8443 8404
ENV LEBA_CONFIG=/etc/leba/leba.conf \
    LEBA_ACME_STORAGE=/var/lib/leba/lego \
    LEBA_ACME_WEBROOT=/var/lib/leba/acme \
    LEBA_ACME_HELPER=lego
ENTRYPOINT ["/usr/local/bin/leba", "-f", "/etc/leba/leba.conf"]
