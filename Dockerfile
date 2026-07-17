# Leba load balancer — multi-stage build.
# Build context should include a prebuilt `leba` binary or a Mako toolchain.
#
# Default: copy a locally-built binary (fast CI/dev).
# For from-source builds, set MAKO_IMAGE and use the mako stage.

FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*
RUN useradd -r -u 10001 -d /var/lib/leba -s /usr/sbin/nologin leba \
    && mkdir -p /etc/leba /var/lib/leba /var/log/leba /opt/leba \
    && chown -R leba:leba /var/lib/leba /var/log/leba

COPY leba /usr/local/bin/leba
COPY deploy/docker/leba.conf /etc/leba/leba.conf
COPY deploy/docker/admin-users.conf /etc/leba/admin-users.conf
COPY admin_ui/index.html /opt/leba/admin_ui/index.html
RUN chmod +x /usr/local/bin/leba

USER leba
EXPOSE 8080 8443 8404
ENV LEBA_CONFIG=/etc/leba/leba.conf
ENTRYPOINT ["/usr/local/bin/leba", "-f", "/etc/leba/leba.conf"]
