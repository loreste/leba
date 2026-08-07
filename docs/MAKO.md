# Building Leba with Mako

Leba is written in [Mako](https://github.com/loreste/mako) and is built to use
**the production path Mako supports for this codebase today**.

## Required toolchain

| Item | Value |
|------|--------|
| Mako | **≥ 0.5.1** tip with [#29](https://github.com/loreste/mako/issues/29) for native *compile* |
| Backend (production) | **`c`** until [#31](https://github.com/loreste/mako/issues/31) is fixed |
| Default build | **`--release`** (`-O3 -flto`) |
| Allocator | **mimalloc** when present (`MAKO_ALLOCATOR`) |

```bash
# Install Mako (macOS/Linux)
curl -fsSL https://github.com/loreste/mako/releases/latest/download/install-release.sh | bash
# For native experiments, use a source checkout at/after 24f36a6:
#   cargo build --release -p mako
#   export MAKO=$PWD/target/release/mako
#   export MAKO_RUNTIME=$PWD/runtime
#   export MAKO_STD=$PWD/std

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
| Native multi-module compile (#29) | **Builds** with tip Mako; **not** production default yet |

## Native backend status

| Stage | Status |
|-------|--------|
| Compile `main.mko --backend native` | **OK** on Mako `main` ≥ `24f36a6` ([#29 closed](https://github.com/loreste/mako/issues/29)) |
| Run / unit tests / concurrent smoke | **Crash** — `SIGSEGV` in `doctor_world` → `mako_native_string_clone_ptr` ([#31 open](https://github.com/loreste/mako/issues/31)) |
| Production default | **`--backend c`** until #31 fixed and full test matrix green |

```bash
# Experimental native (expect crash after config_load until #31):
export MAKO_RUNTIME=/path/to/mako/runtime
mako build main.mko -o leba-native --backend native --release
./leba-native -f configs/leba.conf   # SIGSEGV today

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
(default). Flip to native only after #31 and CI matrix are green.

## Debug / sanitizers

```bash
make build-debug
mako build main.mko -o leba --backend c --sanitize address
```

## Upstream tracker

| Issue | Status | Topic |
|-------|--------|--------|
| [mako#29](https://github.com/loreste/mako/issues/29) | **Closed** | Compile: multi-module IR, builtins, honest diagnostics |
| [mako#31](https://github.com/loreste/mako/issues/31) | **Open** | Runtime: Leba SIGSEGV in `doctor_world` / string clone |

When #31 lands:

```bash
export MAKO=/path/to/mako/target/release/mako
export MAKO_RUNTIME=/path/to/mako/runtime
make clean-cache
MAKO_BACKEND=native make build
make test-full && make test-concurrent
```

## Related

- [PRODUCTION.md](PRODUCTION.md) — cutover checklist
- [LIMITS.md](LIMITS.md) — memory bounds
- [SCORECARD.md](SCORECARD.md) — RPS/latency vs nginx
- [Mako LONG_RUNNING.md](https://github.com/loreste/mako/blob/main/docs/LONG_RUNNING.md)
