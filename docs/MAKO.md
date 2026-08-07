# Building Leba with Mako

Leba is written in [Mako](https://github.com/loreste/mako) and is built to use
**100% of the production path Mako 0.5.1+ supports for this codebase**.

## Required toolchain

| Item | Value |
|------|--------|
| Mako | **≥ 0.5.1** (`mako version`) |
| Backend | **`c`** (required) |
| Default build | **`--release`** (`-O3 -flto`) |
| Allocator | **mimalloc** when present (`MAKO_ALLOCATOR`) |

```bash
# Install Mako (macOS/Linux)
curl -fsSL https://github.com/loreste/mako/releases/latest/download/install-release.sh | bash
# or Linux: install-linux.sh — see mako README

mako doctor
make check-mako   # Leba’s gate: version + backend + allocator
make build        # release binary → ./leba
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

## What we do **not** use yet (honest)

| Feature | Why not |
|---------|---------|
| **Native backend (default in 0.5.0)** | Cranelift still errors: `struct field HostPort.host type is not implemented yet (only scalar fields)`. Leba is full of string/struct fields. |
| **LLVM backend** | Needs `mako` built with `--features llvm-backend`; optional later. |
| **DTLS / WSI (0.5.1)** | Not part of reverse-proxy product surface. |

When native supports non-scalar struct fields, re-evaluate:

```bash
mako build main.mko -o leba --backend native --release
```

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
`make test` / `make build` with `MAKO_BACKEND=c`. Release flag and mimalloc
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

| Issue | Topic |
|-------|--------|
| [mako#29](https://github.com/loreste/mako/issues/29) | Native backend: multi-module apps (Leba) fail — IR missing builtins + misleading scalar-struct fallback |

When that lands, re-try:

```bash
mako build main.mko -o leba --backend native --release
make test-full
```
