# Building Leba with Mako

Leba is written in [Mako](https://github.com/loreste/mako) and targets
**Mako 0.5.1+** with the multi-module native compile path from
[mako#29](https://github.com/loreste/mako/issues/29).

## Required toolchain

| Item | Value |
|------|--------|
| Mako | **≥ 0.5.1** with multi-module native IR fixes (**main ≥ `24f36a6`**, or a release that includes that commit) |
| Backend | **`c`** default for production + tests; **`native`** builds (compile) work |
| Default build | **`--release`** (`-O3 -flto`) |
| Allocator | **mimalloc** when present (`MAKO_ALLOCATOR`) |

```bash
# Install Mako from main (recommended until a release includes #29):
git clone https://github.com/loreste/mako.git && cd mako && make install
# Ensure native_bridge.c is installed (make install on main ≥ 5ef5186).

mako doctor
make check-mako   # Leba’s gate: version + backend + allocator
make build        # release binary → ./leba  (C backend by default)
make build-native # same with --backend native (compile OK)
```

After any Mako upgrade:

```bash
make clean-cache
make build
make test-full
```

## What we use from Mako 0.5.x

| Feature | How Leba uses it |
|---------|------------------|
| Ownership / SAFE drops (0.2.4+) | `own_string`, pending deep-own, stick `stick_table_own` — no free-alias |
| `--release` | Default `make build` / `make build-release` |
| `MAKO_ALLOCATOR` (0.4.11+) | Auto-link static mimalloc if Homebrew/Cellar provides `libmimalloc.a` |
| `sched_set_workers` | Crew pool sized `2×workers+8` so accept never starves |
| HTTP / TLS / H2 / pools | Cleartext fast path + TLS/H2/H3 surfaces |
| `mako doctor` | Install health before CI/cutover |
| Native compile (mako#29) | Shared IR lowers multi-file packs, string struct fields, HTTP/proxy builtins |

## Native backend status (honest)

| Capability | Status |
|------------|--------|
| **Compile** `main.mko` with `--backend native` | **Works** (mako#29 closed) — no more misleading `HostPort.host` scalar error |
| Pack-local helpers + string struct fields | **Works** on shared IR |
| **`make test` / `leba_core1_test.mko` under native** | **Fails** — SIGSEGV in `mako_native_string_slice_free_elements` while dropping `[]string` during `parse_config_text` (`TestRouteAndAcl`) |
| Production default | Still **`c`** until native unit suites are green |

```bash
# Compile path (should succeed with Mako main ≥ 24f36a6 + installed bridge .c):
make build-native
# or:
mako build main.mko -o leba --backend native --release

# Tests still pin C:
make test                    # MAKO_BACKEND=c
mako test leba_core1_test.mko --backend c
```

Crash signature (for upstream):

```text
EXC_BAD_ACCESS in mako_native_string_slice_free_elements
  ← mako_native_string_slice_drop_ptr
  ← parse_config_text
  ← TestRouteAndAcl
```

## Backends

| Backend | When |
|---------|------|
| **`c`** (default) | CI, unit/e2e tests, production until native runtime is clean |
| **`native`** | Experiment / compile validation; `make build-native` |
| **`llvm`** | Optional; needs `mako` built with `--features llvm-backend` |

```bash
make build                    # c + release
make build-c                  # explicit c
make build-native             # native compile
MAKO_BACKEND=native make build

# ASan (C backend only):
mako build main.mko -o leba --backend c --sanitize address
```

## What we do **not** use yet (honest)

| Feature | Why not |
|---------|---------|
| **Native as default for test/prod** | Remaining `[]string` drop crash in config parse (above) |
| **LLVM backend** | Needs `mako` built with `--features llvm-backend`; optional later |
| **DTLS / WSI (0.5.1)** | Not part of reverse-proxy product surface |

## Allocator (RSS under load)

Mako’s system allocator + high churn can leave process RSS high after spikes.
For production-shaped builds:

```bash
# Auto (Makefile detects brew mimalloc):
make build

# Explicit static (preferred — no dylib dep):
MAKO_ALLOCATOR=$(brew --prefix mimalloc)/lib/libmimalloc.a make build

# Force system malloc:
MAKO_ALLOCATOR=system make build

# jemalloc:
MAKO_ALLOCATOR=jemalloc MAKO_LDFLAGS="-L$(brew --prefix jemalloc)/lib" make build
```

Install mimalloc on macOS: `brew install mimalloc`.

Live queues are still **capped in Leba** (see [LIMITS.md](LIMITS.md)); the
allocator choice affects fragmentation and OS reclaim, not those hard caps.

## CI

`.github/workflows/ci.yml` clones Mako `main`, installs with cargo, then
`make test` / `make build` with default **`c`**. Release flag and mimalloc
apply when available on the runner; absence of mimalloc falls back to system.

## Debug / sanitizers

```bash
make build-debug                    # no -O3, system allocator
# ASan (C backend only):
mako build main.mko -o leba --backend c --sanitize address
```

## Related

- [PRODUCTION.md](PRODUCTION.md) — cutover checklist  
- [LIMITS.md](LIMITS.md) — memory bounds  
- [SCORECARD.md](SCORECARD.md) — RPS/latency vs nginx  
- [Mako LONG_RUNNING.md](https://github.com/loreste/mako/blob/main/docs/LONG_RUNNING.md)  

## Upstream tracker

| Issue | Topic | Status |
|-------|--------|--------|
| [mako#29](https://github.com/loreste/mako/issues/29) | Native multi-module compile: missing IR builtins + misleading HostPort fallback | **Fixed** (`24f36a6`) |
| [mako#30](https://github.com/loreste/mako/issues/30) | Native runtime: `[]string` drop SIGSEGV in config parse / `TestRouteAndAcl` | Open — blocks native default for tests |
