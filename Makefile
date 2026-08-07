# Leba — production reverse proxy built with Mako.
#
# Requires Mako ≥ 0.5.1 (ownership drops, --release, MAKO_ALLOCATOR, SAFE model).
# Install: https://github.com/loreste/mako/releases (or build from source).
#
# Backend: C is required today. Mako 0.5+ defaults to native (Cranelift), but
# native still rejects non-scalar struct fields (e.g. HostPort.host: string).
# CI and production always use --backend c until that lands.
#
# After upgrading Mako: `make clean-cache` then rebuild (object cache is not
# versioned across compiler revisions).
#
# Production build (default): --release (-O3 -flto) + mimalloc when available.
# Debug / ASan: make build-debug   MAKO_RELEASE=0

MAKO ?= $(shell command -v mako)
MAKO_BACKEND ?= c

# Release by default so “make build” is the binary you ship.
MAKO_RELEASE ?= 1
ifeq ($(MAKO_RELEASE),1)
MAKO_RELEASE_FLAG := --release
else
MAKO_RELEASE_FLAG :=
endif

# Optional production allocator (Mako 0.4.11+). Auto-detect Homebrew mimalloc.
# Override: MAKO_ALLOCATOR=system | jemalloc | /path/to/libmimalloc.a
# Disable:  MAKO_ALLOCATOR=system make build
MIMALLOC_PREFIX ?= $(shell brew --prefix mimalloc 2>/dev/null)
ifeq ($(origin MAKO_ALLOCATOR),undefined)
  ifneq ($(wildcard $(MIMALLOC_PREFIX)/lib/libmimalloc.a),)
    export MAKO_ALLOCATOR := $(MIMALLOC_PREFIX)/lib/libmimalloc.a
  else ifneq ($(wildcard $(MIMALLOC_PREFIX)/lib/libmimalloc.dylib),)
    export MAKO_ALLOCATOR := mimalloc
    export MAKO_LDFLAGS := -L$(MIMALLOC_PREFIX)/lib
  endif
endif

# Prefer in-tree quiche so HTTP/3 links when the third_party FFI build exists.
export MAKO_QUICHE_ROOT ?= $(shell if [ -f /Users/loreste/mako/runtime/third_party/quiche/target/release/libquiche.a ]; then echo /Users/loreste/mako/runtime/third_party/quiche; elif [ -f "$$HOME/mako/runtime/third_party/quiche/target/release/libquiche.a" ]; then echo "$$HOME/mako/runtime/third_party/quiche"; fi)

.PHONY: all build build-debug build-release check-mako test check doctor doctor-linux \
	explain smoke run clean clean-cache \
	test-linux-assets test-ha-assets test-docs test-haproxy-compare \
	test-soak test-ha-peers test-concurrent test-adversarial \
	test-full test-ci test-all bench-nginx

all: build

# Fail closed if Mako is missing or too old for Leba’s production path.
check-mako:
	@command -v "$(MAKO)" >/dev/null 2>&1 || { echo "mako not found in PATH — install ≥ 0.5.1 from https://github.com/loreste/mako/releases" >&2; exit 1; }
	@"$(MAKO)" version >/dev/null
	@v=$$("$(MAKO)" version 2>/dev/null | sed -n 's/.*mako\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\).*/\1/p' | head -1); \
	if [ -z "$$v" ]; then echo "could not parse mako version from: $$("$(MAKO)" version 2>&1)" >&2; exit 1; fi; \
	major=$$(echo "$$v" | cut -d. -f1); minor=$$(echo "$$v" | cut -d. -f2); patch=$$(echo "$$v" | cut -d. -f3); \
	ok=0; \
	if [ "$$major" -gt 0 ]; then ok=1; fi; \
	if [ "$$major" -eq 0 ] && [ "$$minor" -gt 5 ]; then ok=1; fi; \
	if [ "$$major" -eq 0 ] && [ "$$minor" -eq 5 ] && [ "$$patch" -ge 1 ]; then ok=1; fi; \
	if [ "$$ok" -ne 1 ]; then \
	  echo "Leba requires Mako ≥ 0.5.1 (found $$v). Upgrade: https://github.com/loreste/mako/releases" >&2; \
	  exit 1; \
	fi; \
	echo "mako ok: $$("$(MAKO)" version 2>&1 | head -1) backend=$(MAKO_BACKEND) release=$(MAKO_RELEASE) allocator=$${MAKO_ALLOCATOR:-system}"

build: check-mako
	@echo "building leba: backend=$(MAKO_BACKEND) release=$(MAKO_RELEASE) allocator=$${MAKO_ALLOCATOR:-system}"
	$(MAKO) build main.mko -o leba --backend $(MAKO_BACKEND) $(MAKO_RELEASE_FLAG)

# Fast iterate without -O3 / LTO.
build-debug:
	$(MAKE) build MAKO_RELEASE=0 MAKO_ALLOCATOR=system

# Explicit production binary (same as default when MAKO_RELEASE=1).
build-release:
	$(MAKE) build MAKO_RELEASE=1

# Unit suites only (fast). Prefer these over `mako test .` (C redefinition).
test: check-mako
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
	test -f docs/MAKO.md
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

# Connection budget, keep-alive, body limit, reload under load.
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

# Wipe Mako object cache after upgrading the compiler.
clean-cache:
	rm -rf .mako/cache

clean: clean-cache
	rm -f leba /tmp/leba /tmp/leba_origin
