# Building Leba with Mako

Leba is written in [Mako](https://github.com/loreste/mako) and is built to use
**the production path Mako supports for this codebase today**.

## Required toolchain

| Item | Value |
|------|--------|
| Mako | **≥ 0.5.1** (verified on **0.5.2**) |
| Backend (production) | **`c`** — native compiles ([#29](https://github.com/loreste/mako/issues/29)) but still crashes at runtime on 0.5.2 (see below) |
| Default build | **`--release`** (`-O3 -flto`) |
| Allocator | **mimalloc** when present (`MAKO_ALLOCATOR`) |

```bash
# Install Mako (macOS/Linux)
curl -fsSL https://github.com/loreste/mako/releases/latest/download/install-release.sh | bash

mako doctor
make check-mako
make build        # production: --backend c --release → ./leba
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
| Ownership / SAFE drops (0.2.4+) | `own_string`, pending deep-own, stick `stick_table_own` |
| `--release` | Default `make build` / `make build-release` |
| `MAKO_ALLOCATOR` (0.4.11+) | Auto-link static mimalloc when available |
| `sched_set_workers` | Crew pool sized `2×workers+8` |
| HTTP / TLS / H2 / pools | Cleartext fast path + TLS/H2/H3 surfaces |
| Native multi-module compile (#29) | **Builds** on 0.5.2; **not** production default yet (runtime crash, see below) |

## Native backend status

Re-validated on **Mako 0.5.2** (contains the #31 fix, `f638e64`): native
**still crashes**. The original moved-from-slot bug is fixed, but Leba hits a
follow-on native fault in the same doctor/validation path.

| Stage | Status |
|-------|--------|
| Compile `main.mko --backend native` | **OK** on 0.5.2 ([#29](https://github.com/loreste/mako/issues/29), [#31](https://github.com/loreste/mako/issues/31) closed) |
| Run minimal conf (`frontend web` + `route default -> app`) | **Crash** — `SIGSEGV` in `doctor_world` → `mako_native_string_clone_ptr`; faulting slot address (`x23`) is a wild non-heap value |
| Run full `configs/leba.conf` | **Crash** — `SIGSEGV` in `mako_native_struct_slice_clone_ptr` cloning `[]Route` (10 fields, str_mask=191) |
| Unit tests (`leba_*_test.mko --backend native`) | **Crash** — SIGSEGV / SIGABRT |
| Production default | **`--backend c`** until native survives `make test-full` + concurrent smoke |

```bash
# Experimental native (still crashes on 0.5.2, right after config_load):
mako build main.mko -o leba-native --backend native --release
./leba-native doctor configs/leba.conf   # SIGSEGV

# Production:
make build
```

| Feature | Notes |
|---------|--------|
| **LLVM** | Optional; rebuild Mako with `--features llvm-backend`. |
| **DTLS / WSI** | Not part of reverse-proxy product surface. |

## Allocator (RSS under load)

Prefer production builds with **mimalloc** (`make build` auto-detects Homebrew
`libmimalloc.a`). Live queues are still **capped in Leba** (see [LIMITS.md](LIMITS.md)).

```bash
MAKO_ALLOCATOR=system make build          # force system malloc
brew install mimalloc                     # enable auto static link
```

## CI

`.github/workflows/ci.yml` clones Mako `main` and builds with `MAKO_BACKEND=c`
(default). Flip to native only after the native runtime crash (see above) is
fixed and the CI matrix is green on native.

## Debug / sanitizers

```bash
make build-debug
mako build main.mko -o leba --backend c --sanitize address
```

## Upstream tracker

| Issue | Status | Topic |
|-------|--------|--------|
| [mako#29](https://github.com/loreste/mako/issues/29) | **Closed** | Compile: multi-module IR, builtins, honest diagnostics |
| [mako#31](https://github.com/loreste/mako/issues/31) | **Closed** (`f638e64`, shipped in 0.5.2) | Runtime: moved-from slot use-after-free |
| [mako#32](https://github.com/loreste/mako/issues/32) | **Open** | Runtime: Leba still SIGSEGVs on native in 0.5.2 — `doctor_world` string clone (wild slot address) / `[]Route` struct-slice clone |

When native survives the full gate on a future Mako release:

```bash
make clean-cache
MAKO_BACKEND=native make build
make test-full && make test-concurrent
```

## Related

- [PRODUCTION.md](PRODUCTION.md) — cutover checklist
- [LIMITS.md](LIMITS.md) — memory bounds
- [SCORECARD.md](SCORECARD.md) — RPS/latency vs nginx
- [Mako LONG_RUNNING.md](https://github.com/loreste/mako/blob/main/docs/LONG_RUNNING.md)
