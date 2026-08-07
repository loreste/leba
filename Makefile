# Prefer installed Mako (stable). Override with MAKO=/path/to/mako for local builds.
# Note: in-tree target/release/mako can be ahead of the std install and may crash.
# After upgrading Mako: `make clean-cache` then rebuild (object cache is not versioned by free-analysis).
#
# Backend: default to C. Mako's native/Cranelift path still lacks struct fields and
# some HTTP builtins used by Leba; CI and production builds use --backend c.
MAKO ?= $(shell command -v mako)
MAKO_BACKEND ?= c
# Prefer in-tree quiche so HTTP/3 links when the third_party FFI build exists.
export MAKO_QUICHE_ROOT ?= $(shell if [ -f /Users/loreste/mako/runtime/third_party/quiche/target/release/libquiche.a ]; then echo /Users/loreste/mako/runtime/third_party/quiche; fi)

.PHONY: all build test check doctor doctor-linux explain smoke run clean \
	test-linux-assets test-ha-assets test-docs test-haproxy-compare \
	test-soak test-ha-peers test-concurrent test-adversarial \
	test-full test-ci test-all bench-nginx

all: build

build:
	$(MAKO) build main.mko -o leba --backend $(MAKO_BACKEND)

# Unit suites only (fast). Prefer these over `mako test .` (C redefinition).
test:
	$(MAKO) test leba_core1_test.mko --backend $(MAKO_BACKEND)
	$(MAKO) test leba_core2_test.mko --backend $(MAKO_BACKEND)
	$(MAKO) test leba_web_test.mko --backend $(MAKO_BACKEND)

test-linux-assets:
	test -f deploy/linux/leba.service
	test -f deploy/linux/leba.env
	test -f deploy/linux/leba.conf
	test -f deploy/linux/admin-users.conf
	test -f deploy/linux/sysctl.conf
	grep -q 'ExecStart=/usr/local/bin/leba -f' deploy/linux/leba.service
	grep -q 'LimitNOFILE=1048576' deploy/linux/leba.service
	grep -q 'LEBA_CONFIG=/etc/leba/leba.conf' deploy/linux/leba.env
	grep -q 'LEBA_ADMIN_AUTH=CHANGE_ME_ADMIN:CHANGE_ME_PASSWORD' deploy/linux/leba.env
	grep -q 'bind 80' deploy/linux/leba.conf
	grep -q 'state_file /var/lib/leba/state' deploy/linux/leba.conf
	grep -q 'admin_users_file /etc/leba/admin-users.conf' deploy/linux/leba.conf
	grep -q 'acme_webroot /var/lib/leba/acme' deploy/linux/leba.conf
	grep -q 'acme_storage /var/lib/leba/lego' deploy/linux/leba.conf
	grep -q 'acme_email' deploy/linux/leba.conf
	test -f deploy/linux/leba-acme-renew.timer
	test -f deploy/linux/leba-acme-renew.service
	grep -q 'CHANGE_ME_KDF_ADMIN_PASSWORD' deploy/linux/admin-users.conf
	grep -q '10.0.10.11:8080' deploy/linux/leba.conf

test-ha-assets:
	test -f deploy/ha/README.md
	test -f deploy/ha/keepalived.conf.example
	test -f deploy/ha/docker-compose.ha.yml
	test -f deploy/ha/leba-healthcheck.sh

test-docs:
	test -f docs/PRODUCTION.md
	test -f docs/LIMITS.md
	test -f docs/ROADMAP.md
	test -f docs/ADVERSARIAL_REVIEW.md
	test -f docs/HA.md
	test -f docs/SCORECARD.md
	test -f docs/ACME.md
	test -f scripts/ha_peers_smoke.sh
	test -f scripts/concurrent_smoke.sh
	test -f scripts/adversarial_smoke.sh
	test -f scripts/soak.sh
	test -f scripts/bench_vs_nginx.sh

test-adversarial: test test-linux-assets
	chmod +x scripts/adversarial_smoke.sh
	./scripts/adversarial_smoke.sh

test-concurrent: build
	chmod +x scripts/concurrent_smoke.sh
	./scripts/concurrent_smoke.sh 200

# Connection budget, keep-alive, body limit, reload under load (0.14 platform quality).
test-soak: build
	chmod +x scripts/soak.sh
	./scripts/soak.sh 200 6

# Dual-node peers smoke: HELLO, proxy, stick UPSERT sync, reconnect.
test-ha-peers: build
	chmod +x scripts/ha_peers_smoke.sh
	./scripts/ha_peers_smoke.sh

test-haproxy-compare: build
	chmod +x scripts/haproxy_compare.sh
	./scripts/haproxy_compare.sh

# Pre-push local gate: units + assets + concurrent + adversarial (no multi-min soak).
test-full: test build test-linux-assets test-ha-assets test-docs test-concurrent test-adversarial

# Matches .github/workflows/ci.yml (units, assets, soak, peers).
test-ci: test build test-linux-assets test-ha-assets test-docs test-concurrent test-adversarial test-soak test-ha-peers

# Full matrix including optional HAProxy behavior sample (needs haproxy in PATH).
test-all: test-ci test-haproxy-compare

# Directional RPS/latency vs local nginx (requires nginx in PATH).
bench-nginx: build
	chmod +x scripts/bench_vs_nginx.sh
	./scripts/bench_vs_nginx.sh 8 40

check: doctor

doctor: build
	./leba doctor configs/leba.conf

doctor-linux: build
	./leba doctor deploy/linux/leba.conf

explain: build
	./leba explain configs/leba.conf GET /api/hello localhost

run: build
	./leba -f configs/leba.conf

smoke: build
	chmod +x scripts/smoke.sh
	./scripts/smoke.sh

# Wipe Mako object cache after upgrading the compiler (stale .mako/cache/c can free wrong names).
clean-cache:
	rm -rf .mako/cache

clean: clean-cache
	rm -f leba /tmp/leba /tmp/leba_origin
