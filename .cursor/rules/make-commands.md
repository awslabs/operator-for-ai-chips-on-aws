---
description: How to run make targets — check Go toolchain first, fall back to skipper
alwaysApply: true
---

# Running `make` Commands

Before executing any `make` target (e.g., `make unit-test`, `make fmt`,
`make lint`, `make generate`, `make manifests`, `make manager`):

1. **Check the local Go version:**

   ```bash
   go version 2>/dev/null
   ```

   Compare the output against the version required by `go.mod` (currently
   Go 1.24). The environment is compatible only if `go version` succeeds
   **and** reports a version that satisfies the `go.mod` requirement.

2. **If the environment is compatible**, run the `make` target directly:

   ```bash
   make unit-test
   ```

3. **If the environment is NOT compatible** (wrong Go version, `go` not
   found, or required utilities like `mockgen`/`controller-gen` are
   missing), **do NOT attempt to install, upgrade, or modify the local
   toolchain**. Instead, run the command through Skipper, which provides
   a containerised build environment with the correct Go version and all
   required tools:

   ```bash
   skipper make unit-test
   ```

This applies to every `make` invocation — tests, linting, formatting,
code generation, manifest generation, and builds. Never try to fix a
Go version mismatch or install missing tools on the host; always fall
back to `skipper make ...`.
