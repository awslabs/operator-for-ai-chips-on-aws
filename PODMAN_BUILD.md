# Podman Build Guide

## Prerequisites
```bash
# Install podman
sudo yum install -y podman  # RHEL/Fedora/Amazon Linux
# or
sudo apt-get install -y podman  # Ubuntu/Debian
```

## Usage

Use `make -f Makefile.podman` instead of `make`:

### Build operator image
```bash
make -f Makefile.podman podman-build IMG=<registry>/<repo>/operator:<tag>
```

### Generate bundle (same as regular make)
```bash
make -f Makefile.podman bundle
```

### Build bundle image
```bash
make -f Makefile.podman podman-bundle-build BUNDLE_IMG=<registry>/<repo>/operator-bundle:<tag>
```

### Build index image
```bash
make -f Makefile.podman podman-index \
  BUNDLE_IMG=<registry>/<repo>/operator-bundle:<tag> \
  INDEX_IMG=<registry>/<repo>/operator-index:<tag>
```

### Full release workflow
```bash
# 1. Build operator
make -f Makefile.podman podman-build

# 2. Generate bundle
make -f Makefile.podman bundle

# 3. Build bundle
make -f Makefile.podman podman-bundle-build

# 4. Build index
make -f Makefile.podman podman-index
```

### Test builds (private registry)
```bash
make -f Makefile.podman podman-test-build
make -f Makefile.podman test-bundle
make -f Makefile.podman podman-test-bundle-build
make -f Makefile.podman podman-test-index
```

## Notes
- All non-build targets (manifests, generate, etc.) work the same
- Podman doesn't support multi-arch builds like docker buildx
- Images are built for linux/amd64 only
