# Prefer installed Mako (stable). Override with MAKO=/path/to/mako for local builds.
# Note: in-tree target/release/mako can be ahead of the std install and may crash.
MAKO ?= $(shell command -v mako)
# Prefer in-tree quiche so HTTP/3 links when the third_party FFI build exists.
export MAKO_QUICHE_ROOT ?= $(shell if [ -f /Users/loreste/mako/runtime/third_party/quiche/target/release/libquiche.a ]; then echo /Users/loreste/mako/runtime/third_party/quiche; fi)

.PHONY: all build test check doctor doctor-linux explain smoke run clean test-linux-assets test-haproxy-compare test-soak test-ha-peers test-concurrent test-adversarial

all: build

build:
	$(MAKO) build main.mko -o leba

test:
	$(MAKO) test leba_core1_test.mko
	$(MAKO) test leba_core2_test.mko
	$(MAKO) test leba_web_test.mko

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
	grep -q 'CHANGE_ME_KDF_ADMIN_PASSWORD' deploy/linux/admin-users.conf
	grep -q '10.0.10.11:8080' deploy/linux/leba.conf

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

# Dual-node peers lifecycle on localhost (HELLO/metrics/idle; proxy under peers still experimental).
test-ha-peers: build
	chmod +x scripts/ha_peers_smoke.sh
	./scripts/ha_peers_smoke.sh

test-haproxy-compare: build
	chmod +x scripts/haproxy_compare.sh
	./scripts/haproxy_compare.sh

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
	./scripts/smoke.sh

clean:
	rm -f leba /tmp/leba /tmp/leba_origin
