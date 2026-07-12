MAKO ?= $(shell if [ -x /Users/loreste/mako/target/release/mako ]; then echo /Users/loreste/mako/target/release/mako; else command -v mako; fi)

.PHONY: all build test check doctor doctor-linux explain smoke run clean test-linux-assets test-haproxy-compare

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
